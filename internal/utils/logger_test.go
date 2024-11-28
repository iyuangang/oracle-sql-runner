package utils

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// testWriter 用于测试的writer
type testWriter struct {
	buffer bytes.Buffer
}

func (w *testWriter) Write(p []byte) (n int, err error) {
	return w.buffer.Write(p)
}

func (w *testWriter) String() string {
	return w.buffer.String()
}

func TestNewLogger(t *testing.T) {
	// 创建临时目录
	tmpDir, err := os.MkdirTemp("", "logger-test")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// 保存当前工作目录
	currentDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("获取当前目录失败: %v", err)
	}

	// 切换到临时目录
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("切换到临时目录失败: %v", err)
	}
	// 测试结束后恢复工作目录
	err = os.Chdir(currentDir)
	if err != nil {
		t.Fatalf("恢复工作目录失败: %v", err)
	}

	tests := []struct {
		name     string
		logFile  string
		level    string
		verbose  bool
		wantErr  bool
		validate func(*testing.T, string)
	}{
		{
			name:    "创建成功-标准配置",
			logFile: filepath.Join(tmpDir, "test.log"),
			level:   "info",
			verbose: true,
			validate: func(t *testing.T, path string) {
				if _, err := os.Stat(path); os.IsNotExist(err) {
					t.Error("日志文件未创建")
				}
			},
		},
		{
			name:    "创建成功-不输出到控制台",
			logFile: filepath.Join(tmpDir, "quiet.log"),
			level:   "debug",
			verbose: false,
			validate: func(t *testing.T, path string) {
				if _, err := os.Stat(path); os.IsNotExist(err) {
					t.Error("日志文件未创建")
				}
			},
		},
		{
			name:    "创建成功-默认日志级别",
			logFile: filepath.Join(tmpDir, "default.log"),
			level:   "invalid",
			verbose: true,
			validate: func(t *testing.T, path string) {
				if _, err := os.Stat(path); os.IsNotExist(err) {
					t.Error("日志文件未创建")
				}
			},
		},
		{
			name:    "创建成功-默认日志文件",
			logFile: "",
			level:   "info",
			verbose: true,
		},
		{
			name:    "创建失败-无效目录",
			logFile: filepath.Join("/invalid", "path", "test.log"),
			level:   "info",
			verbose: true,
			wantErr: true,
		},
		{
			name:    "创建失败-无权限目录",
			logFile: filepath.Join("/root", "test.log"),
			level:   "info",
			verbose: true,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger, err := NewLogger(tt.logFile, tt.level, tt.verbose)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewLogger() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err == nil {
				defer logger.Close()
				// 验证日志文件
				if tt.validate != nil {
					tt.validate(t, tt.logFile)
				}
			}
		})
	}
}

func TestLogger_LogLevels(t *testing.T) {
	writer := &testWriter{}
	logger := &Logger{
		writer:  writer,
		level:   LogLevelDebug,
		verbose: true,
	}

	tests := []struct {
		name     string
		logFunc  func(string, ...any)
		level    string
		msg      string
		args     []any
		wantArgs map[string]any
	}{
		{
			name:    "Debug日志",
			logFunc: logger.Debug,
			level:   "DEBUG",
			msg:     "debug message",
			args:    []any{"key1", "value1"},
			wantArgs: map[string]any{
				"key1": "value1",
			},
		},
		{
			name:    "Info日志",
			logFunc: logger.Info,
			level:   "INFO",
			msg:     "info message",
			args:    []any{"key2", "123"},
			wantArgs: map[string]any{
				"key2": "123",
			},
		},
		{
			name:    "Warn日志",
			logFunc: logger.Warn,
			level:   "WARN",
			msg:     "warn message",
			args:    []any{"key3", true},
			wantArgs: map[string]any{
				"key3": true,
			},
		},
		{
			name:    "Error日志",
			logFunc: logger.Error,
			level:   "ERROR",
			msg:     "error message",
			args:    []any{"error", "test error"},
			wantArgs: map[string]any{
				"error": "test error",
			},
		},
		{
			name:    "无参数日志",
			logFunc: logger.Info,
			level:   "INFO",
			msg:     "no args message",
		},
		{
			name:    "奇数参数日志",
			logFunc: logger.Info,
			level:   "INFO",
			msg:     "odd args message",
			args:    []any{"key1", "value1", "key2"},
			wantArgs: map[string]any{
				"key1": "value1",
			},
		},
		{
			name:    "非字符串键日志",
			logFunc: logger.Info,
			level:   "INFO",
			msg:     "invalid key message",
			args:    []any{123, "value1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			writer.buffer.Reset()
			tt.logFunc(tt.msg, tt.args...)

			// 解析日志条目
			var entry LogEntry
			if err := json.NewDecoder(strings.NewReader(writer.String())).Decode(&entry); err != nil {
				t.Fatalf("解析日志失败: %v", err)
			}

			// 验证日志内容
			if entry.Level != tt.level {
				t.Errorf("日志级别错误: got %v, want %v", entry.Level, tt.level)
			}
			if entry.Msg != tt.msg {
				t.Errorf("日志消息错误: got %v, want %v", entry.Msg, tt.msg)
			}
			if tt.wantArgs != nil {
				for k, v := range tt.wantArgs {
					if entry.Args[k] != v {
						t.Errorf("日志参数错误: key %v, got %v, want %v", k, entry.Args[k], v)
					}
				}
			}
			if entry.Source == nil {
				t.Error("日志来源信息为空")
			}
		})
	}
}

func TestLogger_LogLevelFiltering(t *testing.T) {
	writer := &testWriter{}
	logger := &Logger{
		writer:  writer,
		level:   LogLevelInfo,
		verbose: true,
	}

	// Debug级别的日志应该被过滤
	logger.Debug("debug message")
	if writer.String() != "" {
		t.Error("Debug日志未被过滤")
	}

	// Info级别的日志应该被记录
	writer.buffer.Reset()
	logger.Info("info message")
	if writer.String() == "" {
		t.Error("Info日志未被记录")
	}
}

func TestLogger_Fatal(t *testing.T) {
	writer := &testWriter{}
	logger := &Logger{
		writer:  writer,
		level:   LogLevelDebug,
		verbose: true,
	}

	// 创建一个子进程来测试Fatal
	if os.Getenv("TEST_FATAL") == "1" {
		logger.Fatal("fatal message")
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run=TestLogger_Fatal")
	cmd.Env = append(os.Environ(), "TEST_FATAL=1")
	err := cmd.Run()

	if e, ok := err.(*exec.ExitError); !ok || e.Success() {
		t.Error("Fatal未导致进程退出")
	}
}

func TestLogger_Close(t *testing.T) {
	tests := []struct {
		name    string
		writer  io.Writer
		wantErr bool
	}{
		{
			name:    "关闭文件",
			writer:  &os.File{},
			wantErr: true,
		},
		{
			name:    "关闭非closer",
			writer:  &bytes.Buffer{},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := &Logger{writer: tt.writer}
			err := logger.Close()
			if (err != nil) != tt.wantErr {
				t.Errorf("Close() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestGetCallerInfo(t *testing.T) {
	source := getCallerInfo(1)
	if source == nil {
		t.Fatal("获取调用者信息失败")
	}

	if !strings.Contains(source.File, "internal") {
		t.Errorf("文件路径未包含 'internal': %s", source.File)
	}

	if !strings.Contains(source.Function, "TestGetCallerInfo") {
		t.Errorf("函数名错误: got %s, want TestGetCallerInfo", source.Function)
	}

	// 测试无效的skip值
	source = getCallerInfo(999)
	if source != nil {
		t.Error("应该返回nil")
	}
}
