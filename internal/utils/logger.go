package utils

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
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
	file   io.WriteCloser
	logger *zap.Logger
}

const (
	DefaultLogFile = "sql-runner.log"
)

// getZapLevel 将字符串日志级别转换为 zapcore.Level
func getZapLevel(level string) zapcore.Level {
	switch strings.ToLower(level) {
	case "debug":
		return zapcore.DebugLevel
	case "info":
		return zapcore.InfoLevel
	case "warn":
		return zapcore.WarnLevel
	case "error":
		return zapcore.ErrorLevel
	default:
		return zapcore.InfoLevel
	}
}

// NewLogger 创建新的日志记录器
func NewLogger(logFile string, level string, jsonFormat bool) (*Logger, error) {
	// 确保日志目录存在
	logDir := filepath.Dir(logFile)
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return nil, fmt.Errorf("创建日志目录失败: %w", err)
	}

	// 尝试创建或打开日志文件
	file, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o666)
	if err != nil {
		return nil, fmt.Errorf("打开日志文件失败: %w", err)
	}

	// 创建编码器配置
	encoderConfig := zapcore.EncoderConfig{
		TimeKey:        "time",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		FunctionKey:    zapcore.OmitKey,
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.CapitalLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.StringDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	// 创建核心配置
	var core zapcore.Core
	if jsonFormat {
		core = zapcore.NewCore(
			zapcore.NewJSONEncoder(encoderConfig),
			zapcore.AddSync(file),
			getZapLevel(level),
		)
	} else {
		core = zapcore.NewCore(
			zapcore.NewConsoleEncoder(encoderConfig),
			zapcore.AddSync(file),
			getZapLevel(level),
		)
	}

	// 创建 logger
	logger := &Logger{
		file:   file,
		logger: zap.New(core, zap.AddCaller(), zap.AddCallerSkip(1)),
	}

	return logger, nil
}

// log 通用日志记录函数
func (l *Logger) log(level LogLevel, msg string, args ...any) {
	// 构造字段
	fields := make([]zap.Field, 0, len(args)/2+1)

	// 添加调用者信息
	if caller := getCallerInfo(2); caller != nil {
		fields = append(fields, zap.Any("source", caller))
	}

	// 处理参数
	if len(args) > 0 {
		argsMap := make(map[string]interface{})
		for i := 0; i < len(args); i += 2 {
			if i+1 < len(args) {
				if key, ok := args[i].(string); ok {
					argsMap[key] = args[i+1]
				}
			}
		}
		if len(argsMap) > 0 {
			fields = append(fields, zap.Any("args", argsMap))
		}
	}

	// 根据级别记录日志
	switch level {
	case LogLevelDebug:
		l.logger.Debug(msg, fields...)
	case LogLevelInfo:
		l.logger.Info(msg, fields...)
	case LogLevelWarn:
		l.logger.Warn(msg, fields...)
	case LogLevelError:
		l.logger.Error(msg, fields...)
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
	if l.logger != nil {
		_ = l.logger.Sync()
	}
	if l.file != nil {
		if err := l.file.Close(); err != nil {
			return fmt.Errorf("关闭日志文件失败: %w", err)
		}
		l.file = nil
	}
	return nil
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
