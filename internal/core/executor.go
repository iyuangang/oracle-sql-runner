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

// 添加一个新的结构体来存储输出
type outputCapture struct {
	mu      sync.Mutex
	outputs []string
}

func (o *outputCapture) Write(p []byte) (n int, err error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.outputs = append(o.outputs, string(p))
	return len(p), nil
}

func (o *outputCapture) Print() {
	o.mu.Lock()
	defer o.mu.Unlock()
	for _, output := range o.outputs {
		fmt.Print(output)
	}
}

// taskResult 定义任务执行结果
type taskResult struct {
	task models.SQLTask
	err  error
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
	resultChan := make(chan taskResult, len(tasks))

	// 创建输出捕获器
	output := &outputCapture{}

	// 启动工作协程
	var wg sync.WaitGroup
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for task := range taskChan {
				// 增加超时时间，默认设置为30秒
				timeout := 30 * time.Second
				if e.config.Timeout > 0 {
					timeout = time.Duration(e.config.Timeout) * time.Second
				}

				ctx, cancel := context.WithTimeout(context.Background(), timeout)

				start := time.Now()
				err := e.executeTaskWithOutput(ctx, task, output)
				duration := time.Since(start)

				cancel()

				resultChan <- taskResult{task: task, err: err}
				e.metrics.AddQuery(duration, err == nil)
			}
		}()
	}

	// 等待所有任务完成
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// 处理结果
	result = processResults(resultChan)

	// 打印捕获的输出
	output.Print()

	return result
}

// processResults 处理执行结果
func processResults(resultChan chan taskResult) *models.Result {
	result := models.NewResult()

	// 处理所有任务的结果
	for res := range resultChan {
		if res.err != nil {
			result.AddError(res.task, res.err)
		} else {
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

// executeTaskWithOutput 执行单个SQL任务并捕获输出
func (e *Executor) executeTaskWithOutput(ctx context.Context, task models.SQLTask, output *outputCapture) error {
	maxRetries := 3
	var lastErr error

	for retry := 0; retry < maxRetries; retry++ {
		if retry > 0 {
			time.Sleep(time.Duration(retry*100) * time.Millisecond)
		}

		fmt.Fprintf(output, "\n执行任务: Type=%v, SQL=%v\n", task.Type, task.SQL)

		var err error
		switch task.Type {
		case models.SQLTypeQuery:
			fmt.Fprintf(output, "执行查询语句\n")
			err = e.executeQueryWithOutput(ctx, task.SQL, output)
		case models.SQLTypePLSQL:
			fmt.Fprintf(output, "执行PL/SQL块\n")
			err = e.executePLSQL(ctx, task.SQL)
		default:
			fmt.Fprintf(output, "执行普通SQL\n")
			_, err = e.pool.ExecContext(ctx, task.SQL)
		}

		if err == nil {
			return nil
		}

		lastErr = err
		fmt.Fprintf(output, "运行失败, 错误: %v\n", err)

		if !isRetryableError(lastErr) {
			return lastErr
		}
	}

	return lastErr
}

// executeQueryWithOutput 执行查询并捕获输出
func (e *Executor) executeQueryWithOutput(ctx context.Context, sql string, output *outputCapture) error {
	fmt.Fprintf(output, "\n开始执行查询: %v\n", sql)

	// 添加错误处理和重试逻辑
	var lastErr error
	for retry := 0; retry < 3; retry++ {
		if retry > 0 {
			fmt.Fprintf(output, "重试查询 (第 %d 次)\n", retry+1)
			time.Sleep(time.Duration(retry*100) * time.Millisecond)
		}

		rows, err := e.pool.QueryContext(ctx, sql)
		if err != nil {
			lastErr = err
			fmt.Fprintf(output, "查询执行失败: %v\n", err)
			if ctx.Err() == context.DeadlineExceeded {
				return fmt.Errorf("查询超时: %w", err)
			}
			continue
		}
		defer rows.Close()

		fmt.Fprintf(output, "查询执行成功，准备打印结果\n")
		return printQueryResultsWithOutput(rows, output)
	}

	return fmt.Errorf("查询执行失败: %w", lastErr)
}

// printQueryResultsWithOutput 打印查询结果并捕获输出
func printQueryResultsWithOutput(rows *sql.Rows, output *outputCapture) error {
	fmt.Fprintf(output, "\n=== 开始打印查询结果 ===\n")

	columns, err := rows.Columns()
	if err != nil {
		fmt.Fprintf(output, "获取列信息失败: %v\n", err)
		return fmt.Errorf("获取列信息失败: %w", err)
	}

	// 打印列头
	for i, col := range columns {
		if i > 0 {
			fmt.Fprintf(output, "\t")
		}
		fmt.Fprintf(output, "%s", col)
	}
	fmt.Fprintf(output, "\n")
	fmt.Fprintf(output, "%s\n", strings.Repeat("-", 80))

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
			return fmt.Errorf("扫描行数据失败: %w", err)
		}

		for i, value := range values {
			if i > 0 {
				fmt.Fprintf(output, "\t")
			}
			switch v := value.(type) {
			case nil:
				fmt.Fprintf(output, "NULL")
			case []byte:
				fmt.Fprintf(output, "%s", string(v))
			case time.Time:
				fmt.Fprintf(output, "%s", v.Format("2006-01-02 15:04:05"))
			default:
				fmt.Fprintf(output, "%v", v)
			}
		}
		fmt.Fprintf(output, "\n")
		rowCount++
	}

	fmt.Fprintf(output, "\n共返回 %d 行数据\n", rowCount)
	fmt.Fprintf(output, "%s\n", strings.Repeat("-", 80))
	fmt.Fprintf(output, "=== 结束打印查询结果 ===\n\n")

	return rows.Err()
}
