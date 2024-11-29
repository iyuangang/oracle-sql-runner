package models

import (
	"bytes"
	"errors"
	"io"
	"os"
	"strings"
	"testing"
	"time"
)

func TestNewResult(t *testing.T) {
	result := NewResult()

	if result.Success != 0 {
		t.Errorf("Expected Success to be 0, got %d", result.Success)
	}
	if result.Failed != 0 {
		t.Errorf("Expected Failed to be 0, got %d", result.Failed)
	}
	if len(result.Errors) != 0 {
		t.Errorf("Expected empty Errors, got %d errors", len(result.Errors))
	}
	if result.StartTime.IsZero() {
		t.Error("Expected StartTime to be set")

	}
}

func TestResult_AddError(t *testing.T) {
	result := NewResult()
	task := SQLTask{
		SQL:      "SELECT * FROM test",
		Type:     SQLTypeQuery,
		LineNum:  10,
		Filename: "test.sql",
	}
	err := errors.New("test error")

	result.AddError(task, err)

	if result.Failed != 1 {
		t.Errorf("Expected Failed to be 1, got %d", result.Failed)
	}
	if len(result.Errors) != 1 {
		t.Errorf("Expected 1 error, got %d", len(result.Errors))
	}

	errorStr := result.Errors[0].Error()
	expectedParts := []string{
		"test.sql",
		"10",
		"test error",
	}
	for _, part := range expectedParts {
		if !strings.Contains(errorStr, part) {
			t.Errorf("Error message should contain %q, got %q", part, errorStr)
		}
	}
}

func TestResult_AddSuccess(t *testing.T) {
	result := NewResult()
	result.AddSuccess()

	if result.Success != 1 {
		t.Errorf("Expected Success to be 1, got %d", result.Success)
	}
}

func TestResult_Finish(t *testing.T) {
	result := NewResult()
	time.Sleep(10 * time.Millisecond) // 确保有可测量的持续时间
	result.Finish()

	if result.EndTime.IsZero() {
		t.Error("Expected EndTime to be set")
	}
	if result.EndTime.Before(result.StartTime) {
		t.Error("EndTime should not be before StartTime")
	}
}

func TestResult_Print(t *testing.T) {
	tests := []struct {
		name   string
		setup  func(*Result)
		verify func(string) error
	}{
		{
			name: "成功执行",
			setup: func(r *Result) {
				r.AddSuccess()
				r.AddSuccess()
				r.Duration = 1 * time.Second
				r.Finish()
			},
			verify: func(output string) error {
				expectedParts := []string{
					"总语句数: 2",
					"成功: 2",
					"失败: 0",
					"总执行时间",
				}
				for _, part := range expectedParts {
					if !strings.Contains(output, part) {
						return errors.New("missing expected output: " + part)
					}
				}
				return nil
			},
		},
		{
			name: "包含错误",
			setup: func(r *Result) {
				r.AddSuccess()
				r.AddError(SQLTask{
					SQL:      "SELECT * FROM test",
					LineNum:  10,
					Filename: "test.sql",
				}, errors.New("test error"))
				r.Duration = 1 * time.Second
				r.Finish()
			},
			verify: func(output string) error {
				expectedParts := []string{
					"总语句数: 2",
					"成功: 1",
					"失败: 1",
					"错误详情",
					"test.sql",
					"test error",
				}
				for _, part := range expectedParts {
					if !strings.Contains(output, part) {
						return errors.New("missing expected output: " + part)
					}
				}
				return nil
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 创建新的结果对象
			result := NewResult()
			tt.setup(result)
			// 捕获标准输出
			old := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			// 创建一个channel来接收输出
			outputChan := make(chan string)
			go func() {
				var buf bytes.Buffer
				_, err := io.Copy(&buf, r)
				if err != nil {
					t.Errorf("io.Copy error: %v", err)
				}
				outputChan <- buf.String()
			}()

			result.Print()

			// 恢复标准输出
			w.Close()
			os.Stdout = old

			// 获取捕获的输出
			output := <-outputChan

			// 验证输出
			if err := tt.verify(output); err != nil {
				t.Errorf("Print() output verification failed: %v\nOutput: %s", err, output)
			}
		})
	}
}

// captureOutput 捕获标准输出的辅助函数
