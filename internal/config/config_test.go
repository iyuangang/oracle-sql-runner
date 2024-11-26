package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDatabaseConfig_GetConnectionString(t *testing.T) {
	tests := []struct {
		name string
		dc   DatabaseConfig
		want string
	}{
		{
			name: "基本连接字符串",
			dc: DatabaseConfig{
				User:     "test_user",
				Password: "test_pass",
				Host:     "localhost",
				Port:     1521,
				Service:  "ORCL",
			},
			want: `user="test_user" password="test_pass" connectString="localhost:1521/ORCL"`,
		},
		{
			name: "特殊字符处理",
			dc: DatabaseConfig{
				User:     "test@user",
				Password: `test"pass`,
				Host:     "db.example.com",
				Port:     1521,
				Service:  "PROD.WORLD",
			},
			want: `user="test@user" password="test""pass" connectString="db.example.com:1521/PROD.WORLD"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.dc.GetConnectionString(); got != tt.want {
				t.Errorf("GetConnectionString() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLoad(t *testing.T) {
	// 创建临时目录
	tmpDir, err := os.MkdirTemp("", "config-test")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	tests := []struct {
		name     string
		content  string
		wantErr  bool
		validate func(*testing.T, *Config)
	}{
		{
			name: "有效配置",
			content: `{
				"databases": {
					"test": {
						"name": "测试库",
						"user": "test_user",
						"password": "test_pass",
						"host": "localhost",
						"port": 1521,
						"service": "ORCL",
						"max_connections": 10,
						"idle_timeout": 300
					}
				},
				"max_retries": 3,
				"max_concurrent": 5,
				"batch_size": 1000,
				"timeout": 30,
				"log_level": "info",
				"log_file": "sql-runner.log"
			}`,
			wantErr: false,
			validate: func(t *testing.T, cfg *Config) {
				if len(cfg.Databases) != 1 {
					t.Error("应该有一个数据库配置")
				}
				if cfg.MaxRetries != 3 {
					t.Error("MaxRetries 应该是 3")
				}
			},
		},
		{
			name: "使用默认值",
			content: `{
				"databases": {
					"test": {
						"user": "test_user",
						"password": "test_pass",
						"host": "localhost",
						"port": 1521,
						"service": "ORCL"
					}
				}
			}`,
			wantErr: false,
			validate: func(t *testing.T, cfg *Config) {
				if cfg.MaxRetries != 3 {
					t.Error("MaxRetries 默认值应该是 3")
				}
				if cfg.MaxConcurrent != 5 {
					t.Error("MaxConcurrent 默认值应该是 5")
				}
				if cfg.BatchSize != 1000 {
					t.Error("BatchSize 默认值应该是 1000")
				}
				if cfg.Timeout != 30 {
					t.Error("Timeout 默认值应该是 30")
				}
			},
		},
		{
			name:     "无效JSON",
			content:  `{invalid json`,
			wantErr:  true,
			validate: nil,
		},
		{
			name:     "空配置",
			content:  `{}`,
			wantErr:  true,
			validate: nil,
		},
		{
			name: "缺少必需字段",
			content: `{
				"databases": {
					"test": {
						"name": "测试库"
					}
				}
			}`,
			wantErr:  true,
			validate: nil,
		},
		{
			name: "数据库配置验证",
			content: `{
				"databases": {
					"test": {
						"user": "",
						"password": "test_pass",
						"host": "localhost",
						"port": 1521,
						"service": "ORCL"
					}
				}
			}`,
			wantErr:  true,
			validate: nil,
		},
		{
			name: "完整配置",
			content: `{
				"databases": {
					"test": {
						"name": "测试库",
						"user": "test_user",
						"password": "test_pass",
						"host": "localhost",
						"port": 1521,
						"service": "ORCL",
						"max_connections": 10,
						"idle_timeout": 300
					}
				},
				"max_retries": 5,
				"max_concurrent": 10,
				"batch_size": 500,
				"timeout": 60,
				"log_level": "debug",
				"log_file": "test.log"
			}`,
			wantErr: false,
			validate: func(t *testing.T, cfg *Config) {
				db := cfg.Databases["test"]
				if db.MaxConnections != 10 {
					t.Error("MaxConnections 应该是 10")
				}
				if db.IdleTimeout != 300 {
					t.Error("IdleTimeout 应该是 300")
				}
				if cfg.MaxRetries != 5 {
					t.Error("MaxRetries 应该是 5")
				}
				if cfg.LogLevel != "debug" {
					t.Error("LogLevel 应该是 debug")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 创建配置文件
			configPath := filepath.Join(tmpDir, "config.json")
			err := os.WriteFile(configPath, []byte(tt.content), 0o644)
			if err != nil {
				t.Fatalf("写入配置文件失败: %v", err)
			}

			// 测试加载配置
			cfg, err := Load(configPath)
			if (err != nil) != tt.wantErr {
				t.Errorf("Load() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// 如果期望成功且有验证函数，执行验证
			if !tt.wantErr && tt.validate != nil {
				tt.validate(t, cfg)
			}
		})
	}

	// 测试文件不存在的情况
	t.Run("文件不存在", func(t *testing.T) {
		_, err := Load("non_existent_file.json")
		if err == nil {
			t.Error("应该返回错误")
		}
	})
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *Config
		wantErr bool
	}{
		{
			name: "缺少数据库配置",
			cfg: &Config{
				Databases: map[string]DatabaseConfig{},
			},
			wantErr: true,
		},
		{
			name: "缺少用户名",
			cfg: &Config{
				Databases: map[string]DatabaseConfig{
					"test": {
						Password: "pass",
						Host:     "localhost",
						Port:     1521,
						Service:  "ORCL",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "缺少密码",
			cfg: &Config{
				Databases: map[string]DatabaseConfig{
					"test": {
						User:    "user",
						Host:    "localhost",
						Port:    1521,
						Service: "ORCL",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "缺少主机",
			cfg: &Config{
				Databases: map[string]DatabaseConfig{
					"test": {
						User:     "user",
						Password: "pass",
						Port:     1521,
						Service:  "ORCL",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "缺少端口",
			cfg: &Config{
				Databases: map[string]DatabaseConfig{
					"test": {
						User:     "user",
						Password: "pass",
						Host:     "localhost",
						Service:  "ORCL",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "缺少服务名",
			cfg: &Config{
				Databases: map[string]DatabaseConfig{
					"test": {
						User:     "user",
						Password: "pass",
						Host:     "localhost",
						Port:     1521,
					},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := validate(tt.cfg); (err != nil) != tt.wantErr {
				t.Errorf("validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
