package core

import (
	"bufio"
	"os"
	"strings"

	"github.com/iyuangang/oracle-sql-runner/pkg/models"
)

// ParseFile 解析SQL文件
func ParseFile(path string) ([]models.SQLTask, error) {
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

	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())

		// 跳过空行和注释
		if line == "" || strings.HasPrefix(line, "--") {
			continue
		}

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
				SQL:      sql,
				Type:     models.SQLTypePLSQL,
				LineNum:  lineNum,
				Filename: path,
			})
			sqlBuffer.Reset()
			inPLSQLBlock = false
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
				SQL:      sql,
				Type:     sqlType,
				LineNum:  lineNum,
				Filename: path,
			})
			sqlBuffer.Reset()
		}
	}

	return tasks, scanner.Err()
}
