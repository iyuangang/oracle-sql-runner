package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type DatabaseConfig struct {
	Name           string        `json:"name"`
	User           string        `json:"user"`
	Password       string        `json:"password"`
	Host           string        `json:"host"`
	Port           int           `json:"port"`
	Service        string        `json:"service"`
	AutoCommit     bool          `json:"auto_commit"`
	MaxRetries     int           `json:"max_retries"`
	Timeout        time.Duration `json:"timeout_seconds"`
	EnableDBMSOut  bool          `json:"enable_dbms_output"`
	MaxConnections int           `json:"max_connections"`
	PoolTimeout    time.Duration `json:"pool_timeout_seconds"`
}

type ExecutionConfig struct {
	ParallelDegree int           `json:"parallel_degree"`
	BatchSize      int           `json:"batch_size"`
	MaxFileSize    int64         `json:"max_file_size"`
	RetryInterval  time.Duration `json:"retry_interval_seconds"`
}

type Config struct {
	Databases map[string]DatabaseConfig `json:"databases"`
	Execution ExecutionConfig           `json:"execution"`
	LogLevel  string                    `json:"log_level"`
	LogFile   string                    `json:"log_file"`
	encrypted bool
	crypto    *Crypto
}

func LoadConfig(path string) (*Config, error) {
	// 读取配置文件
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %w", err)
	}

	// 初始化加密
	crypto, err := NewCrypto()
	if err != nil {
		return nil, fmt.Errorf("初始化加密失败: %w", err)
	}
	config.crypto = crypto

	// 设置默认值
	config.setDefaults()

	// 处理加密的密码
	if err := config.handlePasswords(); err != nil {
		return nil, err
	}

	return &config, nil
}

func (c *Config) setDefaults() {
	// 执行配置默认值
	if c.Execution.ParallelDegree <= 0 {
		c.Execution.ParallelDegree = 1
	}
	if c.Execution.BatchSize <= 0 {
		c.Execution.BatchSize = 1000
	}
	if c.Execution.RetryInterval <= 0 {
		c.Execution.RetryInterval = 5 * time.Second
	}

	// 数据库配置默认值
	for name, db := range c.Databases {
		if db.MaxRetries <= 0 {
			db.MaxRetries = 3
		}
		if db.Timeout <= 0 {
			db.Timeout = 30 * time.Second
		}
		if db.MaxConnections <= 0 {
			db.MaxConnections = 10
		}
		if db.PoolTimeout <= 0 {
			db.PoolTimeout = 30 * time.Second
		}
		c.Databases[name] = db
	}
}

func (c *Config) handlePasswords() error {
	for name, db := range c.Databases {
		// 如果密码已加密，尝试解密
		if c.encrypted {
			decrypted, err := c.crypto.Decrypt(db.Password)
			if err != nil {
				return fmt.Errorf("解密数据库密码失败 (%s): %w", name, err)
			}
			db.Password = decrypted
		} else {
			// 如果密码未加密，进行加密
			encrypted, err := c.crypto.Encrypt(db.Password)
			if err != nil {
				return fmt.Errorf("加密数据库密码失败 (%s): %w", name, err)
			}
			db.Password = encrypted
		}
		c.Databases[name] = db
	}
	return nil
}

func (c *Config) Save(path string) error {
	// 确保密码已加密
	if !c.encrypted {
		if err := c.handlePasswords(); err != nil {
			return err
		}
		c.encrypted = true
	}

	// 创建配置目录
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}

	// 序列化配置
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化配置失败: %w", err)
	}

	// 写入文件
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("保存配置文件失败: %w", err)
	}

	return nil
}

func (dc *DatabaseConfig) GetConnectionString() string {
	return fmt.Sprintf("%s/%s@%s:%d/%s",
		dc.User,
		dc.Password,
		dc.Host,
		dc.Port,
		dc.Service,
	)
}
