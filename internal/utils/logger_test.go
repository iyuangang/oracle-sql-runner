package utils

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewLogger(t *testing.T) {
	// 创建临时目录
	tmpDir := t.TempDir()

	tests := []struct {
		name       string
		logFile    string
		level      string
		jsonFormat bool
		wantErr    bool
		setup      func(t *testing.T) error
		cleanup    func() error
	}{
		{
			name:       "成功创建JSON日志",
			logFile:    filepath.Join(tmpDir, "logs", "test.log"),
			level:      "debug",
			jsonFormat: true,
			wantErr:    false,
		},
		{
			name:       "成功创建文本日志",
			logFile:    filepath.Join(tmpDir, "logs", "test.log"),
			level:      "info",
			jsonFormat: false,
			wantErr:    false,
		},
		{
			name:       "无效的日志目录",
			logFile:    filepath.Join(os.DevNull, "test.log"),
			level:      "info",
			jsonFormat: true,
			wantErr:    true,
		},
		{
			name:       "无效的日志文件权限",
			logFile:    filepath.Join(tmpDir, "readonly", "test.log"),
			level:      "info",
			jsonFormat: true,
			wantErr:    true,
			setup: func(t *testing.T) error {
				dir := filepath.Join(tmpDir, "readonly")
				if err := os.MkdirAll(dir, 0o755); err != nil {
					return err
				}

				if runtime.GOOS != "windows" {
					// Unix 系统：设置目录为只读
					if err := os.Chmod(dir, 0o444); err != nil {
						return err
					}
				} else {
					// Windows 系统：使用系统保留设备文件名
					// 例如 CON, PRN, AUX, NUL 等
					t.Setenv("TEST_LOG_FILE", filepath.Join(dir, ":"))
				}
				return nil
			},
			cleanup: func() error {
				if runtime.GOOS != "windows" {
					dir := filepath.Join(tmpDir, "readonly")
					// 恢复目录权限以便清理
					return os.Chmod(dir, 0o755)
				}
				return nil
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 执行设置
			if tt.setup != nil {
				err := tt.setup(t)
				require.NoError(t, err)
			}

			// 执行清理
			if tt.cleanup != nil {
				defer func() {
					err := tt.cleanup()
					require.NoError(t, err)
				}()
			}

			// 获取日志文件路径
			logFile := tt.logFile
			if envFile := os.Getenv("TEST_LOG_FILE"); envFile != "" && tt.name == "无效的日志文件权限" {
				logFile = envFile
			}

			// 创建日志记录器
			logger, err := NewLogger(logFile, tt.level, tt.jsonFormat)
			if tt.wantErr {
				assert.Error(t, err)
				if logger != nil {
					logger.Close()
				}
				return
			}

			require.NoError(t, err)
			require.NotNil(t, logger)

			// 清理
			if logger != nil {
				err = logger.Close()
				assert.NoError(t, err)
			}
		})
	}
}

func TestLogger_LogLevels(t *testing.T) {
	// 创建临时目录和日志文件
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "test.log")

	// 创建日志记录器
	logger, err := NewLogger(logFile, "debug", true)
	require.NoError(t, err)
	defer logger.Close()

	// 测试所有日志级别
	tests := []struct {
		name     string
		logFunc  func(string, ...any)
		level    string
		message  string
		args     []any
		validate func(*testing.T, map[string]interface{})
	}{
		{
			name:    "Debug日志",
			logFunc: logger.Debug,
			level:   "DEBUG",
			message: "debug message",
			args:    []any{"key1", "value1", "key2", 42},
			validate: func(t *testing.T, entry map[string]interface{}) {
				assert.Equal(t, "DEBUG", entry["level"])
				assert.Equal(t, "debug message", entry["msg"])
				args := entry["args"].(map[string]interface{})
				assert.Equal(t, "value1", args["key1"])
				assert.Equal(t, float64(42), args["key2"])
			},
		},
		{
			name:    "Info日志",
			logFunc: logger.Info,
			level:   "INFO",
			message: "info message",
			args:    []any{"key1", "value1"},
			validate: func(t *testing.T, entry map[string]interface{}) {
				assert.Equal(t, "INFO", entry["level"])
				assert.Equal(t, "info message", entry["msg"])
				args := entry["args"].(map[string]interface{})
				assert.Equal(t, "value1", args["key1"])
			},
		},
		{
			name:    "Warn日志",
			logFunc: logger.Warn,
			level:   "WARN",
			message: "warn message",
			validate: func(t *testing.T, entry map[string]interface{}) {
				assert.Equal(t, "WARN", entry["level"])
				assert.Equal(t, "warn message", entry["msg"])
			},
		},
		{
			name:    "Error日志",
			logFunc: logger.Error,
			level:   "ERROR",
			message: "error message",
			validate: func(t *testing.T, entry map[string]interface{}) {
				assert.Equal(t, "ERROR", entry["level"])
				assert.Equal(t, "error message", entry["msg"])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 记录日志
			tt.logFunc(tt.message, tt.args...)

			// 等待日志写入
			time.Sleep(100 * time.Millisecond)

			// 读取并验证日志内容
			content, err := os.ReadFile(logFile)
			require.NoError(t, err)

			// 解析JSON日志条目
			lines := strings.Split(strings.TrimSpace(string(content)), "\n")
			lastLine := lines[len(lines)-1]

			var entry map[string]interface{}
			err = json.Unmarshal([]byte(lastLine), &entry)
			require.NoError(t, err)

			// 验证日志条目
			tt.validate(t, entry)
		})
	}
}

func TestLogger_Fatal(t *testing.T) {
	// 保存原始 osExit 函数
	originalOsExit := osExit
	defer func() { osExit = originalOsExit }()

	exitCalled := false
	exitCode := 0

	// 模拟 osExit
	osExit = func(code int) {
		exitCalled = true
		exitCode = code
	}

	// 创建临时目录和日志文件
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "test.log")

	// 创建日志记录器
	logger, err := NewLogger(logFile, "debug", true)
	require.NoError(t, err)
	defer logger.Close()

	// 调用 Fatal
	logger.Fatal("fatal error", "key", "value")

	// 验证 exit 被调用
	assert.True(t, exitCalled, "osExit 应该被调用")
	assert.Equal(t, 1, exitCode, "退出码应为 1")

	// 等待日志写入
	time.Sleep(100 * time.Millisecond)

	// 读取并验证日志内容
	content, err := os.ReadFile(logFile)
	require.NoError(t, err)

	var entry map[string]interface{}
	err = json.Unmarshal(content, &entry)
	require.NoError(t, err)

	assert.Equal(t, "ERROR", entry["level"])
	assert.Equal(t, "fatal error", entry["msg"])
	args := entry["args"].(map[string]interface{})
	assert.Equal(t, "value", args["key"])
}

func TestGetCallerInfo(t *testing.T) {
	source := getCallerInfo(1)
	require.NotNil(t, source)
	assert.Contains(t, source.Function, "TestGetCallerInfo")
	assert.Contains(t, source.File, "logger_test.go")
	assert.Greater(t, source.Line, 0)
}

func TestGetZapLevel(t *testing.T) {
	tests := []struct {
		name  string
		level string
		want  string
	}{
		{"Debug级别", "debug", "DEBUG"},
		{"Info级别", "info", "INFO"},
		{"Warn级别", "warn", "WARN"},
		{"Error级别", "error", "ERROR"},
		{"未知级别", "unknown", "INFO"},
		{"空级别", "", "INFO"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			level := getZapLevel(tt.level)
			assert.Equal(t, tt.want, level.CapitalString())
		})
	}
}

func TestLogger_Close(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "test.log")

	logger, err := NewLogger(logFile, "info", true)
	require.NoError(t, err)
	require.NotNil(t, logger)

	// 第一次关闭
	err = logger.Close()
	assert.NoError(t, err)

	// 第二次关闭，应该无错误
	err = logger.Close()
	assert.NoError(t, err)

	// 模拟关闭时出现错误
	// 替换 logger.file 为一个只读文件
	logger.file = &mockWriteCloser{readOnly: true}
	err = logger.Close()
	assert.Error(t, err)
}

// mockWriteCloser 模拟只能读的 WriteCloser
type mockWriteCloser struct {
	readOnly bool
}

func (m *mockWriteCloser) Write(p []byte) (n int, err error) {
	if m.readOnly {
		return 0, fmt.Errorf("read-only file")
	}
	return len(p), nil
}

func (m *mockWriteCloser) Close() error {
	if m.readOnly {
		return fmt.Errorf("read-only file cannot be closed")
	}
	return nil
}

func TestLogger_ConcurrentLogging(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "test.log")

	logger, err := NewLogger(logFile, "debug", true)
	require.NoError(t, err)
	defer logger.Close()

	var wg sync.WaitGroup
	numGoroutines := 100
	numMessages := 100

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numMessages; j++ {
				logger.Info(fmt.Sprintf("Goroutine %d message %d", id, j))
			}
		}(i)
	}

	wg.Wait()

	// 等待日志写入完成
	time.Sleep(200 * time.Millisecond)

	// 读取日志文件
	content, err := os.ReadFile(logFile)
	require.NoError(t, err)

	lines := strings.Split(strings.TrimSpace(string(content)), "\n")
	expectedLines := numGoroutines * numMessages
	assert.Equal(t, expectedLines, len(lines), "日志行数不符")
}

func TestLogger_EncoderFormats(t *testing.T) {
	tmpDir := t.TempDir()
	logFileJSON := filepath.Join(tmpDir, "json.log")
	logFileConsole := filepath.Join(tmpDir, "console.log")

	// 测试 JSON 编码器
	loggerJSON, err := NewLogger(logFileJSON, "info", true)
	require.NoError(t, err)
	defer loggerJSON.Close()

	loggerJSON.Info("JSON format log", "key", "value")

	contentJSON, err := os.ReadFile(logFileJSON)
	require.NoError(t, err)

	var entryJSON map[string]interface{}
	err = json.Unmarshal(contentJSON, &entryJSON)
	require.NoError(t, err)
	assert.Equal(t, "INFO", entryJSON["level"])
	assert.Equal(t, "JSON format log", entryJSON["msg"])
	assert.Equal(t, "value", entryJSON["args"].(map[string]interface{})["key"])

	// 测试 Console 编码器
	loggerConsole, err := NewLogger(logFileConsole, "info", false)
	require.NoError(t, err)
	defer loggerConsole.Close()

	loggerConsole.Info("Console format log", "key", "value")

	contentConsole, err := os.ReadFile(logFileConsole)
	require.NoError(t, err)

	// 简单检查日志内容包含预期字符串
	logStr := string(contentConsole)
	assert.Contains(t, logStr, "INFO")
	assert.Contains(t, logStr, "Console format log")
	assert.Contains(t, logStr, "\"key\":\"value\"")
}

func TestLogger_TimeAndLevel(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "test.log")

	logger, err := NewLogger(logFile, "info", true)
	require.NoError(t, err)
	defer logger.Close()

	currentTime := time.Now()
	logger.Info("Test time and level")

	// 等待日志写入
	time.Sleep(100 * time.Millisecond)

	// 读取日志内容
	content, err := os.ReadFile(logFile)
	require.NoError(t, err)

	var entry map[string]interface{}
	err = json.Unmarshal(content, &entry)
	require.NoError(t, err)

	// 验证时间戳
	logTimeStr, ok := entry["time"].(string)
	require.True(t, ok, "日志中缺少time字段")
	logTime, err := time.Parse(time.RFC3339, logTimeStr)
	require.NoError(t, err)
	assert.WithinDuration(t, currentTime, logTime, time.Second, "日志时间戳不在预期范围内")

	// 验证日志级别
	assert.Equal(t, "INFO", entry["level"])
}
