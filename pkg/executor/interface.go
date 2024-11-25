package executor

import "github.com/iyuangang/oracle-sql-runner/pkg/models"

// SQLExecutor 定义执行器接口
type SQLExecutor interface {
	ExecuteFile(path string) *models.Result
	ExecuteSQL(sql string) error
	Close() error
}

// QueryResult 查询结果接口
type QueryResult interface {
	Columns() []string
	Next() bool
	Scan(dest ...interface{}) error
	Close() error
}

// Transaction 事务接口
type Transaction interface {
	Exec(sql string) error
	Query(sql string) (QueryResult, error)
	Commit() error
	Rollback() error
}
