package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestFiles(t *testing.T) (string, string, string) {
	// 创建临时目录
	tmpDir := t.TempDir()

	// 复制配置文件到临时目录
	configSrc := "../../config.json"
	configDst := filepath.Join(tmpDir, "config.json")
	configData, err := os.ReadFile(configSrc)
	require.NoError(t, err, "读取配置文件失败")
	err = os.WriteFile(configDst, configData, 0o644)
	require.NoError(t, err, "写入配置文件失败")

	// 创建 SQL 文件
	sqlPath := filepath.Join(tmpDir, "test.sql")
	err = os.WriteFile(sqlPath, []byte("SELECT 1 FROM DUAL;"), 0o644)
	require.NoError(t, err, "创建 SQL 文件失败")

	return tmpDir, configDst, sqlPath
}

func TestRootCmd(t *testing.T) {
	tmpDir, configPath, sqlPath := setupTestFiles(t)
	defer os.RemoveAll(tmpDir)

	// 创建日志目录
	logDir := filepath.Join(tmpDir, "logs")
	err := os.MkdirAll(logDir, 0o755)
	require.NoError(t, err, "创建日志目录失败")

	// 读取原始配置
	configData, err := os.ReadFile(configPath)
	require.NoError(t, err, "读取配置文件失败")
	originalConfig := string(configData)

	tests := []struct {
		name     string
		args     []string
		wantErr  bool
		errMsg   string
		setup    func() error
		validate func(*testing.T, *bytes.Buffer)
	}{
		{
			name: "缺少SQL文件参数",
			args: []string{
				"-c", configPath,
				"-d", "test",
			},
			wantErr: true,
			errMsg:  "请指定SQL文件路径 (-f)",
		},
		{
			name: "缺少数据库参数",
			args: []string{
				"-c", configPath,
				"-f", sqlPath,
			},
			wantErr: true,
			errMsg:  "请指定数据库名称 (-d)",
		},
		{
			name: "无效配置文件",
			args: []string{
				"-c", "nonexistent.json",
				"-f", sqlPath,
				"-d", "test",
			},
			wantErr: true,
			errMsg:  "加载配置失败",
		},
		{
			name: "完整参数",
			args: []string{
				"-c", configPath,
				"-f", sqlPath,
				"-d", "test",
			},
			wantErr: false,
		},
		{
			name: "日志初始化失败",
			args: []string{
				"-c", configPath,
				"-f", sqlPath,
				"-d", "test",
			},
			wantErr: true,
			errMsg:  "初始化日志失败",
			setup: func() error {
				// 使用不存在的深层目录作为日志路径
				nonExistentDir := filepath.Join(tmpDir, "non", "existent", "dir")
				logPath := filepath.Join(nonExistentDir, "test.log")

				// 更新配置
				newConfig := strings.Replace(originalConfig,
					`"log_file": "logs/sql-runner.log"`,
					fmt.Sprintf(`"log_file": "%s"`, strings.ReplaceAll(logPath, "\\", "/")),
					1)

				return os.WriteFile(configPath, []byte(newConfig), 0o644)
			},
		},
		{
			name: "成功执行并正确关闭",
			args: []string{
				"-c", configPath,
				"-f", sqlPath,
				"-d", "test",
			},
			wantErr: false,
			setup: func() error {
				// 使用临时目录中的日志路径
				logPath := filepath.Join(logDir, "sql-runner.log")

				// 更新配置
				newConfig := strings.Replace(originalConfig,
					`"log_file": "logs/sql-runner.log"`,
					fmt.Sprintf(`"log_file": "%s"`, strings.ReplaceAll(logPath, "\\", "/")),
					1)

				return os.WriteFile(configPath, []byte(newConfig), 0o644)
			},
			validate: func(t *testing.T, buf *bytes.Buffer) {
				// 等待日志写入完成
				time.Sleep(100 * time.Millisecond)

				// 验证日志文件
				logPath := filepath.Join(logDir, "sql-runner.log")
				content, err := os.ReadFile(logPath)
				require.NoError(t, err, "读取日志文件失败")

				logContent := string(content)
				assert.Contains(t, logContent, "启动SQL Runner", "日志中未找到启动信息")
				assert.Contains(t, logContent, "SQL文件执行完成", "日志中未找到执行完成信息")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 执行设置
			if tt.setup != nil {
				err := tt.setup()
				require.NoError(t, err, "设置测试环境失败")
			}

			// 创建新的命令实例
			cmd := &cobra.Command{
				Use:     "sql-runner",
				Short:   "Oracle SQL 脚本执行工具",
				Version: Version,
				RunE:    rootCmd.RunE,
			}

			// 设置标志
			cmd.PersistentFlags().StringVarP(&configFile, "config", "c", "config.json", "配置文件路径")
			cmd.PersistentFlags().StringVarP(&sqlFile, "file", "f", "", "SQL文件路径")
			cmd.PersistentFlags().StringVarP(&dbName, "database", "d", "", "数据库名称")
			cmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "显示详细信息")

			// 捕获输出
			buf := new(bytes.Buffer)
			cmd.SetOut(buf)
			cmd.SetErr(buf)

			// 设置命令行参数
			cmd.SetArgs(tt.args)

			// 执行命令
			err := cmd.Execute()

			// 验证错误
			if (err != nil) != tt.wantErr {
				t.Errorf("Execute() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err != nil && !strings.Contains(err.Error(), tt.errMsg) {
				t.Errorf("错误消息不匹配，got = %v, want contain %v", err, tt.errMsg)
			}

			// 执行自定义验证
			if tt.validate != nil {
				tt.validate(t, buf)
			}
		})
	}
}

func TestVersion(t *testing.T) {
	// 保存并修改版本信息
	oldVersion := Version
	oldBuildTime := BuildTime
	Version = "test-version"
	BuildTime = "test-time"
	defer func() {
		Version = oldVersion
		BuildTime = oldBuildTime
	}()

	// 创建新的命令实例
	cmd := &cobra.Command{
		Use:     "sql-runner",
		Short:   "Oracle SQL 脚本执行工具",
		Version: Version,
		RunE:    rootCmd.RunE,
	}

	// 捕获输出
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	// 设置命令行参数
	cmd.SetArgs([]string{"--version"})

	if err := cmd.Execute(); err != nil {
		t.Errorf("执行 version 命令失败: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "test-version") {
		t.Errorf("版本信息未包含在输出中: %s", output)
	}
}

func TestMainFunction(t *testing.T) {
	// 保存原始的 os.Exit 和 os.Args
	oldOsExit := osExit
	oldArgs := os.Args
	defer func() {
		osExit = oldOsExit
		os.Args = oldArgs
	}()
	// 创建测试用的临时目录和文件
	tmpDir, _, sqlPath := setupTestFiles(t)
	defer os.RemoveAll(tmpDir)

	tests := []struct {
		name     string
		args     []string
		wantExit int
	}{
		{
			name:     "无效标志",
			args:     []string{"sql-runner", "--invalid-flag"},
			wantExit: 1,
		},
		{
			name:     "缺少必需参数",
			args:     []string{"sql-runner"},
			wantExit: 1,
		},
		{
			name: "无效配置",
			args: []string{
				"sql-runner",
				"-c", "invalid.json",
				"-f", sqlPath,
				"-d", "test",
			},
			wantExit: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 重置命令
			rootCmd.ResetFlags()
			rootCmd.PersistentFlags().StringVarP(&configFile, "config", "c", "config.json", "配置文件路径")
			rootCmd.PersistentFlags().StringVarP(&sqlFile, "file", "f", "", "SQL文件路径")
			rootCmd.PersistentFlags().StringVarP(&dbName, "database", "d", "", "数据库名称")
			rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "显示详细信息")

			// 设置测试参数
			os.Args = tt.args

			// 捕获退出码
			var gotExit int
			osExit = func(code int) {
				gotExit = code
			}

			// 执行 main
			main()

			if gotExit != tt.wantExit {
				t.Errorf("期望退出码为 %d，got %d", tt.wantExit, gotExit)
			}
		})
	}
}

// 辅助函数：检查字符串是否包含子串
func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}
