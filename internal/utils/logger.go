package utils

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// LogLevel 日志级别
type LogLevel int

const (
	LogLevelDebug LogLevel = iota
	LogLevelInfo
	LogLevelWarn
	LogLevelError
)

// 日志级别字符串映射
var levelStrings = map[LogLevel]string{
	LogLevelDebug: "DEBUG",
	LogLevelInfo:  "INFO",
	LogLevelWarn:  "WARN",
	LogLevelError: "ERROR",
}

// LogEntry 表示一条日志记录
type LogEntry struct {
	Time   time.Time      `json:"time"`
	Level  string         `json:"level"`
	Source *LogSource     `json:"source"`
	Msg    string         `json:"msg"`
	Args   map[string]any `json:"args,omitempty"`
}

// LogSource 表示日志来源信息
type LogSource struct {
	Function string `json:"function"`
	File     string `json:"file"`
	Line     int    `json:"line"`
}

// Logger 日志记录器
type Logger struct {
	writer  io.Writer
	level   LogLevel
	verbose bool
}

// NewLogger 创建新的日志记录器
func NewLogger(logFile string, level string, verbose bool) (*Logger, error) {
	// 创建日志目录
	if err := os.MkdirAll(filepath.Dir(logFile), 0o755); err != nil {
		return nil, fmt.Errorf("创建日志目录失败: %w", err)
	}

	// 打开日志文件
	file, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o666)
	if err != nil {
		return nil, fmt.Errorf("创建日志文件失败: %w", err)
	}

	// 设置输出writer
	var writer io.Writer
	if verbose {
		writer = io.MultiWriter(file, os.Stdout)
	} else {
		writer = file
	}

	// 解析日志级别
	var logLevel LogLevel
	switch strings.ToLower(level) {
	case "debug":
		logLevel = LogLevelDebug
	case "info":
		logLevel = LogLevelInfo
	case "warn":
		logLevel = LogLevelWarn
	case "error":
		logLevel = LogLevelError
	default:
		logLevel = LogLevelInfo
	}

	return &Logger{
		writer:  writer,
		level:   logLevel,
		verbose: verbose,
	}, nil
}

// getCallerInfo 获取调用者信息
func getCallerInfo(skip int) *LogSource {
	pc, file, line, ok := runtime.Caller(skip)
	if !ok {
		return nil
	}

	// 获取函数名
	fn := runtime.FuncForPC(pc)
	funcName := fn.Name()

	// 简化函数名（去除完整路径）
	if idx := strings.LastIndex(funcName, "/"); idx != -1 {
		funcName = funcName[idx+1:]
	}

	// 简化文件路径（只保留internal及之后的路径）
	if idx := strings.Index(file, "internal"); idx != -1 {
		file = file[idx:]
	}

	return &LogSource{
		Function: funcName,
		File:     file,
		Line:     line,
	}
}

// log 通用日志记录函数
func (l *Logger) log(level LogLevel, msg string, args ...any) {
	if level < l.level {
		return
	}

	// 构造日志条目
	entry := &LogEntry{
		Time:   time.Now(),
		Level:  levelStrings[level],
		Source: getCallerInfo(3), // 跳过log、Debug/Info等函数和实际调用处
		Msg:    msg,
		Args:   make(map[string]any),
	}

	// 处理参数
	for i := 0; i < len(args); i += 2 {
		if i+1 < len(args) {
			key, ok := args[i].(string)
			if ok {
				entry.Args[key] = args[i+1]
			}
		}
	}

	// 如果Args为空，则在JSON中省略它
	if len(entry.Args) == 0 {
		entry.Args = nil
	}

	// 序列化并写入
	data, _ := json.Marshal(entry)
	if _, err := l.writer.Write(append(data, '\n')); err != nil {
		fmt.Printf("写入日志失败: %v\n", err)
	}
}

func (l *Logger) Debug(msg string, args ...any) {
	l.log(LogLevelDebug, msg, args...)
}

func (l *Logger) Info(msg string, args ...any) {
	l.log(LogLevelInfo, msg, args...)
}

func (l *Logger) Warn(msg string, args ...any) {
	l.log(LogLevelWarn, msg, args...)
}

func (l *Logger) Error(msg string, args ...any) {
	l.log(LogLevelError, msg, args...)
}

func (l *Logger) Fatal(msg string, args ...any) {
	l.log(LogLevelError, msg, args...)
	os.Exit(1)
}

// Close 关闭日志文件
func (l *Logger) Close() error {
	if closer, ok := l.writer.(io.Closer); ok {
		return closer.Close()
	}
	return nil
}
