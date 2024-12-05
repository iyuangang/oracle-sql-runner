package core

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/iyuangang/oracle-sql-runner/pkg/models"
)

// normalizeSQL 规范化SQL语句格式
func normalizeSQL(sql string, sqlType models.SQLType) string {
	if sqlType != models.SQLTypePLSQL {
		return strings.TrimSpace(sql)
	}

	// 处理PL/SQL块
	lines := strings.Split(sql, "\n")
	var normalized []string
	baseIndent := "    " // 基础缩进为4个空格

	// 计算最小缩进
	minIndent := -1
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		indent := len(line) - len(strings.TrimLeft(line, " \t"))
		if minIndent == -1 || indent < minIndent {
			minIndent = indent
		}
	}

	// 规范化每一行
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		// 根据关键字调整缩进
		indent := ""
		upperTrimmed := strings.ToUpper(trimmed)

		// 减少缩进的关键字
		if strings.HasPrefix(upperTrimmed, "END") ||
			strings.HasPrefix(upperTrimmed, "EXCEPTION") {
			indent = ""
		} else if strings.HasPrefix(upperTrimmed, "BEGIN") ||
			strings.HasPrefix(upperTrimmed, "DECLARE") ||
			strings.HasPrefix(upperTrimmed, "CREATE") {
			indent = ""
		} else if strings.HasPrefix(upperTrimmed, "ELSE") ||
			strings.HasPrefix(upperTrimmed, "ELSIF") {
			indent = baseIndent
		} else {
			// 普通语句使用基础缩进
			indent = baseIndent
		}

		normalized = append(normalized, indent+trimmed)
	}

	return strings.Join(normalized, "\n")
}

// validateSQLContent 验证SQL文件内容是否有效
func validateSQLContent(content string) error {
	// 去除注释和空行
	lines := strings.Split(content, "\n")
	var validLines []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "--") {
			continue
		}
		validLines = append(validLines, line)
	}

	// 如果去除注释和空行后没有内容，则无效
	if len(validLines) == 0 {
		return fmt.Errorf("SQL文件内容为空或仅包含注释")
	}

	// 检查是否包含常见SQL关键字
	content = strings.ToUpper(strings.Join(validLines, " "))
	validKeywords := []string{
		"SELECT", "INSERT", "UPDATE", "DELETE",
		"CREATE", "ALTER", "DROP", "MERGE",
		"BEGIN", "DECLARE", "EXECUTE", "GRANT",
		"TRUNCATE", "COMMENT", "ANALYZE", "CALL",
	}

	for _, keyword := range validKeywords {
		if strings.Contains(content, keyword) {
			return nil
		}
	}

	return fmt.Errorf("SQL文件内容无效: 未找到有效的SQL语句")
}

// ParseFile 解析SQL文件
func ParseFile(path string) ([]models.SQLTask, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	// 验证SQL文件内容
	if err := validateSQLContent(string(content)); err != nil {
		return nil, err
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var tasks []models.SQLTask
	scanner := bufio.NewScanner(file)
	var sqlBuffer strings.Builder
	lineNum := 0
	inPLSQLBlock := false
	hasContent := false

	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())

		// 跳过空行和注释
		if line == "" || strings.HasPrefix(line, "--") {
			continue
		}

		hasContent = true

		// 检查是否进入PL/SQL块
		upperLine := strings.ToUpper(line)
		if strings.HasPrefix(upperLine, "BEGIN") ||
			strings.HasPrefix(upperLine, "DECLARE") ||
			strings.HasPrefix(upperLine, "CREATE OR REPLACE PROCEDURE") ||
			strings.HasPrefix(upperLine, "CREATE PROCEDURE") ||
			strings.HasPrefix(upperLine, "CREATE OR REPLACE FUNCTION") ||
			strings.HasPrefix(upperLine, "CREATE FUNCTION") ||
			strings.HasPrefix(upperLine, "CREATE OR REPLACE TRIGGER") ||
			strings.HasPrefix(upperLine, "CREATE TRIGGER") ||
			strings.HasPrefix(upperLine, "CREATE OR REPLACE PACKAGE") ||
			strings.HasPrefix(upperLine, "CREATE PACKAGE") {
			inPLSQLBlock = true
		}

		sqlBuffer.WriteString(line)
		sqlBuffer.WriteString("\n")

		// 处理PL/SQL块
		if inPLSQLBlock && line == "/" {
			sql := strings.TrimSpace(sqlBuffer.String())
			sql = strings.TrimSuffix(sql, "/")
			tasks = append(tasks, models.SQLTask{
				SQL:      normalizeSQL(sql, models.SQLTypePLSQL),
				Type:     models.SQLTypePLSQL,
				LineNum:  lineNum,
				Filename: path,
			})
			sqlBuffer.Reset()
			inPLSQLBlock = false
			hasContent = false
			continue
		}

		// 处理普通SQL语句
		if !inPLSQLBlock && strings.HasSuffix(line, ";") {
			sql := strings.TrimSpace(sqlBuffer.String())
			sql = strings.TrimSuffix(sql, ";")

			sqlType := models.SQLTypeExec
			if strings.HasPrefix(strings.ToUpper(sql), "SELECT") {
				sqlType = models.SQLTypeQuery
			}

			tasks = append(tasks, models.SQLTask{
				SQL:      normalizeSQL(sql, sqlType),
				Type:     sqlType,
				LineNum:  lineNum,
				Filename: path,
			})
			sqlBuffer.Reset()
			hasContent = false
		}
	}

	// 处理文件末尾的最后一条语句（如果没有分号）
	if hasContent {
		sql := strings.TrimSpace(sqlBuffer.String())
		sqlType := models.SQLTypeExec
		if strings.HasPrefix(strings.ToUpper(sql), "SELECT") {
			sqlType = models.SQLTypeQuery
		}

		tasks = append(tasks, models.SQLTask{
			SQL:      normalizeSQL(sql, sqlType),
			Type:     sqlType,
			LineNum:  lineNum,
			Filename: path,
		})
	}

	return tasks, scanner.Err()
}
