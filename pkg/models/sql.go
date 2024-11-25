package models

import (
	"fmt"
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

// Result 表示执行结果
type Result struct {
	Success   int
	Failed    int
	Errors    []error
	StartTime time.Time
	EndTime   time.Time
}

// NewResult 创建新的结果对象
func NewResult() *Result {
	return &Result{
		StartTime: time.Now(),
	}
}

// AddError 添加错误
func (r *Result) AddError(task SQLTask, err error) {
	r.Failed++
	r.Errors = append(r.Errors, fmt.Errorf("文件 %s 第 %d 行: %v", task.Filename, task.LineNum, err))
}

// AddSuccess 添加成功
func (r *Result) AddSuccess() {
	r.Success++
}

// Finish 完成执行
func (r *Result) Finish() {
	r.EndTime = time.Now()
}

// Print 打印结果
func (r *Result) Print() {
	duration := r.EndTime.Sub(r.StartTime)

	fmt.Printf("\n执行结果:\n")
	fmt.Printf("总执行时间: %v\n", duration)
	fmt.Printf("成功: %d\n", r.Success)
	fmt.Printf("失败: %d\n", r.Failed)

	if r.Failed > 0 {
		fmt.Printf("\n错误详情:\n")
		for i, err := range r.Errors {
			fmt.Printf("%d. %s\n", i+1, err)
		}
	}
}
