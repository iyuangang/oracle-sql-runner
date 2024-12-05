package main

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iyuangang/oracle-sql-runner/internal/config"
	"github.com/iyuangang/oracle-sql-runner/internal/utils"
)

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
		setup    func() error
		cleanup  func() error
	}{
		{
			name:     "无参数运行",
			args:     []string{"sql-runner"},
			wantCode: 1,
		},
		{
			name:     "帮助命令",
			args:     []string{"sql-runner", "--help"},
			wantCode: 0,
		},
		{
			name:     "版本命令",
			args:     []string{"sql-runner", "--version"},
			wantCode: 0,
		},
		{
			name:     "缺少数据库参数",
			args:     []string{"sql-runner", "-f", "test.sql"},
			wantCode: 1,
		},
		{
			name:     "缺少SQL文件参数",
			args:     []string{"sql-runner", "-d", "testdb"},
			wantCode: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 设置测试环境
			if tt.setup != nil {
				err := tt.setup()
				require.NoError(t, err)
			}

			// 清理测试环境
			if tt.cleanup != nil {
				defer func() {
					err := tt.cleanup()
					require.NoError(t, err)
				}()
			}

			// 保存原始参数
			oldArgs := os.Args
			defer func() { os.Args = oldArgs }()

			// 设置测试参数
			os.Args = tt.args

			// 执行测试
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

			main()
		})
	}
}

func setupTestEnv(t *testing.T) (*config.Config, *utils.Logger) {
	// 加载配置文件
	cfg, err := config.Load("../../config.json")
	require.NoError(t, err, "加载配置文件失败")

	// 解密所有数据库密码
	for name, dbConfig := range cfg.Databases {
		if utils.IsEncrypted(dbConfig.Password) {
			decrypted, err := utils.DecryptPassword(dbConfig.Password)
			require.NoError(t, err, "解密数据库密码失败")
			dbConfig.Password = decrypted
			cfg.Databases[name] = dbConfig
		}
	}

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

func TestRunE(t *testing.T) {
	cfg, logger := setupTestEnv(t)
	defer logger.Close()

	// 确保测试数据库的密码是加密的
	dbConfig := cfg.Databases["test"]
	if !utils.IsEncrypted(dbConfig.Password) {
		encrypted, err := utils.EncryptPassword(dbConfig.Password)
		require.NoError(t, err)
		dbConfig.Password = encrypted
		cfg.Databases["test"] = dbConfig
		err = config.Save("../../config.json", cfg)
		require.NoError(t, err)
	}

	// 创建临时目录
	tmpDir := t.TempDir()

	// 保存配置到临时文件
	configFile := filepath.Join(tmpDir, "test_config.json")
	err := config.Save(configFile, cfg)
	require.NoError(t, err, "保存配置文件失败")

	// 创建测试SQL文件
	sqlFile := filepath.Join(tmpDir, "test.sql")
	err = os.WriteFile(sqlFile, []byte(`
		SELECT 1 FROM DUAL;
		SELECT SYSDATE FROM DUAL;
		BEGIN
			NULL;
		END;
		/
	`), 0644)
	require.NoError(t, err)

	tests := []struct {
		name    string
		args    []string
		wantErr bool
		setup   func() error
	}{
		{
			name: "正常运行",
			args: []string{
				"--config", configFile,
				"--file", sqlFile,
				"--database", "test",
				"--verbose",
			},
			wantErr: false,
		},
		{
			name: "配置文件不存在",
			args: []string{
				"--config", "nonexistent.json",
				"--file", sqlFile,
				"--database", "test",
			},
			wantErr: true,
		},
		{
			name: "SQL文件不存在",
			args: []string{
				"--config", configFile,
				"--file", "nonexistent.sql",
				"--database", "test",
			},
			wantErr: true,
		},
		{
			name: "数据库不存在",
			args: []string{
				"--config", configFile,
				"--file", sqlFile,
				"--database", "nonexistent",
			},
			wantErr: true,
		},
		{
			name: "无效的SQL文件内容",
			args: []string{
				"--config", configFile,
				"--file", configFile, // 使用配置文件作为SQL文件
				"--database", "test",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setup != nil {
				err := tt.setup()
				require.NoError(t, err)
			}

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

func TestEncryptDecryptCommands(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{
			name:    "加密空密码",
			args:    []string{"encrypt", "--password", ""},
			wantErr: true,
		},
		{
			name:    "加密有效密码",
			args:    []string{"encrypt", "--password", "test123"},
			wantErr: false,
		},
		{
			name:    "解密空密码",
			args:    []string{"decrypt", "--password", ""},
			wantErr: true,
		},
		{
			name:    "解密无效密文",
			args:    []string{"decrypt", "--password", "invalid"},
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

func TestCommandFlags(t *testing.T) {
	// 验证根命令的持久性标志
	assert.NotNil(t, rootCmd.PersistentFlags().Lookup("config"))
	assert.NotNil(t, rootCmd.PersistentFlags().Lookup("file"))
	assert.NotNil(t, rootCmd.PersistentFlags().Lookup("database"))
	assert.NotNil(t, rootCmd.PersistentFlags().Lookup("verbose"))

	// 验证加密命令的标志
	assert.NotNil(t, encryptCmd.Flags().Lookup("password"))

	// 验证解密命令的标志
	assert.NotNil(t, decryptCmd.Flags().Lookup("password"))

	// 验证命令的使用说明
	assert.NotEmpty(t, rootCmd.Use)
	assert.NotEmpty(t, rootCmd.Short)
	assert.NotEmpty(t, encryptCmd.Use)
	assert.NotEmpty(t, encryptCmd.Short)
	assert.NotEmpty(t, decryptCmd.Use)
	assert.NotEmpty(t, decryptCmd.Short)

	// 验证子命令是否正确添加
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd == encryptCmd {
			found = true
			break
		}
	}
	assert.True(t, found, "encrypt command should be added to root command")

	found = false
	for _, cmd := range rootCmd.Commands() {
		if cmd == decryptCmd {
			found = true
			break
		}
	}
	assert.True(t, found, "decrypt command should be added to root command")
} 
