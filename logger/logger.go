package logger

import (
	"sync"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	Log   *zap.Logger
	Sugar *zap.SugaredLogger
	once  sync.Once
)

func Init() error {
	var err error
	once.Do(func() {
		// 基础配置
		config := zap.NewProductionConfig()
		config.OutputPaths = []string{"sql-runner.log", "stdout"}
		config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

		// 创建logger
		Log, err = config.Build(zap.AddCallerSkip(1))
		if err != nil {
			return
		}

		Sugar = Log.Sugar()
	})
	return err
}

func Sync() {
	if Log != nil {
		Log.Sync()
	}
}

// 日志方法包装
func Info(msg string, keysAndValues ...interface{}) {
	Sugar.Infow(msg, keysAndValues...)
}

func Error(msg string, keysAndValues ...interface{}) {
	Sugar.Errorw(msg, keysAndValues...)
}

func Debug(msg string, keysAndValues ...interface{}) {
	Sugar.Debugw(msg, keysAndValues...)
}

func Warn(msg string, keysAndValues ...interface{}) {
	Sugar.Warnw(msg, keysAndValues...)
}
