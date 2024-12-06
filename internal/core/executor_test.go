package core

import (
	"context"
	"os"
	"path/filepath"
	"sync"
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
	require.NoError(t, err, "加载配置失败")

	// 处理所有数据库的加密密码
	for name, dbConfig := range cfg.Databases {
		if utils.IsEncrypted(dbConfig.Password) {
			decrypted, err := utils.DecryptPassword(dbConfig.Password)
			require.NoError(t, err, "解密数据库 %s 的密码失败", name)
			dbConfig.Password = decrypted
			dbConfig.MaxConnections = 10 // 增加连接池大小
			cfg.Databases[name] = dbConfig
		}
	}

	// 创建临时日志文件
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

func TestParallelExecutionConcurrencySafety(t *testing.T) {
	// 设置测试环境
	cfg, logger := setupTestEnv(t)
	executor, err := NewExecutor(cfg, "test", logger)
	require.NoError(t, err)
	defer executor.Close()

	// 创建测试任务
	numTasks := 100
	tasks := make([]models.SQLTask, numTasks)
	for i := 0; i < numTasks; i++ {
		tasks[i] = models.SQLTask{
			SQL:     "SELECT 1 FROM DUAL",
			LineNum: i + 1,
		}
	}

	// 设置较短的超时时间，避免长时间等待
	executor.config.Timeout = 5

	// 执行并发测试
	var wg sync.WaitGroup
	results := make([]*models.Result, 5) // 执行多次测试
	for i := range results {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			results[idx] = executor.executeParallel(tasks)
		}(i)
	}
	wg.Wait()

	// 验证结果
	for i, result := range results {
		assert.Equal(t, numTasks, result.Success+result.Failed,
			"Total tasks should match for test %d", i)
		assert.Equal(t, numTasks, result.Success,
			"All tasks should succeed for test %d", i)
		assert.Equal(t, 0, result.Failed,
			"No tasks should fail for test %d", i)
	}
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
