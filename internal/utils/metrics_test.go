package utils

import (
	"strings"
	"testing"
	"time"
)

func TestNewMetrics(t *testing.T) {
	m := NewMetrics()
	if m == nil {
		t.Fatal("NewMetrics() returned nil")
	}
	if m.StartTime.IsZero() {
		t.Error("StartTime should be initialized")
	}
	if m.QueryCount != 0 {
		t.Errorf("QueryCount = %d, want 0", m.QueryCount)

	}
}

func TestMetrics_StartEnd(t *testing.T) {
	m := NewMetrics()
	originalStart := m.StartTime

	// 等待一小段时间后重新开始
	time.Sleep(10 * time.Millisecond)
	m.Start()
	if m.StartTime.Equal(originalStart) {
		t.Error("Start() did not update StartTime")
	}

	// 等待一小段时间后结束
	time.Sleep(10 * time.Millisecond)
	m.End()
	if m.EndTime.IsZero() {
		t.Error("End() did not set EndTime")
	}
	if !m.EndTime.After(m.StartTime) {
		t.Error("EndTime is not after StartTime")
	}
}

func TestMetrics_AddQuery(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		success  bool
		want     *Metrics
	}{
		{
			name:     "成功查询",
			duration: 100 * time.Millisecond,
			success:  true,
			want: &Metrics{
				QueryCount:    1,
				SuccessCount:  1,
				FailureCount:  0,
				TotalDuration: int64(100 * time.Millisecond),
			},
		},
		{
			name:     "失败查询",
			duration: 50 * time.Millisecond,
			success:  false,
			want: &Metrics{
				QueryCount:    1,
				SuccessCount:  0,
				FailureCount:  1,
				TotalDuration: int64(50 * time.Millisecond),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewMetrics()
			m.AddQuery(tt.duration, tt.success)

			if m.QueryCount != tt.want.QueryCount {
				t.Errorf("QueryCount = %d, want %d", m.QueryCount, tt.want.QueryCount)
			}
			if m.SuccessCount != tt.want.SuccessCount {
				t.Errorf("SuccessCount = %d, want %d", m.SuccessCount, tt.want.SuccessCount)
			}
			if m.FailureCount != tt.want.FailureCount {
				t.Errorf("FailureCount = %d, want %d", m.FailureCount, tt.want.FailureCount)
			}
			if m.TotalDuration != tt.want.TotalDuration {
				t.Errorf("TotalDuration = %d, want %d", m.TotalDuration, tt.want.TotalDuration)
			}
		})
	}

	// 测试并发安全性
	t.Run("并发安全性", func(t *testing.T) {
		m := NewMetrics()
		done := make(chan bool)
		iterations := 100

		worker := func() {
			for i := 0; i < iterations; i++ {
				m.AddQuery(time.Millisecond, true)
			}
			done <- true
		}

		// 启动多个 goroutine
		numWorkers := 10
		for i := 0; i < numWorkers; i++ {
			go worker()
		}

		// 等待所有 goroutine 完成
		for i := 0; i < numWorkers; i++ {
			<-done
		}

		expectedQueries := int64(numWorkers * iterations)
		if m.QueryCount != expectedQueries {
			t.Errorf("并发测试失败: QueryCount = %d, want %d", m.QueryCount, expectedQueries)
		}
		if m.SuccessCount != expectedQueries {
			t.Errorf("并发测试失败: SuccessCount = %d, want %d", m.SuccessCount, expectedQueries)
		}
	})
}

func TestMetrics_Duration(t *testing.T) {
	tests := []struct {
		name      string
		setupFunc func() *Metrics
		wantFunc  func(time.Duration) bool
	}{
		{
			name: "未结束的度量",
			setupFunc: func() *Metrics {
				m := NewMetrics()
				time.Sleep(10 * time.Millisecond)
				return m
			},
			wantFunc: func(d time.Duration) bool {
				return d >= 10*time.Millisecond
			},
		},
		{
			name: "已结束的度量",
			setupFunc: func() *Metrics {
				m := NewMetrics()
				time.Sleep(10 * time.Millisecond)
				m.End()
				return m
			},
			wantFunc: func(d time.Duration) bool {
				return d >= 10*time.Millisecond
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := tt.setupFunc()
			duration := m.Duration()
			if !tt.wantFunc(duration) {
				t.Errorf("Duration() = %v, want >= 10ms", duration)
			}
		})
	}
}

func TestMetrics_AverageDuration(t *testing.T) {
	tests := []struct {
		name  string
		setup func(*Metrics)
		want  time.Duration
	}{
		{
			name:  "无查询",
			setup: func(m *Metrics) {},
			want:  0,
		},
		{
			name: "单次查询",
			setup: func(m *Metrics) {
				m.AddQuery(100*time.Millisecond, true)
			},
			want: 100 * time.Millisecond,
		},
		{
			name: "多次查询",
			setup: func(m *Metrics) {
				m.AddQuery(100*time.Millisecond, true)
				m.AddQuery(200*time.Millisecond, true)
			},
			want: 150 * time.Millisecond,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewMetrics()
			tt.setup(m)
			got := m.AverageDuration()
			if got != tt.want {
				t.Errorf("AverageDuration() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMetrics_String(t *testing.T) {
	m := NewMetrics()
	m.AddQuery(100*time.Millisecond, true)
	m.AddQuery(200*time.Millisecond, false)
	m.End()

	result := m.String()

	// 验证包含所有必要信息
	requiredFields := []string{
		"执行统计",
		"总执行时间",
		"总查询数: 2",
		"成功数: 1",
		"失败数: 1",
		"平均执行时间",
	}

	for _, field := range requiredFields {
		if !strings.Contains(result, field) {
			t.Errorf("String() output missing field: %s", field)
		}
	}
}
