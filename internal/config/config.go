package config

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// DatabaseConfig 数据库配置
type DatabaseConfig struct {
	Name           string        `json:"name"`
	User           string        `json:"user"`
	Password       string        `json:"password"`
	Host           string        `json:"host"`
	Port           int           `json:"port"`
	Service        string        `json:"service"`
	MaxConnections int           `json:"max_connections"`
	IdleTimeout    time.Duration `json:"idle_timeout"`
}

// Config 全局配置
type Config struct {
	Databases     map[string]DatabaseConfig `json:"databases"`
	MaxRetries    int                       `json:"max_retries"`
	MaxConcurrent int                       `json:"max_concurrent"`
	BatchSize     int                       `json:"batch_size"`
	Timeout       int                       `json:"timeout"`
	LogLevel      string                    `json:"log_level"`
	LogFile       string                    `json:"log_file"`
}

// GetConnectionString 获取数据库连接字符串
func (dc *DatabaseConfig) GetConnectionString() string {
	return fmt.Sprintf(`user="%s" password="%s" connectString="%s:%d/%s"`,
		dc.User,
		dc.Password,
		dc.Host,
		dc.Port,
		dc.Service,
	)
}

// Load 加载配置文件
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %w", err)
	}

	// 设置默认值
	if cfg.MaxRetries == 0 {
		cfg.MaxRetries = 3
	}
	if cfg.MaxConcurrent == 0 {
		cfg.MaxConcurrent = 5
	}
	if cfg.BatchSize == 0 {
		cfg.BatchSize = 1000
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 30
	}

	return &cfg, validate(&cfg)
}

// validate 验证配置
func validate(cfg *Config) error {
	if len(cfg.Databases) == 0 {
		return fmt.Errorf("至少需要配置一个数据库")
	}

	for name, db := range cfg.Databases {
		if db.User == "" {
			return fmt.Errorf("数据库 %s 未配置用户名", name)
		}
		if db.Password == "" {
			return fmt.Errorf("数据库 %s 未配置密码", name)
		}
		if db.Host == "" {
			return fmt.Errorf("数据库 %s 未配置主机地址", name)
		}
		if db.Port == 0 {
			return fmt.Errorf("数据库 %s 未配置端口", name)
		}
		if db.Service == "" {
			return fmt.Errorf("数据库 %s 未配置服务名", name)
		}
	}

	return nil
}
