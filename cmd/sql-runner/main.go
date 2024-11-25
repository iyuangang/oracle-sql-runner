package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/iyuangang/oracle-sql-runner/internal/config"
	"github.com/iyuangang/oracle-sql-runner/internal/core"
	"github.com/iyuangang/oracle-sql-runner/internal/utils"
)

var (
	Version   = "dev"
	BuildTime = "unknown"
)

func main() {
	// 命令行参数
	configFile := flag.String("c", "config.json", "配置文件路径")
	sqlFile := flag.String("f", "", "SQL文件路径")
	dbName := flag.String("d", "", "数据库名称")
	verbose := flag.Bool("v", false, "显示详细信息")
	showVersion := flag.Bool("version", false, "显示版本信息")
	flag.Parse()

	// 显示版本信息
	if *showVersion {
		fmt.Printf("SQL Runner v%s (构建时间: %s)\n", Version, BuildTime)
		os.Exit(0)
	}

	// 验证必要参数
	if *sqlFile == "" {
		fmt.Println("错误: 请指定SQL文件路径 (-f)")
		flag.Usage()
		os.Exit(1)
	}

	if *dbName == "" {
		fmt.Println("错误: 请指定数据库名称 (-d)")
		flag.Usage()
		os.Exit(1)
	}

	// 加载配置
	cfg, err := config.Load(*configFile)
	if err != nil {
		fmt.Printf("加载配置失败: %v\n", err)
		os.Exit(1)
	}

	// 创建日志目录
	logDir := "logs"
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		fmt.Printf("创建日志目录失败: %v\n", err)
		os.Exit(1)
	}

	// 初始化日志
	logFile := filepath.Join(logDir, "sql-runner.log")
	logger, err := utils.NewLogger(logFile, cfg.LogLevel, *verbose)
	if err != nil {
		fmt.Printf("初始化日志失败: %v\n", err)
		os.Exit(1)
	}
	defer logger.Close()

	logger.Info("启动SQL Runner",
		"version", Version,
		"build_time", BuildTime,
		"config", *configFile,
		"sql_file", *sqlFile,
		"database", *dbName)

	// 创建执行器
	executor, err := core.NewExecutor(cfg, *dbName, logger)
	if err != nil {
		logger.Fatal("创建执行器失败", "error", err)
	}
	defer executor.Close()

	// 执行SQL文件
	result := executor.ExecuteFile(*sqlFile)

	// 输出结果
	result.Print()

	// 根据执行结果设置退出码
	if result.Failed > 0 {
		os.Exit(1)
	}
}
