package main

import (
	"fmt"
	"os"

	"github.com/iyuangang/oracle-sql-runner/internal/config"
	"github.com/iyuangang/oracle-sql-runner/internal/core"
	"github.com/iyuangang/oracle-sql-runner/internal/utils"
	"github.com/spf13/cobra"
)

var (
	Version   = "dev"
	BuildTime = "unknown"

	// 命令行参数
	configFile string
	sqlFile    string
	dbName     string
	verbose    bool

	osExit = os.Exit
)

var rootCmd = &cobra.Command{
	Use:     "sql-runner",
	Short:   "Oracle SQL 脚本执行工具",
	Version: Version,
	RunE: func(cmd *cobra.Command, args []string) error {
		// 验证必要参数
		if sqlFile == "" {
			return fmt.Errorf("请指定SQL文件路径 (-f)")
		}

		if dbName == "" {
			return fmt.Errorf("请指定数据库名称 (-d)")
		}

		// 加载配置
		cfg, err := config.Load(configFile)
		if err != nil {
			return fmt.Errorf("加载配置失败: %v", err)
		}

		// 创建日志目录和初始化日志
		logger, err := utils.NewLogger(cfg.LogFile, cfg.LogLevel, verbose)
		if err != nil {
			return fmt.Errorf("初始化日志失败: %v", err)
		}
		defer logger.Close()

		logger.Info("启动SQL Runner",
			"version", Version,
			"build_time", BuildTime,
			"config", configFile,
			"sql_file", sqlFile,
			"database", dbName)

		// 创建执行器
		executor, err := core.NewExecutor(cfg, dbName, logger)
		if err != nil {
			return fmt.Errorf("创建执行器失败: %v", err)
		}
		defer executor.Close()

		// 执行SQL文件
		result := executor.ExecuteFile(sqlFile)
		result.Print()

		if result.Failed > 0 {
			return fmt.Errorf("执行失败")
		}

		return nil
	},
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&configFile, "config", "c", "config.json", "配置文件路径")
	rootCmd.PersistentFlags().StringVarP(&sqlFile, "file", "f", "", "SQL文件路径")
	rootCmd.PersistentFlags().StringVarP(&dbName, "database", "d", "", "数据库名称")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "显示详细信息")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		osExit(1)
	}
}
