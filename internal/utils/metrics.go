package utils

import (
	"fmt"
	"sync/atomic"
	"time"
)

// Metrics 性能指标收集器
type Metrics struct {
	StartTime     time.Time
	EndTime       time.Time
	QueryCount    int64
	SuccessCount  int64
	FailureCount  int64
	TotalDuration int64 // 纳秒
}

// NewMetrics 创建新的指标收集器
func NewMetrics() *Metrics {
	return &Metrics{
		StartTime: time.Now(),
	}
}

// Start 开始收集指标
func (m *Metrics) Start() {
	m.StartTime = time.Now()
}

// End 结束收集指标
func (m *Metrics) End() {
	m.EndTime = time.Now()
}

// AddQuery 添加查询统计
func (m *Metrics) AddQuery(duration time.Duration, success bool) {
	atomic.AddInt64(&m.QueryCount, 1)
	atomic.AddInt64(&m.TotalDuration, int64(duration))

	if success {
		atomic.AddInt64(&m.SuccessCount, 1)
	} else {
		atomic.AddInt64(&m.FailureCount, 1)
	}
}

// Duration 获取总执行时间
func (m *Metrics) Duration() time.Duration {
	return m.EndTime.Sub(m.StartTime)
}

// AverageDuration 获取平均执行时间
func (m *Metrics) AverageDuration() time.Duration {
	if m.QueryCount == 0 {
		return 0
	}
	return time.Duration(m.TotalDuration / m.QueryCount)
}

// String 获取指标字符串表示
func (m *Metrics) String() string {
	return fmt.Sprintf(
		"执行统计:\n"+
			"总执行时间: %v\n"+
			"总查询数: %d\n"+
			"成功数: %d\n"+
			"失败数: %d\n"+
			"平均执行时间: %v",
		m.Duration(),
		m.QueryCount,
		m.SuccessCount,
		m.FailureCount,
		m.AverageDuration(),
	)
}
