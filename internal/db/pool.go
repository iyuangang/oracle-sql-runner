package db

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	"github.com/iyuangang/oracle-sql-runner/internal/config"
	"github.com/iyuangang/oracle-sql-runner/internal/utils"
)

// Pool 数据库连接池
type Pool struct {
	db      *sql.DB
	config  *config.DatabaseConfig
	mu      sync.Mutex
	metrics *utils.Metrics
	logger  *utils.Logger
}

// NewPool 创建新的连接池
func NewPool(cfg *config.DatabaseConfig, logger *utils.Logger) (*Pool, error) {
	db, err := sql.Open("godror", cfg.GetConnectionString())
	if err != nil {
		return nil, fmt.Errorf("创建数据库连接失败: %w", err)
	}

	// 配置连接池
	db.SetMaxOpenConns(cfg.MaxConnections)
	db.SetMaxIdleConns(cfg.MaxConnections / 2)
	db.SetConnMaxIdleTime(cfg.IdleTimeout)

	// 测试连接
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("测试数据库连接失败: %w", err)
	}

	return &Pool{
		db:      db,
		config:  cfg,
		metrics: utils.NewMetrics(),
		logger:  logger,
	}, nil
}

// ExecContext 执行SQL语句
func (p *Pool) ExecContext(ctx context.Context, sql string, args ...interface{}) (sql.Result, error) {
	start := time.Now()
	result, err := p.db.ExecContext(ctx, sql, args...)
	duration := time.Since(start)

	if err != nil {
		p.metrics.AddQuery(duration, false)
		p.logger.Error("SQL执行失败",
			"sql", sql,
			"duration", duration,
			"error", err)
		return nil, err
	}

	p.metrics.AddQuery(duration, true)
	p.logger.Debug("SQL执行成功",
		"sql", sql,
		"duration", duration)
	return result, nil
}

// QueryContext 执行查询
func (p *Pool) QueryContext(ctx context.Context, sql string, args ...interface{}) (*sql.Rows, error) {
	start := time.Now()
	rows, err := p.db.QueryContext(ctx, sql, args...)
	duration := time.Since(start)

	if err != nil {
		p.metrics.AddQuery(duration, false)
		p.logger.Error("查询执行失败",
			"sql", sql,
			"duration", duration,
			"error", err)
		return nil, err
	}

	p.metrics.AddQuery(duration, true)
	p.logger.Debug("查询执行成功",
		"sql", sql,
		"duration", duration)
	return rows, nil
}

// Begin 开始事务
func (p *Pool) Begin() (*sql.Tx, error) {
	return p.db.Begin()
}

// Close 关闭连接池
func (p *Pool) Close() error {
	return p.db.Close()
}

// Stats 获取连接池统计信息
func (p *Pool) Stats() sql.DBStats {
	return p.db.Stats()
}
