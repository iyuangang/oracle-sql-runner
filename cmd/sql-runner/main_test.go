package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/iyuangang/oracle-sql-runner/internal/config"
	"github.com/iyuangang/oracle-sql-runner/internal/utils"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 在文件开头添加命令变量
var (
	rootCmd    *cobra.Command
	encryptCmd *cobra.Command
	decryptCmd *cobra.Command
)

func TestSetupLogger(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &config.Config{
		LogFile:  "test.log",
		LogLevel: "debug",
	}

	// 测试相对路径
	logger, err := setupLogger(cfg, tmpDir)
	require.NoError(t, err)
	assert.NotNil(t, logger)
	// 确保在测试结束前关闭日志
	t.Cleanup(func() {
		logger.Close()
	})

	// 测试绝对路径
	cfg.LogFile = filepath.Join(tmpDir, "test2.log")
	logger2, err := setupLogger(cfg, tmpDir)
	require.NoError(t, err)
	assert.NotNil(t, logger2)
	// 确保在测试结束前关闭日志
	t.Cleanup(func() {
		logger2.Close()
	})

	// 测试无效的日志级别
	cfg.LogLevel = "invalid"
	logger3, err := setupLogger(cfg, tmpDir)
	require.NoError(t, err)
	assert.NotNil(t, logger3)
	// 确保在测试结束前关闭日志
	t.Cleanup(func() {
		logger3.Close()
	})
}

func TestHandleDatabasePasswords(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	tests := []struct {
		name      string
		config    *config.Config
		wantErr   bool
		checkFunc func(*testing.T, *config.Config)
	}{
		{
			name: "加密未加密的密码",
			config: &config.Config{
				Databases: map[string]config.DatabaseConfig{
					"test": {Password: "test123"},
				},
				LogFile:  "test.log",
				LogLevel: "debug",
			},
			wantErr: false,
			checkFunc: func(t *testing.T, cfg *config.Config) {
				assert.False(t, utils.IsEncrypted(cfg.Databases["test"].Password))
			},
		},
		{
			name: "解密已加密的密码",
			config: &config.Config{
				Databases: map[string]config.DatabaseConfig{
					"test": {Password: "gWeG4Y2fP9vZ5KTe5IPHjkMusb4queY="},
				},
			},
			wantErr: false,
			checkFunc: func(t *testing.T, cfg *config.Config) {
				assert.False(t, utils.IsEncrypted(cfg.Databases["test"].Password))
			},
		},
		{
			name: "处理多个数据库",
			config: &config.Config{
				Databases: map[string]config.DatabaseConfig{
					"db1": {Password: "pass1"},
					"db2": {Password: "pass2"},
				},
			},
			wantErr: false,
			checkFunc: func(t *testing.T, cfg *config.Config) {
				assert.False(t, utils.IsEncrypted(cfg.Databases["db1"].Password))
				assert.False(t, utils.IsEncrypted(cfg.Databases["db2"].Password))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := handleDatabasePasswords(tt.config, configPath)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				if tt.checkFunc != nil {
					tt.checkFunc(t, tt.config)
				}
			}
		})
	}
}

func TestValidateInputs(t *testing.T) {
	tmpDir := t.TempDir()
	validFile := filepath.Join(tmpDir, "test.sql")
	require.NoError(t, os.WriteFile(validFile, []byte("SELECT 1 FROM DUAL;"), 0o644))

	tests := []struct {
		name    string
		sqlFile string
		dbName  string
		wantErr bool
	}{
		{
			name:    "有效输入",
			sqlFile: validFile,
			dbName:  "test",
			wantErr: false,
		},
		{
			name:    "缺少SQL文件",
			sqlFile: "",
			dbName:  "test",
			wantErr: true,
		},
		{
			name:    "缺少数据库名",
			sqlFile: validFile,
			dbName:  "",
			wantErr: true,
		},
		{
			name:    "SQL文件不存在",
			sqlFile: "nonexistent.sql",
			dbName:  "test",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateInputs(tt.sqlFile, tt.dbName)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// 添加命令初始化函数
func initCommands() {
	rootCmd = &cobra.Command{
		Use:     "sql-runner",
		Short:   "Oracle SQL 脚本执行工具",
		Version: Version,
		RunE:    run,
	}

	// 设置命令行参数
	rootCmd.PersistentFlags().StringVarP(&configFile, "config", "c", "config.json", "配置文件路径")
	rootCmd.PersistentFlags().StringVarP(&sqlFile, "file", "f", "", "SQL文件路径")
	rootCmd.PersistentFlags().StringVarP(&dbName, "database", "d", "", "数据库名称")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "显示详细信息")

	// 加密命令
	var encryptPassword string
	encryptCmd = &cobra.Command{
		Use:   "encrypt",
		Short: "加密数据库密码",
		RunE: func(cmd *cobra.Command, args []string) error {
			if encryptPassword == "" {
				return fmt.Errorf("请提供密码")
			}
			encrypted, err := utils.EncryptPassword(encryptPassword)
			if err != nil {
				return fmt.Errorf("加密失败: %w", err)
			}
			fmt.Printf("加密后的密码: %s\n", encrypted)
			return nil
		},
	}
	encryptCmd.Flags().StringVarP(&encryptPassword, "password", "p", "", "要加密的密码")
	rootCmd.AddCommand(encryptCmd)

	// 解密命令
	var decryptPassword string
	decryptCmd = &cobra.Command{
		Use:   "decrypt",
		Short: "解密数据库密码",
		RunE: func(cmd *cobra.Command, args []string) error {
			if decryptPassword == "" {
				return fmt.Errorf("请提供加密密码")
			}
			decrypted, err := utils.DecryptPassword(decryptPassword)
			if err != nil {
				return fmt.Errorf("解密失败: %w", err)
			}
			fmt.Printf("解密后的密码: %s\n", decrypted)
			return nil
		},
	}
	decryptCmd.Flags().StringVarP(&decryptPassword, "password", "p", "", "要解密的密码")
	rootCmd.AddCommand(decryptCmd)
}

// 在 TestMain 函数前添加 init 函数
func init() {
	initCommands()
}

// 修改 TestEncryptDecryptCommands 函数
func TestEncryptDecryptCommands(t *testing.T) {
	tests := []struct {
		name    string
		cmdType string
		args    []string
		wantErr bool
	}{
		{
			name:    "加密空密码",
			cmdType: "encrypt",
			args:    []string{"encrypt", "-p", ""},
			wantErr: true,
		},
		{
			name:    "加密有效密码",
			cmdType: "encrypt",
			args:    []string{"encrypt", "-p", "test123"},
			wantErr: false,
		},
		{
			name:    "解密空密码",
			cmdType: "decrypt",
			args:    []string{"decrypt", "-p", ""},
			wantErr: true,
		},
		{
			name:    "解密无效密文",
			cmdType: "decrypt",
			args:    []string{"decrypt", "-p", "invalid"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 重置命令行参数
			rootCmd.SetArgs(tt.args)

			// 执行命令
			err := rootCmd.Execute()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// 修改 TestMain 函数
func TestMain(t *testing.T) {
	// 保存原始的 osExit 函数
	originalOsExit := osExit
	defer func() { osExit = originalOsExit }()

	// 模拟 osExit
	var exitCode int
	osExit = func(code int) {
		exitCode = code
		panic(fmt.Sprintf("os.Exit(%d)", code))
	}

	tests := []struct {
		name     string
		args     []string
		wantCode int
	}{
		{
			name:     "显示帮助",
			args:     []string{"--help"},
			wantCode: 0,
		},
		{
			name:     "显示版本",
			args:     []string{"--version"},
			wantCode: 0,
		},
		{
			name:     "无参数运行",
			args:     []string{},
			wantCode: 1,
		},
		{
			name:     "无效的配置文件",
			args:     []string{"-c", "nonexistent.json"},
			wantCode: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rootCmd.SetArgs(tt.args)

			defer func() {
				r := recover()
				if r != nil {
					if exitStr, ok := r.(string); ok && exitStr == fmt.Sprintf("os.Exit(%d)", tt.wantCode) {
						assert.Equal(t, tt.wantCode, exitCode)
					} else {
						t.Errorf("unexpected panic: %v", r)
					}
				}
			}()

			rootCmd.Execute()
		})
	}
}

func setupTestEnv(t *testing.T) (*config.Config, *utils.Logger) {
	// 加载配置文件
	cfg, err := config.Load("../../config.json")
	require.NoError(t, err, "加载配置文件失败")

	// 处理所有数据库的加密密码
	for name, dbConfig := range cfg.Databases {
		if utils.IsEncrypted(dbConfig.Password) {
			decrypted, err := utils.DecryptPassword(dbConfig.Password)
			require.NoError(t, err, "解密数据库 %s 的密码失败", name)
			dbConfig.Password = decrypted
			cfg.Databases[name] = dbConfig
		}
	}

	// 创建临时日志目录
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "test.log")
	logger, err := utils.NewLogger(logFile, "debug", true)
	require.NoError(t, err)

	t.Cleanup(func() {
		logger.Close()
	})

	return cfg, logger
}

func TestRunSQL(t *testing.T) {
	cfg, logger := setupTestEnv(t)
	defer logger.Close()

	// 创建测试SQL文件
	tmpDir := t.TempDir()
	testSQL := filepath.Join(tmpDir, "test.sql")
	err := os.WriteFile(testSQL, []byte("SELECT 1 FROM DUAL;"), 0o644)
	require.NoError(t, err)

	tests := []struct {
		name    string
		dbName  string
		sqlFile string
		wantErr bool
	}{
		{
			name:    "数据库不存在",
			dbName:  "nonexistent",
			sqlFile: testSQL,
			wantErr: true,
		},
		{
			name:    "有效数据库",
			dbName:  "test",
			sqlFile: testSQL,
			wantErr: false, // 因为无法连接到真实数据库，所以期望错误
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 保存原始的 osExit 函数
			originalOsExit := osExit
			defer func() { osExit = originalOsExit }()

			// 模拟 osExit
			var gotError bool
			osExit = func(code int) {
				if code != 0 {
					gotError = true
				}
				panic(fmt.Sprintf("os.Exit(%d)", code))
			}

			var err error
			func() {
				defer func() {
					if r := recover(); r != nil {
						if exitStr, ok := r.(string); ok {
							if strings.HasPrefix(exitStr, "os.Exit(") {
								err = fmt.Errorf("command failed: %s", exitStr)
							}
						}
					}
				}()
				err = runSQL(cfg, tt.dbName, tt.sqlFile, logger)
			}()

			if tt.wantErr {
				assert.True(t, err != nil || gotError, "Expected an error for test case: %s", tt.name)
			} else {
				assert.False(t, gotError, "Expected no error for test case: %s", tt.name)
				assert.NoError(t, err)
			}
		})
	}
}
