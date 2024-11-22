package executor

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/iyuangang/oracle-sql-runner/config"
	"github.com/iyuangang/oracle-sql-runner/logger"
	"github.com/iyuangang/oracle-sql-runner/utils"
)

var (
	ErrPoolTimeout = errors.New("连接池超时")
	ErrExecTimeout = errors.New("执行超时")
)

type ExecutionResult struct {
	SuccessCount int
	FailureCount int
	Errors       []error
	Duration     time.Duration
}

type SQLExecutor struct {
	pool       *ConnectionPool
	config     *config.DatabaseConfig
	execConfig *config.ExecutionConfig
	parser     *SQLParser
	ctx        context.Context
	cancel     context.CancelFunc
	progress   *utils.Progress
	results    chan *ExecutionResult
	errors     chan error
	wg         sync.WaitGroup
}

func NewSQLExecutor(dbConfig *config.DatabaseConfig, execConfig *config.ExecutionConfig) (*SQLExecutor, error) {
	// 创建连接池
	pool, err := NewConnectionPool(dbConfig)
	if err != nil {
		return nil, fmt.Errorf("创建连接池失败: %w", err)
	}

	// 创建上下文（带超时）
	ctx, cancel := context.WithTimeout(context.Background(), dbConfig.Timeout)

	executor := &SQLExecutor{
		pool:       pool,
		config:     dbConfig,
		execConfig: execConfig,
		ctx:        ctx,
		cancel:     cancel,
		results:    make(chan *ExecutionResult, execConfig.ParallelDegree),
		errors:     make(chan error, execConfig.ParallelDegree),
	}

	// 启用DBMS输出（如果配置）
	if dbConfig.EnableDBMSOut {
		if err := executor.enableDBMSOutput(); err != nil {
			return nil, err
		}
	}

	return executor, nil
}

func (e *SQLExecutor) ExecuteFile(filePath string) (*ExecutionResult, error) {
	defer e.cancel()
	startTime := time.Now()

	// 检查文件大小并可能分割
	files, err := utils.SplitSQLFile(filePath, e.execConfig.MaxFileSize)
	if err != nil {
		return nil, fmt.Errorf("处理SQL文件失败: %w", err)
	}

	totalResult := &ExecutionResult{}

	for _, file := range files {
		result, err := e.executeOneFile(file)
		if err != nil {
			return totalResult, err
		}
		totalResult.merge(result)
	}

	totalResult.Duration = time.Since(startTime)
	return totalResult, nil
}

func (e *SQLExecutor) executeOneFile(filePath string) (*ExecutionResult, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("打开SQL文件失败: %w", err)
	}
	defer file.Close()

	e.parser = NewSQLParser(file)
	statements := make(chan string, e.execConfig.BatchSize)

	// 启动工作协程
	for i := 0; i < e.execConfig.ParallelDegree; i++ {
		e.wg.Add(1)
		go e.worker(statements)
	}

	// 读取SQL语句
	go func() {
		defer close(statements)
		for {
			stmt, err := e.parser.NextStatement()
			if err == io.EOF {
				break
			}
			if err != nil {
				e.errors <- fmt.Errorf("解析SQL失败: %w", err)
				return
			}
			if stmt = strings.TrimSpace(stmt); stmt != "" {
				statements <- stmt
			}
		}
	}()

	// 等待所有工作完成
	e.wg.Wait()
	close(e.results)
	close(e.errors)

	// 收集结果
	result := &ExecutionResult{}
	for r := range e.results {
		result.merge(r)
	}

	// 检查错误
	for err := range e.errors {
		if err != nil {
			return result, err
		}
	}

	return result, nil
}

func (e *SQLExecutor) worker(statements chan string) {
	defer e.wg.Done()

	result := &ExecutionResult{}

	for stmt := range statements {
		// 获取数据库连接
		conn, err := e.pool.Acquire(e.ctx)
		if err != nil {
			e.errors <- fmt.Errorf("获取数据库连接失败: %w", err)
			return
		}

		// 执行SQL语句
		if err := e.executeStatement(conn, stmt); err != nil {
			result.FailureCount++
			result.Errors = append(result.Errors, err)
			logger.Error("执行SQL失败", "error", err, "sql", stmt)
		} else {
			result.SuccessCount++
		}

		// 释放连接
		e.pool.Release(conn)
	}

	e.results <- result
}

func (e *SQLExecutor) executeStatement(conn *sql.Conn, stmt string) error {
	sqlType := DetermineSQLType(stmt)

	var err error
	retries := 0

	for retries <= e.config.MaxRetries {
		switch sqlType {
		case SQLTypeDML:
			err = e.executeDML(conn, stmt)
		case SQLTypePLSQL, SQLTypeAnonymousBlock:
			err = e.executePLSQL(conn, stmt)
		case SQLTypeQuery:
			err = e.executeQuery(conn, stmt)
		default:
			err = e.executeDirectly(conn, stmt)
		}

		if err == nil {
			return nil
		}

		// 检查是否需要重试
		if !e.shouldRetry(err) {
			return err
		}

		retries++
		if retries <= e.config.MaxRetries {
			time.Sleep(e.execConfig.RetryInterval)
			logger.Warn("重试执行SQL", "attempt", retries, "max", e.config.MaxRetries)
		}
	}

	return err
}

func (e *SQLExecutor) executeDML(conn *sql.Conn, stmt string) error {
	if !e.config.AutoCommit {
		tx, err := conn.BeginTx(e.ctx, nil)
		if err != nil {
			return fmt.Errorf("开始事务失败: %w", err)
		}
		defer func() {
			if err != nil {
				tx.Rollback()
			}
		}()

		if _, err = tx.ExecContext(e.ctx, stmt); err != nil {
			return fmt.Errorf("执行DML失败: %w", err)
		}

		return tx.Commit()
	}

	_, err := conn.ExecContext(e.ctx, stmt)
	return err
}

func (e *SQLExecutor) executePLSQL(conn *sql.Conn, stmt string) error {
	// 移除结尾的/
	stmt = strings.TrimSuffix(strings.TrimSpace(stmt), "/")

	_, err := conn.ExecContext(e.ctx, stmt)
	if err != nil {
		return fmt.Errorf("执行PL/SQL失败: %w", err)
	}

	// 如果启用了DBMS输出，获取输出内容
	if e.config.EnableDBMSOut {
		output, err := e.getDBMSOutput(conn)
		if err != nil {
			logger.Warn("获取DBMS输出失败", "error", err)
		} else if output != "" {
			logger.Info("DBMS输出", "output", output)
		}
	}

	return nil
}

func (e *SQLExecutor) executeQuery(conn *sql.Conn, stmt string) error {
	rows, err := conn.QueryContext(e.ctx, stmt)
	if err != nil {
		return fmt.Errorf("执行查询失败: %w", err)
	}
	defer rows.Close()

	// 仅验证查询是否可以执行
	return nil
}

func (e *SQLExecutor) executeDirectly(conn *sql.Conn, stmt string) error {
	_, err := conn.ExecContext(e.ctx, stmt)
	return err
}

func (e *SQLExecutor) enableDBMSOutput() error {
	conn, err := e.pool.Acquire(e.ctx)
	if err != nil {
		return err
	}
	defer e.pool.Release(conn)

	_, err = conn.ExecContext(e.ctx, "BEGIN DBMS_OUTPUT.ENABLE(NULL); END;")
	return err
}

func (e *SQLExecutor) getDBMSOutput(conn *sql.Conn) (string, error) {
	var output strings.Builder
	var line string
	var status int

	for {
		err := conn.QueryRowContext(e.ctx, `
            BEGIN
                DBMS_OUTPUT.GET_LINE(:1, :2);
            END;
        `, sql.Out{Dest: &line}, sql.Out{Dest: &status}).Scan(&line, &status)
		if err != nil {
			return "", err
		}

		if status != 0 {
			break
		}

		output.WriteString(line + "\n")
	}

	return output.String(), nil
}

func (e *SQLExecutor) shouldRetry(err error) bool {
	// 可以根据具体的Oracle错误代码判断是否需要重试
	return true // 简化版本
}

func (e *SQLExecutor) Close() error {
	if e.pool != nil {
		return e.pool.Close()
	}
	return nil
}

func (r *ExecutionResult) merge(other *ExecutionResult) {
	r.SuccessCount += other.SuccessCount
	r.FailureCount += other.FailureCount
	r.Errors = append(r.Errors, other.Errors...)
}
