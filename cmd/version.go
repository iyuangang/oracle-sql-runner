package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	Version   = "1.0.0"
	BuildTime = "unknown"
	GitCommit = "unknown"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "显示版本信息",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("Oracle SQL Runner v%s\n", Version)
		fmt.Printf("构建时间: %s\n", BuildTime)
		fmt.Printf("Git提交: %s\n", GitCommit)
	},
}
