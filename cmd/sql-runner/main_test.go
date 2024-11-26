package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestMain(m *testing.M) {
	// 保存原始的 stdout
	oldStdout := os.Stdout
	defer func() { os.Stdout = oldStdout }()

	// 运行测试
	code := m.Run()
	os.Exit(code)
}

func setupTestFiles(t *testing.T) (string, string, string) {
	// 创建临时目录
	tmpDir, err := os.MkdirTemp("", "sql-runner-test")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}

	// 创建测试配置文件
	configPath := filepath.Join(tmpDir, "config.json")
	configContent := `{
		"databases": {
			"test": {
				"name": "test",
				"user": "test",
				"password": "test",
				"host": "localhost",
				"port": 1521,
				"service": "test"
			}
		},
		"max_retries": 3,
		"max_concurrent": 2,
		"batch_size": 1000,
		"timeout": 30,
		"log_level": "debug"
	}`
	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("创建配置文件失败: %v", err)
	}

	// 创建测试SQL文件
	sqlPath := filepath.Join(tmpDir, "test.sql")
	sqlContent := "SELECT 1 FROM dual;"
	if err := os.WriteFile(sqlPath, []byte(sqlContent), 0o644); err != nil {
		t.Fatalf("创建SQL文件失败: %v", err)
	}

	return tmpDir, configPath, sqlPath
}

func TestRootCmd(t *testing.T) {
	tmpDir, configPath, sqlPath := setupTestFiles(t)
	defer os.RemoveAll(tmpDir)

	tests := []struct {
		name     string
		args     []string
		wantErr  bool
		errMsg   string
		validate func(*testing.T, *bytes.Buffer)
	}{
		{
			name:    "缺少SQL文件参数",
			args:    []string{"-d", "test"},
			wantErr: true,
			errMsg:  "请指定SQL文件路径",
		},
		{
			name:    "缺少数据库参数",
			args:    []string{"-f", sqlPath},
			wantErr: true,
			errMsg:  "请指定数据库名称",
		},
		{
			name:    "无效配置文件",
			args:    []string{"-c", "invalid.json", "-f", sqlPath, "-d", "test"},
			wantErr: true,
			errMsg:  "加载配置失败",
		},
		{
			name: "完整参数",
			args: []string{
				"-c", configPath,
				"-f", sqlPath,
				"-d", "test",
				"-v",
			},
			wantErr: true, // 因为无法连接到实际数据库
			errMsg:  "创建执行器失败",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
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
