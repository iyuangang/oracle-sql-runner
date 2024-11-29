package db

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	_ "github.com/godror/godror"
	"github.com/iyuangang/oracle-sql-runner/internal/config"
	"github.com/iyuangang/oracle-sql-runner/internal/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestEnv(t *testing.T) (*config.DatabaseConfig, *utils.Logger) {
	// 加载测试配置
	cfg, err := config.Load("../../config.json")
	require.NoError(t, err, "加载配置失败")

	dbConfig, ok := cfg.Databases["test"]
	require.True(t, ok, "未找到测试数据库配置")

	// 创建临时日志文件
	tmpDir, err := os.MkdirTemp("", "sql-runner-test")
	require.NoError(t, err, "创建临时目录失败")

	logFile := filepath.Join(tmpDir, "test.log")
	logger, err := utils.NewLogger(logFile, "debug", true)
	require.NoError(t, err, "创建日志记录器失败")

	t.Cleanup(func() {
		logger.Close()
		os.RemoveAll(tmpDir)
	})

	return &dbConfig, logger
}

func TestNewPool(t *testing.T) {
	cfg, logger := setupTestEnv(t)

	tests := []struct {
		name    string
		cfg     *config.DatabaseConfig
		wantErr bool
	}{
		{
			name:    "Valid config",
			cfg:     cfg,
			wantErr: false,
		},
		{
			name: "Invalid host",
			cfg: &config.DatabaseConfig{
				User:     cfg.User,
				Password: cfg.Password,
				Host:     "nonexistent",
				Port:     cfg.Port,
				Service:  cfg.Service,
			},
			wantErr: true,
		},
		{
			name: "Invalid credentials",
			cfg: &config.DatabaseConfig{
				User:     "invalid",
				Password: "invalid",
				Host:     cfg.Host,
				Port:     cfg.Port,
				Service:  cfg.Service,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool, err := NewPool(tt.cfg, logger)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, pool)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, pool)
				if pool != nil {
					defer pool.Close()
					// 验证连接池配置
					stats := pool.Stats()
					assert.Equal(t, tt.cfg.MaxConnections, stats.MaxOpenConnections)
				}
			}
		})
	}
}

func TestPoolExecContext(t *testing.T) {
	cfg, logger := setupTestEnv(t)
	pool, err := NewPool(cfg, logger)
	require.NoError(t, err)
	defer pool.Close()

	ctx := context.Background()

	tests := []struct {
		name       string
		setupSQL   string
		sql        string
		args       []interface{}
		wantErr    bool
		cleanupSQL string
	}{
		{
			name:       "Create table",
			sql:        "CREATE TABLE test_exec (id NUMBER)",
			wantErr:    false,
			cleanupSQL: "DROP TABLE test_exec",
		},
		{
			name:    "Invalid SQL",
			sql:     "CREATE INVALID",
			wantErr: true,
		},
		{
			name:     "Insert with parameters",
			setupSQL: "CREATE TABLE test_exec_params (id NUMBER)",
			sql:      "INSERT INTO test_exec_params VALUES (:1)",
			args:     []interface{}{1},
			wantErr:  false,
		},
		{
			name:       "Update with parameters",
			sql:        "UPDATE test_exec_params SET id = :1 WHERE id = :2",
			args:       []interface{}{2, 1},
			wantErr:    false,
			cleanupSQL: "DROP TABLE test_exec_params",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			if tt.setupSQL != "" {
				_, err := pool.ExecContext(ctx, tt.setupSQL)
				require.NoError(t, err)
			}

			// Test
			_, err := pool.ExecContext(ctx, tt.sql, tt.args...)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			// Cleanup
			if tt.cleanupSQL != "" {
				_, err := pool.ExecContext(ctx, tt.cleanupSQL)
				require.NoError(t, err)
			}
		})
	}
}

func TestPoolQueryContext(t *testing.T) {
	cfg, logger := setupTestEnv(t)
	pool, err := NewPool(cfg, logger)
	require.NoError(t, err)
	defer pool.Close()

	ctx := context.Background()

	tests := []struct {
		name       string
		setupSQL   string
		sql        string
		args       []interface{}
		wantErr    bool
		wantRows   bool
		cleanupSQL string
	}{
		{
			name:     "Simple query",
			sql:      "SELECT 1 FROM DUAL",
			wantErr:  false,
			wantRows: true,
		},
		{
			name:    "Invalid query",
			sql:     "SELECT * FROM nonexistent_table",
			wantErr: true,
		},
		{
			name:     "Query with parameters",
			setupSQL: "CREATE TABLE test_query (id NUMBER)",
			sql:      "SELECT * FROM test_query WHERE id = :1",
			args:     []interface{}{1},
			wantErr:  false,
			wantRows: false,
		},
		{
			name:       "Query with results",
			setupSQL:   "INSERT INTO test_query VALUES (1)",
			sql:        "SELECT * FROM test_query",
			wantErr:    false,
			wantRows:   true,
			cleanupSQL: "DROP TABLE test_query",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			if tt.setupSQL != "" {
				_, err := pool.ExecContext(ctx, tt.setupSQL)
				require.NoError(t, err)
			}

			// Test
			rows, err := pool.QueryContext(ctx, tt.sql, tt.args...)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, rows)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, rows)
				if rows != nil {
					defer rows.Close()
					hasRows := rows.Next()
					assert.Equal(t, tt.wantRows, hasRows)
				}
			}

			// Cleanup
			if tt.cleanupSQL != "" {
				_, err := pool.ExecContext(ctx, tt.cleanupSQL)
				require.NoError(t, err)
			}
		})
	}
}

func TestPoolTransaction(t *testing.T) {
	cfg, logger := setupTestEnv(t)
	pool, err := NewPool(cfg, logger)
	require.NoError(t, err)
	defer pool.Close()

	ctx := context.Background()

	t.Run("Transaction commit", func(t *testing.T) {
		// Setup
		_, err := pool.ExecContext(ctx, "CREATE TABLE test_tx (id NUMBER)")
		require.NoError(t, err)
		defer pool.ExecContext(ctx, "DROP TABLE test_tx")

		// Begin transaction
		tx, err := pool.Begin()
		require.NoError(t, err)

		// Execute in transaction
		_, err = tx.Exec("INSERT INTO test_tx VALUES (1)")
		assert.NoError(t, err)

		// Commit
		err = tx.Commit()
		assert.NoError(t, err)

		// Verify
		var count int
		row := pool.db.QueryRow("SELECT COUNT(*) FROM test_tx")
		err = row.Scan(&count)
		assert.NoError(t, err)
		assert.Equal(t, 1, count)
	})

	t.Run("Transaction rollback", func(t *testing.T) {
		// Setup
		_, err := pool.ExecContext(ctx, "CREATE TABLE test_tx_rollback (id NUMBER)")
		require.NoError(t, err)
		defer pool.ExecContext(ctx, "DROP TABLE test_tx_rollback")

		// Begin transaction
		tx, err := pool.Begin()
		require.NoError(t, err)

		// Execute in transaction
		_, err = tx.Exec("INSERT INTO test_tx_rollback VALUES (1)")
		assert.NoError(t, err)

		// Rollback
		err = tx.Rollback()
		assert.NoError(t, err)

		// Verify
		var count int
		row := pool.db.QueryRow("SELECT COUNT(*) FROM test_tx_rollback")
		err = row.Scan(&count)
		assert.NoError(t, err)
		assert.Equal(t, 0, count)
	})
}

func TestPoolStats(t *testing.T) {
	cfg, logger := setupTestEnv(t)
	pool, err := NewPool(cfg, logger)
	require.NoError(t, err)
	defer pool.Close()

	// 获取初始统计信息
	stats := pool.Stats()
	assert.Equal(t, int(stats.MaxOpenConnections), cfg.MaxConnections)
	assert.GreaterOrEqual(t, stats.OpenConnections, int(0))

	// 执行一些查询以创建连接
	ctx := context.Background()
	for i := 0; i < 5; i++ {
		rows, err := pool.QueryContext(ctx, "SELECT 1 FROM DUAL")
		require.NoError(t, err)
		rows.Close()
	}

	// 验证连接数增加
	statsAfter := pool.Stats()
	assert.GreaterOrEqual(t, statsAfter.OpenConnections, int(0))
}

func TestPoolClose(t *testing.T) {
	cfg, logger := setupTestEnv(t)
	pool, err := NewPool(cfg, logger)
	require.NoError(t, err)

	// 关闭连接池
	err = pool.Close()
	assert.NoError(t, err)

	// 验证连接已关闭
	ctx := context.Background()
	_, err = pool.QueryContext(ctx, "SELECT 1 FROM DUAL")
	assert.Error(t, err)
}
