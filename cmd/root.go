package cmd

import (
	"fmt"
	"time"

	"github.com/iyuangang/oracle-sql-runner/config"
	"github.com/iyuangang/oracle-sql-runner/executor"
	"github.com/iyuangang/oracle-sql-runner/utils"
	"github.com/spf13/cobra"
)

var (
	configFile   string
	dbName       string
	sqlFile      string
	parallel     int
	verbose      bool
	noProgress   bool
	validateOnly bool
	outputFormat string
)

var rootCmd = &cobra.Command{
	Use:   "sql-runner",
	Short: "Oracle SQL脚本执行工具",
	Long: `一个用于执行Oracle SQL脚本的命令行工具。
支持执行普通SQL语句、存储过程、匿名块等。
可以通过配置文件管理多个数据库连接。`,
	RunE: runExecute,
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.Flags().StringVarP(&configFile, "config", "c", "config.json", "配置文件路径")
	rootCmd.Flags().StringVarP(&dbName, "database", "d", "", "数据库名称")
	rootCmd.Flags().StringVarP(&sqlFile, "file", "f", "", "SQL文件路径")
	rootCmd.Flags().IntVarP(&parallel, "parallel", "p", 0, "并行度")
	rootCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "显示详细输出")
	rootCmd.Flags().BoolVar(&noProgress, "no-progress", false, "不显示进度条")
	rootCmd.Flags().BoolVar(&validateOnly, "validate", false, "仅验证SQL语法")
	rootCmd.Flags().StringVarP(&outputFormat, "output", "o", "text", "输出格式 (text/json)")

	rootCmd.MarkFlagRequired("database")
	rootCmd.MarkFlagRequired("file")

	// 添加子命令
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(testConnCmd)
	// rootCmd.AddCommand(encryptCmd)
}

func runExecute(cmd *cobra.Command, args []string) error {
	startTime := time.Now()

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

	// 更新执行配置
	if parallel > 0 {
		cfg.Execution.ParallelDegree = parallel
	}

	// 验证SQL文件
	if !utils.FileExists(sqlFile) {
		return fmt.Errorf("SQL文件不存在: %s", sqlFile)
	}

	// 创建执行器
	exec, err := executor.NewSQLExecutor(&dbConfig, &cfg.Execution)
	if err != nil {
		return fmt.Errorf("创建执行器失败: %w", err)
	}
	defer exec.Close()

	// 执行SQL文件
	result, err := exec.ExecuteFile(sqlFile)
	if err != nil {
		return fmt.Errorf("执行SQL文件失败: %w", err)
	}

	// 输出结果
	printResult(result, time.Since(startTime))

	return nil
}

func printResult(result *executor.ExecutionResult, duration time.Duration) {
	switch outputFormat {
	case "json":
		printJSONResult(result, duration)
	default:
		printTextResult(result, duration)
	}
}

func printTextResult(result *executor.ExecutionResult, duration time.Duration) {
	fmt.Printf("\n执行结果摘要:\n")
	fmt.Printf("总执行时间: %s\n", utils.FormatDuration(duration))
	fmt.Printf("成功语句数: %d\n", result.SuccessCount)
	fmt.Printf("失败语句数: %d\n", result.FailureCount)

	if result.FailureCount > 0 {
		fmt.Printf("\n错误详情:\n")
		for i, err := range result.Errors {
			fmt.Printf("%d. %v\n", i+1, err)
		}
	}
}

func printJSONResult(result *executor.ExecutionResult, duration time.Duration) {
	// 实现JSON格式输出
}
