package executor

import (
	"bufio"
	"io"
	"strings"
	"unicode"
)

type SQLParser struct {
	reader         *bufio.Reader
	delimiter      string
	currentLine    int
	currentStmt    strings.Builder
	inComment      bool
	inString       bool
	inMultiComment bool
}

func NewSQLParser(reader io.Reader) *SQLParser {
	return &SQLParser{
		reader:      bufio.NewReader(reader),
		delimiter:   ";",
		currentLine: 1,
	}
}

func (p *SQLParser) NextStatement() (string, error) {
	p.currentStmt.Reset()
	var previousChar rune

	for {
		char, _, err := p.reader.ReadRune()
		if err != nil {
			if err == io.EOF && p.currentStmt.Len() > 0 {
				return p.finalizeStatement()
			}
			return "", err
		}

		// 处理换行
		if char == '\n' {
			p.currentLine++
		}

		// 处理字符串
		if char == '\'' && !p.inComment && !p.inMultiComment {
			if !p.inString {
				p.inString = true
			} else if previousChar != '\\' {
				p.inString = false
			}
		}

		// 处理多行注释
		if !p.inString {
			if char == '/' && previousChar == '*' && p.inMultiComment {
				p.inMultiComment = false
				previousChar = char
				continue
			}
			if char == '*' && previousChar == '/' && !p.inMultiComment {
				p.inMultiComment = true
				previousChar = char
				continue
			}
		}

		// 处理单行注释
		if !p.inString && !p.inMultiComment {
			if char == '-' && previousChar == '-' {
				// 跳过本行
				p.skipLine()
				continue
			}
		}

		// 如果在注释中，跳过
		if p.inMultiComment {
			previousChar = char
			continue
		}

		// 检查PL/SQL块
		if !p.inString && char == '/' && p.isPLSQLBlock() {
			return p.finalizePLSQLBlock()
		}

		// 检查语句结束
		if !p.inString && char == ';' {
			stmt := p.currentStmt.String()
			if strings.TrimSpace(stmt) != "" {
				return strings.TrimSpace(stmt), nil
			}
			p.currentStmt.Reset()
			continue
		}

		// 添加字符到当前语句
		p.currentStmt.WriteRune(char)
		previousChar = char
	}
}

func (p *SQLParser) skipLine() error {
	for {
		char, _, err := p.reader.ReadRune()
		if err != nil {
			return err
		}
		if char == '\n' {
			p.currentLine++
			return nil
		}
	}
}

func (p *SQLParser) isPLSQLBlock() bool {
	stmt := strings.TrimSpace(strings.ToUpper(p.currentStmt.String()))
	return strings.HasPrefix(stmt, "CREATE OR REPLACE") ||
		strings.HasPrefix(stmt, "DECLARE") ||
		strings.HasPrefix(stmt, "BEGIN")
}

func (p *SQLParser) finalizePLSQLBlock() (string, error) {
	stmt := strings.TrimSpace(p.currentStmt.String())
	p.currentStmt.Reset()
	return stmt, nil
}

func (p *SQLParser) finalizeStatement() (string, error) {
	stmt := strings.TrimSpace(p.currentStmt.String())
	if strings.HasSuffix(stmt, ";") {
		stmt = stmt[:len(stmt)-1]
	}
	return strings.TrimSpace(stmt), nil
}

// 辅助方法：检查是否为空白字符
func isWhitespace(r rune) bool {
	return unicode.IsSpace(r) || r == '\n' || r == '\r' || r == '\t'
}
