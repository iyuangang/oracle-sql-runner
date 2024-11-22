package executor

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	_ "github.com/godror/godror"
	"github.com/iyuangang/oracle-sql-runner/config"
	"github.com/iyuangang/oracle-sql-runner/logger"
)

type ConnectionPool struct {
	db       *sql.DB
	config   *config.DatabaseConfig
	mu       sync.Mutex
	active   int
	maxConns int
}

func NewConnectionPool(cfg *config.DatabaseConfig) (*ConnectionPool, error) {
	connStr := fmt.Sprintf(`user="%s" password="%s" connectString="%s:%d/%s"`,
		cfg.User,
		cfg.Password,
		cfg.Host,
		cfg.Port,
		cfg.Service,
	)

	logger.Debug("Database connection string", "connStr", connStr)

	db, err := sql.Open("godror", connStr)
	if err != nil {
		return nil, fmt.Errorf("打开数据库连接失败: %w", err)
	}

	if cfg.MaxConnections <= 0 {
		cfg.MaxConnections = 10
	}
	db.SetMaxOpenConns(cfg.MaxConnections)
	db.SetMaxIdleConns(cfg.MaxConnections / 2)
	db.SetConnMaxLifetime(cfg.PoolTimeout)

	pool := &ConnectionPool{
		db:       db,
		config:   cfg,
		maxConns: cfg.MaxConnections,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	if err := pool.db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("验证数据库连接失败: %w", err)
	}

	return pool, nil
}

func (p *ConnectionPool) Acquire(ctx context.Context) (*sql.Conn, error) {
	p.mu.Lock()
	if p.active >= p.maxConns {
		p.mu.Unlock()
		logger.Debug("等待可用连接", "active", p.active, "max", p.maxConns)

		// 等待可用连接
		timer := time.NewTimer(p.config.PoolTimeout)
		defer timer.Stop()

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-timer.C:
			return nil, ErrPoolTimeout
		}
	}
	p.active++
	p.mu.Unlock()

	conn, err := p.db.Conn(ctx)
	if err != nil {
		p.mu.Lock()
		p.active--
		p.mu.Unlock()
		return nil, err
	}

	return conn, nil
}

func (p *ConnectionPool) Release(conn *sql.Conn) {
	if conn != nil {
		conn.Close()
		p.mu.Lock()
		p.active--
		p.mu.Unlock()
	}
}

func (p *ConnectionPool) Ping() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return p.db.PingContext(ctx)
}

func (p *ConnectionPool) Close() error {
	return p.db.Close()
}

func (p *ConnectionPool) Stats() sql.DBStats {
	return p.db.Stats()
}
