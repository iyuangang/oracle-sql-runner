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

// setupLogger 初始化日志记录器
func setupLogger(cfg *config.Config, configDir string) (*utils.Logger, error) {
	logFile := cfg.LogFile
	if !filepath.IsAbs(logFile) {
		logFile = filepath.Join(configDir, logFile)
	}
	return utils.NewLogger(logFile, cfg.LogLevel, verbose)
}

// handleDatabasePasswords 处理数据库密码的加密和解密
func handleDatabasePasswords(cfg *config.Config, configPath string) error {
	configModified := false
	memoryConfig := &config.Config{
		Databases:      make(map[string]config.DatabaseConfig),
		MaxConcurrent: cfg.MaxConcurrent,
		LogFile:       cfg.LogFile,
		LogLevel:      cfg.LogLevel,
	}

	// 处理所有数据库的密码
	for name, dbConfig := range cfg.Databases {
		newConfig := dbConfig // 创建副本
		if utils.IsEncrypted(dbConfig.Password) {
			// 解密密码用于内存中的配置
			decrypted, err := utils.DecryptPassword(dbConfig.Password)
			if err != nil {
				return fmt.Errorf("解密数据库 %s 的密码失败: %w", name, err)
			}
			newConfig.Password = decrypted
			memoryConfig.Databases[name] = newConfig
		} else {
			// 加密密码用于保存到文件
			encrypted, err := utils.EncryptPassword(dbConfig.Password)
			if err != nil {
				return fmt.Errorf("加密数据库 %s 的密码失败: %w", name, err)
			}
			// 保存原始密码到内存配置
			memoryConfig.Databases[name] = dbConfig
			// 更新文件配置中的密码为加密版本
			newConfig.Password = encrypted
			cfg.Databases[name] = newConfig
			configModified = true
		}
	}

	// 如果有密码被加密，保存配置文件
	if configModified {
		if err := config.Save(configPath, cfg); err != nil {
			return fmt.Errorf("保存加密后的配置失败: %w", err)
		}
		if logger, err := utils.NewLogger("sql-runner.log", "info", verbose); err == nil {
			logger.Info("数据库密码已加密并保存到配置文件")
			logger.Close()
		}
	}

	// 用解密后的配置替换原配置
	*cfg = *memoryConfig
	return nil
}

// validateInputs 验证输入参数
func validateInputs(sqlFile, dbName string) error {
	if sqlFile == "" {
		return fmt.Errorf("请指定SQL文件路径 (-f)")
	}
	if dbName == "" {
		return fmt.Errorf("请指定数据库名称 (-d)")
	}
	if _, err := os.Stat(sqlFile); os.IsNotExist(err) {
		return fmt.Errorf("SQL文件不存在: %s", sqlFile)
	}
	return nil
}

// runSQL 执行SQL文件
func runSQL(cfg *config.Config, dbName, sqlFile string, logger *utils.Logger) error {
	// 检查数据库配置是否存在
	if _, ok := cfg.Databases[dbName]; !ok {
		return fmt.Errorf("数据库 %s 未配置", dbName)
	}

	// 创建执行器
	executor, err := core.NewExecutor(cfg, dbName, logger)
	if err != nil {
		return fmt.Errorf("创建执行器失败: %w", err)
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

// run 主要执行逻辑
func run(cmd *cobra.Command, args []string) error {
	// 验证输入参数
	if err := validateInputs(sqlFile, dbName); err != nil {
		return err
	}

	// 加载配置
	cfg, err := config.Load(configFile)
	if err != nil {
		return fmt.Errorf("加载配置失败: %w", err)
	}

	// 处理数据库密码
	if err := handleDatabasePasswords(cfg, configFile); err != nil {
		return err
	}

	// 设置日志记录器
	logger, err := setupLogger(cfg, filepath.Dir(configFile))
	if err != nil {
		return fmt.Errorf("初始化日志失败: %w", err)
	}
	defer logger.Close()

	// 记录启动信息
	logger.Info("启动SQL Runner",
		"version", Version,
		"build_time", BuildTime,
		"config", configFile,
		"sql_file", sqlFile,
		"database", dbName)

	// 执行SQL文件
	return runSQL(cfg, dbName, sqlFile, logger)
}

func main() {
	rootCmd := &cobra.Command{
		Use:   "sql-runner",
		Short: "Oracle SQL 脚本执行工具",
		Long: fmt.Sprintf(`Oracle SQL 脚本执行工具
版本: %s
提交: %s
构建时间: %s`, Version, Commit, BuildTime),
		Version: Version,
		RunE:    run,
	}

	// 设置命令行参数
	rootCmd.PersistentFlags().StringVarP(&configFile, "config", "c", "config.json", "配置文件路径")
	rootCmd.PersistentFlags().StringVarP(&sqlFile, "file", "f", "", "SQL文件路径")
	rootCmd.PersistentFlags().StringVarP(&dbName, "database", "d", "", "数据库名称")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "显示详细信息")

	// 加密命令
	var encryptPassword string
	encryptCmd := &cobra.Command{
		Use:   "encrypt",
		Short: "加密数据库密码",
		RunE: func(cmd *cobra.Command, args []string) error {
			if encryptPassword == "" {
				return fmt.Errorf("请提供密码")
			}
			encrypted, err := utils.EncryptPassword(encryptPassword)
			if err != nil {
				return fmt.Errorf("加密失败: %w", err)
			}
			fmt.Printf("加密后的密码: %s\n", encrypted)
			return nil
		},
	}
	encryptCmd.Flags().StringVarP(&encryptPassword, "password", "p", "", "要加密的密码")
	rootCmd.AddCommand(encryptCmd)

	// 解密命令
	var decryptPassword string
	decryptCmd := &cobra.Command{
		Use:   "decrypt",
		Short: "解密数据库密码",
		RunE: func(cmd *cobra.Command, args []string) error {
			if decryptPassword == "" {
				return fmt.Errorf("请提供加密密码")
			}
			decrypted, err := utils.DecryptPassword(decryptPassword)
			if err != nil {
				return fmt.Errorf("解密失败: %w", err)
			}
			fmt.Printf("解密后的密码: %s\n", decrypted)
			return nil
		},
	}
	decryptCmd.Flags().StringVarP(&decryptPassword, "password", "p", "", "要解密的密码")
	rootCmd.AddCommand(decryptCmd)

	// 执行命令
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		osExit(1)
	}
}
