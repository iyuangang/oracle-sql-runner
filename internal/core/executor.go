package core

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"sync"
	"time"

	_ "github.com/godror/godror"
	"github.com/iyuangang/oracle-sql-runner/internal/config"
	"github.com/iyuangang/oracle-sql-runner/internal/db"
	"github.com/iyuangang/oracle-sql-runner/internal/utils"
	"github.com/iyuangang/oracle-sql-runner/pkg/models"
)

// Executor 核心执行器
type Executor struct {
	pool    *db.Pool
	logger  *utils.Logger
	config  *config.Config
	metrics *utils.Metrics
}

// NewExecutor 创建新的执行器
func NewExecutor(cfg *config.Config, dbName string, logger *utils.Logger) (*Executor, error) {
	dbConfig, ok := cfg.Databases[dbName]
	if !ok {
		return nil, fmt.Errorf("未找到数据库配置: %s", dbName)
	}

	pool, err := db.NewPool(&dbConfig, logger)
	if err != nil {
		return nil, fmt.Errorf("创建连接池失败: %w", err)
	}

	return &Executor{
		pool:    pool,
		logger:  logger,
		config:  cfg,
		metrics: utils.NewMetrics(),
	}, nil
}

// ExecuteFile 执行SQL文件
func (e *Executor) ExecuteFile(path string) *models.Result {
	e.logger.Info("开始执行SQL文件", "file", path)
	e.metrics.Start()

	// 解析SQL文件
	tasks, err := ParseFile(path)
	if err != nil {
		e.logger.Error("解析SQL文件失败", "error", err)
		return models.NewErrorResult(err)
	}

	// 执行SQL任务
	result := e.executeParallel(tasks)

	e.metrics.End()
	e.logger.Info("SQL文件执行完成",
		"success", result.Success,
		"failed", result.Failed,
		"duration", e.metrics.Duration())
	result.Duration = e.metrics.Duration()

	return result
}

// executeParallel 并行执行SQL任务
func (e *Executor) executeParallel(tasks []models.SQLTask) *models.Result {
	result := models.NewResult()
	sem := make(chan struct{}, e.config.MaxConcurrent)
	var wg sync.WaitGroup
	var mu sync.Mutex // 添加互斥锁保护结果对象

	for _, task := range tasks {
		wg.Add(1)
		sem <- struct{}{}

		go func(t models.SQLTask) {
			defer wg.Done()
			defer func() { <-sem }()

			ctx, cancel := context.WithTimeout(context.Background(), time.Duration(e.config.Timeout)*time.Second)
			defer cancel()

			start := time.Now()
			err := e.executeTask(ctx, t)
			duration := time.Since(start)

			mu.Lock() // 加锁保护结果对象的修改
			defer mu.Unlock()

			if err != nil {
				e.logger.Error("SQL任务执行失败",
					"sql", t.SQL,
					"line", t.LineNum,
					"error", err)
				result.AddError(t, err)
				e.metrics.AddQuery(duration, false)
			} else {
				e.logger.Debug("SQL执行成功",
					"sql", t.SQL,
					"line", t.LineNum,
					"duration", duration)
				result.AddSuccess()
				e.metrics.AddQuery(duration, true)
			}
		}(task)
	}

	wg.Wait()
	return result
}

// executeTask 执行单个SQL任务
func (e *Executor) executeTask(ctx context.Context, task models.SQLTask) error {
	var err error
	for i := 0; i < e.config.MaxRetries; i++ {
		if i > 0 {
			e.logger.Info("重试执行SQL",
				"attempt", i+1,
				"sql", task.SQL)
			time.Sleep(time.Second * time.Duration(i))
		}

		switch task.Type {
		case models.SQLTypeQuery:
			err = e.executeQuery(ctx, task.SQL)
		case models.SQLTypePLSQL:
			err = e.executePLSQL(ctx, task.SQL)
		default:
			err = e.executeSQL(ctx, task.SQL)
		}

		if err == nil {
			return nil
		}

		if !isRetryableError(err) {
			return err
		}
	}

	return err
}

// executeQuery 执行查询
func (e *Executor) executeQuery(ctx context.Context, sql string) error {
	rows, err := e.pool.QueryContext(ctx, sql)
	if err != nil {
		return err
	}
	defer rows.Close()

	return printQueryResults(rows)
}

// executePLSQL 执行PL/SQL块
func (e *Executor) executePLSQL(ctx context.Context, sql string) error {
	_, err := e.pool.ExecContext(ctx, sql)
	return err
}

// executeSQL 执行普通SQL
func (e *Executor) executeSQL(ctx context.Context, sql string) error {
	_, err := e.pool.ExecContext(ctx, sql)
	return err
}

// isRetryableError 判断是否可重试的错误
func isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	// Oracle 错误码映射
	oracleErrors := map[int]bool{
		1033:  true, // ORACLE initialization or shutdown in progress
		1034:  true, // ORACLE not available
		1089:  true, // immediate shutdown in progress
		1090:  true, // shutdown in progress
		1092:  true, // ORACLE instance terminated
		3113:  true, // end-of-file on communication channel
		3114:  true, // not connected to ORACLE
		12153: true, // TNS:not connected
		12154: true, // TNS:could not resolve service name
		12170: true, // TNS:Connect timeout occurred
		12171: true, // TNS:could not resolve connect identifier
		12257: true, // TNS:protocol adapter not loadable
		12514: true, // TNS:listener does not currently know of service requested
		12528: true, // TNS:listener: all appropriate instances are blocking new connections
		12537: true, // TNS:connection closed
		12541: true, // TNS:no listener
		12571: true, // TNS:packet writer failure
	}

	errStr := err.Error()
	// 提取错误码
	var errorCode int
	_, scanErr := fmt.Sscanf(errStr, "ORA-%d:", &errorCode)
	if scanErr == nil {
		return oracleErrors[errorCode]
	}

	// 检查网络相关错误
	networkErrors := []string{
		"connection refused",
		"connection reset",
		"connection timed out",
		"no route to host",
		"network is unreachable",
	}

	for _, netErr := range networkErrors {
		if strings.Contains(strings.ToLower(errStr), netErr) {
			return true
		}
	}

	return false
}

// Close 关闭执行器
func (e *Executor) Close() error {
	return e.pool.Close()
}

// printQueryResults 打印查询结果
func printQueryResults(rows *sql.Rows) error {
	// 获取列信息
	columns, err := rows.Columns()
	if err != nil {
		return err
	}

	// 打印列头
	for i, col := range columns {
		if i > 0 {
			fmt.Print("\t")
		}
		fmt.Print(col)
	}
	fmt.Println()
	fmt.Println(strings.Repeat("-", 80))

	// 准备扫描目标
	values := make([]interface{}, len(columns))
	scanArgs := make([]interface{}, len(columns))
	for i := range values {
		scanArgs[i] = &values[i]
	}

	// 打印数据行
	rowCount := 0
	for rows.Next() {
		err := rows.Scan(scanArgs...)
		if err != nil {
			return err
		}

		for i, value := range values {
			if i > 0 {
				fmt.Print("\t")
			}
			switch v := value.(type) {
			case nil:
				fmt.Print("NULL")
			case []byte:
				fmt.Print(string(v))
			case time.Time:
				fmt.Print(v.Format("2006-01-02 15:04:05"))
			default:
				fmt.Print(v)
			}
		}
		fmt.Println()
		rowCount++
	}

	// 打印统计信息
	fmt.Printf("\n共返回 %d 行数据\n", rowCount)
	fmt.Println(strings.Repeat("-", 80))

	return rows.Err()
}
