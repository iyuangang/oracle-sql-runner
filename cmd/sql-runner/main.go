package main

import (
	"log"
	"os"
	"runtime"

	"github.com/iyuangang/oracle-sql-runner/cmd"
	"github.com/iyuangang/oracle-sql-runner/logger"
)

func init() {
	// 设置最大处理器数
	runtime.GOMAXPROCS(runtime.NumCPU())
}

func main() {
	// 初始化日志
	if err := logger.Init(); err != nil {
		log.Fatal("初始化日志失败:", err)
	}
	defer logger.Sync()

	// 执行主程序
	if err := cmd.Execute(); err != nil {
		logger.Error("程序执行失败", "error", err)
		os.Exit(1)
	}
}
