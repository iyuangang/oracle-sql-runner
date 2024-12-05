package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/iyuangang/oracle-sql-runner/internal/config"
	"github.com/iyuangang/oracle-sql-runner/internal/core"
	"github.com/iyuangang/oracle-sql-runner/internal/utils"
	"github.com/spf13/cobra"
)

var (
	// 版本信息，通过编译时注入
	Version   = "dev"
	Commit    = "none"
	BuildTime = "unknown"

	// 命令行参数
	configFile string
	sqlFile    string
	dbName     string
	verbose    bool
	osExit     = os.Exit
)

var rootCmd = &cobra.Command{
	Use:   "sql-runner",
	Short: "Oracle SQL 脚本执行工具",
	Long: fmt.Sprintf(`Oracle SQL 脚本执行工具
版本: %s
提交: %s
构建时间: %s`, Version, Commit, BuildTime),
	Version: Version,
	RunE:    run,
}

var encryptCmd = &cobra.Command{
	Use:   "encrypt",
	Short: "加密数据库密码",
	RunE:  runEncrypt,
}

var decryptCmd = &cobra.Command{
	Use:   "decrypt",
	Short: "解密数据库密码",
	RunE:  runDecrypt,
}

func runEncrypt(cmd *cobra.Command, args []string) error {
	password := cmd.Flag("password").Value.String()
	if password == "" {
		return fmt.Errorf("请提供密码")
	}

	encrypted, err := utils.EncryptPassword(password)
	if err != nil {
		return fmt.Errorf("加密失败: %w", err)
	}

	fmt.Printf("加密后的密码: %s\n", encrypted)
	return nil
}

func runDecrypt(cmd *cobra.Command, args []string) error {
	password := cmd.Flag("password").Value.String()
	if password == "" {
		return fmt.Errorf("请提供加密密码")
	}

	decrypted, err := utils.DecryptPassword(password)
	if err != nil {
		return fmt.Errorf("解密失败: %w", err)
	}

	fmt.Printf("解密后的密码: %s\n", decrypted)
	return nil
}

func run(cmd *cobra.Command, args []string) error {
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
		return fmt.Errorf("加载配置失败: %w", err)
	}

	// 检查并处理数据库密码
	for name, dbConfig := range cfg.Databases {
		if !utils.IsEncrypted(dbConfig.Password) {
			encrypted, err := utils.EncryptPassword(dbConfig.Password)
			if err != nil {
				return fmt.Errorf("加密数据库 %s 的密码失败: %w", name, err)
			}
			dbConfig.Password = encrypted
			cfg.Databases[name] = dbConfig
		}
	}

	// 保存更新后的配置
	if err := config.Save(configFile, cfg); err != nil {
		return fmt.Errorf("保存配置失败: %w", err)
	}

	// 确保日志文件路径是绝对路径
	logFile := cfg.LogFile
	if !filepath.IsAbs(logFile) {
		// 使用配置文件所在目录作为基准目录
		configDir := filepath.Dir(configFile)
		logFile = filepath.Join(configDir, logFile)
	}

	// 初始化日志
	logger, err := utils.NewLogger(logFile, cfg.LogLevel, verbose)
	if err != nil {
		return fmt.Errorf("初始化日志失败: %w", err)
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
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&configFile, "config", "c", "config.json", "配置文件路径")
	rootCmd.PersistentFlags().StringVarP(&sqlFile, "file", "f", "", "SQL文件路径")
	rootCmd.PersistentFlags().StringVarP(&dbName, "database", "d", "", "数据库名称")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "显示详细信息")

	encryptCmd.Flags().String("password", "", "要加密的密码")
	decryptCmd.Flags().String("password", "", "要解密的密码")

	rootCmd.AddCommand(encryptCmd)
	rootCmd.AddCommand(decryptCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		osExit(1)
	}
}
