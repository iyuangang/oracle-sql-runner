package integration

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/iyuangang/oracle-sql-runner/internal/config"
	"github.com/iyuangang/oracle-sql-runner/internal/core"
	"github.com/iyuangang/oracle-sql-runner/internal/utils"
)

func TestExecutor(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过集成测试")
	}

	// 加载测试配置
	cfg, err := config.Load("../../config.json")
	if err != nil {
		t.Fatalf("加载配置失败: %v", err)
	}

	// 创建临时日志文件
	tmpDir, err := os.MkdirTemp("", "sql-runner-test")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	logFile := filepath.Join(tmpDir, "test.log")
	logger, err := utils.NewLogger(logFile, "debug", true)
	if err != nil {
		t.Fatalf("创建日志记录器失败: %v", err)
	}
	defer logger.Close()

	// 创建执行器
	executor, err := core.NewExecutor(cfg, "test", logger)
	if err != nil {
		t.Fatalf("创建执行器失败: %v", err)
	}
	defer executor.Close()

	// 测试用例
	tests := []struct {
		name     string
		sqlFile  string
		wantErr  bool
		expected struct {
			success int
			failed  int
		}
	}{
		{
			name:    "基本查询测试",
			sqlFile: "../fixtures/basic_query.sql",
			wantErr: false,
			expected: struct {
				success int
				failed  int
			}{
				success: 1,
				failed:  0,
			},
		},
		{
			name:    "PL/SQL块测试",
			sqlFile: "../fixtures/plsql_block.sql",
			wantErr: false,
			expected: struct {
				success int
				failed  int
			}{
				success: 1,
				failed:  0,
			},
		},
		{
			name:    "多语句测试",
			sqlFile: "../fixtures/multi_statements.sql",
			wantErr: false,
			expected: struct {
				success int
				failed  int
			}{
				success: 35,
				failed:  0,
			},
		},
		{
			name:    "错误查询测试",
			sqlFile: "../fixtures/error_query.sql",
			wantErr: true,
			expected: struct {
				success int
				failed  int
			}{
				success: 0,
				failed:  1,
			},
		},
		{
			name:    "空文件测试",
			sqlFile: "../fixtures/empty_file.sql",
			wantErr: false,
			expected: struct {
				success int
				failed  int
			}{
				success: 0,
				failed:  0,
			},
		},
		{
			name:    "单语句无分号测试",
			sqlFile: "../fixtures/one_statement_without_semicolon.sql",
			wantErr: false,
			expected: struct {
				success int
				failed  int
			}{
				success: 1,
				failed:  0,
			},
		},
		// 添加更多测试用例...
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := executor.ExecuteFile(tt.sqlFile)

			if (result.Failed > 0) != tt.wantErr {
				t.Errorf("ExecuteFile() error = %v, wantErr %v", result.Failed > 0, tt.wantErr)
				return
			}

			if result.Success != tt.expected.success {
				t.Errorf("成功数量不匹配 got = %v, want %v", result.Success, tt.expected.success)
			}

			if result.Failed != tt.expected.failed {
				t.Errorf("失败数量不匹配 got = %v, want %v", result.Failed, tt.expected.failed)
			}
		})
	}
}
