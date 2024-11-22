package executor

import (
	"errors"
	"fmt"
	"strings"
)

// 定义错误类型
var (
	ErrInvalidSQL     = errors.New("无效的SQL语句")
	ErrConnectionLost = errors.New("数据库连接丢失")
	ErrTimeout        = errors.New("执行超时")
	ErrCancelled      = errors.New("执行被取消")
)

// SQLError 包装SQL执行错误
type SQLError struct {
	SQL       string
	Err       error
	LineNum   int
	Retryable bool
}

func (e *SQLError) Error() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("SQL执行错误 (行号: %d): %v", e.LineNum, e.Err))
	if e.SQL != "" {
		sb.WriteString(fmt.Sprintf("\nSQL: %s", e.SQL))
	}
	return sb.String()
}

func (e *SQLError) Unwrap() error {
	return e.Err
}

// IsRetryableError 判断错误是否可重试
func IsRetryableError(err error) bool {
	var sqlErr *SQLError
	if errors.As(err, &sqlErr) {
		return sqlErr.Retryable
	}
	return false
}
