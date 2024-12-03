package models

import (
	"fmt"
	"sync"
	"time"
)

// SQLType 定义SQL语句类型
type SQLType string

const (
	SQLTypeQuery SQLType = "query"
	SQLTypeExec  SQLType = "exec"
	SQLTypePLSQL SQLType = "plsql"
)

// SQLTask 表示单个SQL任务
type SQLTask struct {
	SQL      string
	Type     SQLType
	LineNum  int
	Filename string
}

// Result SQL执行结果
type Result struct {
	mu        sync.Mutex // 添加互斥锁
	Success   int
	Failed    int
	Errors    []SQLError
	Duration  time.Duration
	StartTime time.Time
	EndTime   time.Time
}

// NewResult 创建新的结果对象
func NewResult() *Result {
	return &Result{
		StartTime: time.Now(),
	}
}

// AddError 添加错误信息
func (r *Result) AddError(task SQLTask, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.Failed++
	r.Errors = append(r.Errors, SQLError{
		SQL:     task.SQL,
		Message: err.Error(),
		Line:    task.LineNum,
		File:    task.Filename,
	})
}

// AddSuccess 添加成功计数
func (r *Result) AddSuccess() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.Success++
}

// Finish 完成执行
func (r *Result) Finish() {
	r.EndTime = time.Now()
}

// Print 打印结果
func (r *Result) Print() {
	fmt.Printf("\n执行结果:\n")
	fmt.Printf("总语句数: %d\n", r.Success+r.Failed)
	fmt.Printf("成功: %d\n", r.Success)
	fmt.Printf("失败: %d\n", r.Failed)
	fmt.Printf("总执行时间: %.2f秒\n", r.Duration.Seconds())

	if r.Failed > 0 {
		fmt.Printf("\n错误详情:\n")
		for i, err := range r.Errors {
			fmt.Printf("%d. %s\n", i+1, err.Error())
		}
	}
}
