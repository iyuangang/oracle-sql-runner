package cmd

import (
	"fmt"
	"time"

	"github.com/iyuangang/oracle-sql-runner/config"
	"github.com/iyuangang/oracle-sql-runner/executor"
	"github.com/spf13/cobra"
)

var testConnCmd = &cobra.Command{
	Use:   "test-connection",
	Short: "测试数据库连接",
	RunE: func(cmd *cobra.Command, args []string) error {
		// 加载配置
		cfg, err := config.LoadConfig(configFile)
		if err != nil {
			return fmt.Errorf("加载配置失败: %w", err)
		}

		// 获取数据库配置
		dbConfig, ok := cfg.Databases[dbName]
		if !ok {
			return fmt.Errorf("未找到数据库配置: %s", dbName)
		}

		// 创建连接池
		pool, err := executor.NewConnectionPool(&dbConfig)
		if err != nil {
			return fmt.Errorf("连接数据库失败: %w", err)
		}
		defer pool.Close()

		// 测试连接
		start := time.Now()
		if err := pool.Ping(); err != nil {
			return fmt.Errorf("连接测试失败: %w", err)
		}

		fmt.Printf("成功连接到数据库 %s (%s)\n", dbName, dbConfig.Name)
		fmt.Printf("响应时间: %s\n", time.Since(start))

		// 显示连接池统计信息
		stats := pool.Stats()
		fmt.Printf("\n连接池统计:\n")
		fmt.Printf("打开的连接数: %d\n", stats.OpenConnections)
		fmt.Printf("使用中的连接数: %d\n", stats.InUse)
		fmt.Printf("空闲连接数: %d\n", stats.Idle)

		return nil
	},
}
