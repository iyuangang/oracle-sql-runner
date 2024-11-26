package utils

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
)

// LogLevel 日志级别
type LogLevel = slog.Level

const (
	LogLevelDebug = slog.LevelDebug
	LogLevelInfo  = slog.LevelInfo
	LogLevelWarn  = slog.LevelWarn
	LogLevelError = slog.LevelError
)

// Logger 日志记录器
type Logger struct {
	logger  *slog.Logger
	file    *os.File
	verbose bool
}

// NewLogger 创建新的日志记录器
func NewLogger(logFile string, level string, verbose bool) (*Logger, error) {
	// 创建日志目录
	if err := os.MkdirAll(filepath.Dir(logFile), 0o755); err != nil {
		return nil, fmt.Errorf("创建日志目录失败: %w", err)
	}

	// 创建日志文件
	file, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o666)
	if err != nil {
		return nil, fmt.Errorf("创建日志文件失败: %w", err)
	}

	// 设置日志级别
	var logLevel LogLevel
	switch level {
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

	// 创建日志处理器
	opts := &slog.HandlerOptions{
		Level:     logLevel,
		AddSource: true,
	}

	var handler slog.Handler
	if verbose {
		// 同时输出到文件和控制台
		handler = slog.NewJSONHandler(io.MultiWriter(file, os.Stdout), opts)
	} else {
		// 仅输出到文件
		handler = slog.NewJSONHandler(file, opts)
	}

	return &Logger{
		logger:  slog.New(handler),
		file:    file,
		verbose: verbose,
	}, nil
}

// Debug 记录调试日志
func (l *Logger) Debug(msg string, args ...any) {
	l.logger.Debug(msg, args...)
}

// Info 记录信息日志
func (l *Logger) Info(msg string, args ...any) {
	l.logger.Info(msg, args...)
}

// Warn 记录警告日志
func (l *Logger) Warn(msg string, args ...any) {
	l.logger.Warn(msg, args...)
}

// Error 记录错误日志
func (l *Logger) Error(msg string, args ...any) {
	l.logger.Error(msg, args...)
}

// Fatal 记录致命错误并退出
func (l *Logger) Fatal(msg string, args ...any) {
	l.logger.Error(msg, args...)
	os.Exit(1)
}

// Close 关闭日志文件
func (l *Logger) Close() error {
	return l.file.Close()
}
