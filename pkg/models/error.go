package models

import "fmt"

// SQLError 自定义SQL错误
type SQLError struct {
	SQL     string
	Message string
	Line    int
	File    string
}

func NewErrorResult(err error) *Result {
	return &Result{
		Success: 0,
		Errors:  []error{err},
	}
}

func (e *SQLError) Error() string {
	return fmt.Sprintf("SQL错误 [%s:%d]: %s\nSQL: %s", e.File, e.Line, e.Message, e.SQL)
}

// NewSQLError 创建新的SQL错误
func NewSQLError(sql string, message string, line int, file string) *SQLError {
	return &SQLError{
		SQL:     sql,
		Message: message,
		Line:    line,
		File:    file,
	}
}
