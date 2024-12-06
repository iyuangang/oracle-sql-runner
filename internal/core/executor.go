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
	if len(tasks) == 0 {
		return result
	}

	// 创建工作池
	workerCount := e.config.MaxConcurrent
	if workerCount > len(tasks) {
		workerCount = len(tasks)
	}

	// 创建任务通道
	taskChan := make(chan models.SQLTask, len(tasks))
	for _, task := range tasks {
		taskChan <- task
	}
	close(taskChan)

	// 创建结果通道
	type taskResult struct {
		task models.SQLTask
		err  error
	}
	resultChan := make(chan taskResult, len(tasks))

	// 启动工作协程
	var wg sync.WaitGroup
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for task := range taskChan {
				ctx, cancel := context.WithTimeout(context.Background(),
					time.Duration(e.config.Timeout)*time.Second)

				start := time.Now()
				err := e.executeTask(ctx, task)
				duration := time.Since(start)

				cancel() // 确保取消上下文

				resultChan <- taskResult{task: task, err: err}
				e.metrics.AddQuery(duration, err == nil)
			}
		}()
	}

	// 启动结果收集协程
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// 处理结果
	for res := range resultChan {
		if res.err != nil {
			e.logger.Error("SQL任务执行失败",
				"sql", res.task.SQL,
				"line", res.task.LineNum,
				"error", res.err)
			result.AddError(res.task, res.err)
		} else {
			e.logger.Debug("SQL执行成功",
				"sql", res.task.SQL,
				"line", res.task.LineNum)
			result.AddSuccess()
		}
	}

	return result
}

// executeTask 执行单个SQL任务
func (e *Executor) executeTask(ctx context.Context, task models.SQLTask) error {
	maxRetries := e.config.MaxRetries
	var lastErr error
	var err error

	for retry := 0; retry < maxRetries; retry++ {
		if retry > 0 {
			// 重试前等待一小段时间
			time.Sleep(time.Duration(retry*100) * time.Millisecond)
		}

		switch task.Type {
		case models.SQLTypeQuery:
			err = e.executeQuery(ctx, task.SQL)
		case models.SQLTypePLSQL:
			err = e.executePLSQL(ctx, task.SQL)
		default:
			_, err = e.pool.ExecContext(ctx, task.SQL)
		}

		if err == nil {
			return nil
		}

		lastErr = err
		e.logger.Warn("SQL执行失败，准备重试",
			"sql", task.SQL,
			"line", task.LineNum,
			"retry", retry+1,
			"error", err)

		// 如果是不可重试的错误，直接返回
		if !isRetryableError(err) {
			return err
		}
	}

	return lastErr
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
