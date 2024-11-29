package core

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/iyuangang/oracle-sql-runner/internal/config"
	"github.com/iyuangang/oracle-sql-runner/internal/utils"
	"github.com/iyuangang/oracle-sql-runner/pkg/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestEnv(t *testing.T) (*config.Config, *utils.Logger) {
	// 加载配置文件
	cfg, err := config.Load("../../config.json")
	require.NoError(t, err, "加载配置文件失败")

	// 创建临时日志目录
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "test.log")
	logger, err := utils.NewLogger(logFile, "debug", true)
	require.NoError(t, err, "创建日志记录器失败")

	t.Cleanup(func() {
		logger.Close()
	})

	return cfg, logger
}

func createTestSQLFile(t *testing.T, content string) string {
	tmpDir := t.TempDir()
	filename := filepath.Join(tmpDir, "test.sql")
	err := os.WriteFile(filename, []byte(content), 0o644)
	require.NoError(t, err)
	return filename
}

func TestNewExecutor(t *testing.T) {
	cfg, logger := setupTestEnv(t)

	tests := []struct {
		name    string
		dbName  string
		wantErr bool
	}{
		{
			name:    "Valid database",
			dbName:  "test",
			wantErr: false,
		},
		{
			name:    "Invalid database",
			dbName:  "nonexistent",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			executor, err := NewExecutor(cfg, tt.dbName, logger)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, executor)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, executor)
				executor.Close()
			}
		})
	}
}

func TestExecuteFile(t *testing.T) {
	cfg, logger := setupTestEnv(t)
	executor, err := NewExecutor(cfg, "test", logger)
	require.NoError(t, err)
	defer executor.Close()

	tests := []struct {
		name     string
		sql      string
		wantErr  bool
		expected struct {
			success int
			failed  int
		}
	}{
		{
			name: "Simple query",
			sql: `
				SELECT 1 FROM DUAL;
				SELECT SYSDATE FROM DUAL;
			`,
			wantErr: false,
			expected: struct {
				success int
				failed  int
			}{success: 2, failed: 0},
		},
		{
			name: "PL/SQL block",
			sql: `
				BEGIN
					NULL;
				END;
				/
			`,
			wantErr: false,
			expected: struct {
				success int
				failed  int
			}{success: 1, failed: 0},
		},
		{
			name: "Invalid SQL",
			sql: `
				SELECT * FROM nonexistent_table;
			`,
			wantErr: true,
			expected: struct {
				success int
				failed  int
			}{success: 0, failed: 1},
		},
		{
			name: "Mixed queries",
			sql: `
				SELECT 1 FROM DUAL;
				BEGIN
					NULL;
				END;
				/
				SELECT SYSDATE FROM DUAL;
			`,
			wantErr: false,
			expected: struct {
				success int
				failed  int
			}{success: 3, failed: 0},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filename := createTestSQLFile(t, tt.sql)
			result := executor.ExecuteFile(filename)

			assert.Equal(t, tt.expected.success, result.Success, "成功数量不匹配")
			assert.Equal(t, tt.expected.failed, result.Failed, "失败数量不匹配")
			assert.True(t, result.Duration > 0, "执行时间应大于0")
		})
	}
}

func TestExecuteTask(t *testing.T) {
	cfg, logger := setupTestEnv(t)
	executor, err := NewExecutor(cfg, "test", logger)
	require.NoError(t, err)
	defer executor.Close()

	tests := []struct {
		name    string
		task    models.SQLTask
		wantErr bool
	}{
		{
			name: "Query task",
			task: models.SQLTask{
				SQL:     "SELECT 1 FROM DUAL",
				Type:    models.SQLTypeQuery,
				LineNum: 1,
			},
			wantErr: false,
		},
		{
			name: "PL/SQL task",
			task: models.SQLTask{
				SQL:     "BEGIN NULL; END;",
				Type:    models.SQLTypePLSQL,
				LineNum: 1,
			},
			wantErr: false,
		},
		{
			name: "DML task",
			task: models.SQLTask{
				SQL:     "CREATE GLOBAL TEMPORARY TABLE test_temp (id NUMBER) ON COMMIT PRESERVE ROWS",
				Type:    models.SQLTypeExec,
				LineNum: 1,
			},
			wantErr: false,
		},
		{
			name: "DML task cleanup",
			task: models.SQLTask{
				SQL:     "DROP TABLE test_temp",
				Type:    models.SQLTypeExec,
				LineNum: 1,
			},
			wantErr: false,
		},
		{
			name: "Invalid task",
			task: models.SQLTask{
				SQL:     "SELECT * FROM nonexistent_table",
				Type:    models.SQLTypeQuery,
				LineNum: 1,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			err := executor.executeTask(ctx, tt.task)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}

	// 测试超时
	t.Run("Timeout", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
		defer cancel()

		task := models.SQLTask{
			SQL:     "SELECT DBMS_LOCK.SLEEP(1) FROM DUAL",
			Type:    models.SQLTypeQuery,
			LineNum: 1,
		}

		err := executor.executeTask(ctx, task)
		assert.Error(t, err)
	})
}

func TestParallelExecution(t *testing.T) {
	cfg, logger := setupTestEnv(t)
	cfg.MaxConcurrent = 3 // 设置并发数
	executor, err := NewExecutor(cfg, "test", logger)
	require.NoError(t, err)
	defer executor.Close()

	// 创建多个测试任务
	tasks := []models.SQLTask{
		{SQL: "SELECT 1 FROM DUAL", Type: models.SQLTypeQuery, LineNum: 1},
		{SQL: "SELECT 2 FROM DUAL", Type: models.SQLTypeQuery, LineNum: 2},
		{SQL: "SELECT 3 FROM DUAL", Type: models.SQLTypeQuery, LineNum: 3},
		{SQL: "SELECT 4 FROM DUAL", Type: models.SQLTypeQuery, LineNum: 4},
		{SQL: "SELECT 5 FROM DUAL", Type: models.SQLTypeQuery, LineNum: 5},
	}

	result := executor.executeParallel(tasks)
	assert.Equal(t, len(tasks), result.Success)
	assert.Equal(t, 0, result.Failed)
}

func TestPrintQueryResults(t *testing.T) {
	cfg, logger := setupTestEnv(t)
	executor, err := NewExecutor(cfg, "test", logger)
	require.NoError(t, err)
	defer executor.Close()

	tests := []struct {
		name    string
		sql     string
		wantErr bool
	}{
		{
			name:    "Simple types",
			sql:     "SELECT 1 as num, 'test' as str, NULL as null_val FROM DUAL",
			wantErr: false,
		},
		{
			name:    "Date type",
			sql:     "SELECT SYSDATE as date_val FROM DUAL",
			wantErr: false,
		},
		{
			name:    "CLOB type",
			sql:     "SELECT TO_CLOB('test') as clob_val FROM DUAL",
			wantErr: false,
		},
		{
			name:    "Multiple rows",
			sql:     "SELECT LEVEL as num FROM DUAL CONNECT BY LEVEL <= 5",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rows, err := executor.pool.QueryContext(context.Background(), tt.sql)
			require.NoError(t, err)
			defer rows.Close()

			err = printQueryResults(rows)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
