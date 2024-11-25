package utils

import (
	"fmt"
	"log"
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

// Logger 日志记录器
type Logger struct {
	level   LogLevel
	logger  *log.Logger
	file    *os.File
	verbose bool
}

// NewLogger 创建新的日志记录器
func NewLogger(logFile string, level string, verbose bool) (*Logger, error) {
	// 创建日志文件
	file, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o666)
	if err != nil {
		return nil, fmt.Errorf("创建日志文件失败: %w", err)
	}

	// 设置日志级别
	logLevel := LogLevelInfo
	switch strings.ToLower(level) {
	case "debug":
		logLevel = LogLevelDebug
	case "info":
		logLevel = LogLevelInfo
	case "warn":
		logLevel = LogLevelWarn
	case "error":
		logLevel = LogLevelError
	}

	return &Logger{
		level:   logLevel,
		logger:  log.New(file, "", log.LstdFlags),
		file:    file,
		verbose: verbose,
	}, nil
}

// log 记录日志
func (l *Logger) log(level LogLevel, msg string, args ...interface{}) {
	if level < l.level {
		return
	}

	// 获取调用信息
	_, file, line, ok := runtime.Caller(2)
	caller := "???"
	if ok {
		caller = fmt.Sprintf("%s:%d", filepath.Base(file), line)
	}

	// 格式化参数
	var pairs []string
	for i := 0; i < len(args); i += 2 {
		if i+1 < len(args) {
			pairs = append(pairs, fmt.Sprintf("%v=%v", args[i], args[i+1]))
		}
	}

	// 构建日志消息
	logMsg := fmt.Sprintf("[%s] %s %s %s",
		time.Now().Format("2006-01-02 15:04:05"),
		levelString(level),
		msg,
		strings.Join(pairs, " "))

	// 写入日志文件
	l.logger.Printf("%s %s", caller, logMsg)

	// 如果开启详细模式，同时输出到控制台
	if l.verbose {
		fmt.Println(logMsg)
	}
}

// Debug 记录调试日志
func (l *Logger) Debug(msg string, args ...interface{}) {
	l.log(LogLevelDebug, msg, args...)
}

// Info 记录信息日志
func (l *Logger) Info(msg string, args ...interface{}) {
	l.log(LogLevelInfo, msg, args...)
}

// Warn 记录警告日志
func (l *Logger) Warn(msg string, args ...interface{}) {
	l.log(LogLevelWarn, msg, args...)
}

// Error 记录错误日志
func (l *Logger) Error(msg string, args ...interface{}) {
	l.log(LogLevelError, msg, args...)
}

// Fatal 记录致命错误并退出
func (l *Logger) Fatal(msg string, args ...interface{}) {
	l.log(LogLevelError, msg, args...)
	os.Exit(1)
}

// Close 关闭日志文件
func (l *Logger) Close() error {
	return l.file.Close()
}

// levelString 获取日志级别字符串
func levelString(level LogLevel) string {
	switch level {
	case LogLevelDebug:
		return "DEBUG"
	case LogLevelInfo:
		return "INFO"
	case LogLevelWarn:
		return "WARN"
	case LogLevelError:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}
