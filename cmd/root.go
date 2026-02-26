package cmd

import (
	"cfddns/config"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	cfgFile string
	verbose bool
	rootCmd = &cobra.Command{
		Use:   "cfddns",
		Short: "Cloudflare DDNS 客戶端",
		Long:  "基於 Cloudflare API 的動態 DNS 客戶端，支援 webhook 通知",
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			if verbose {
				fmt.Printf("🔧 詳細模式已啟用\n")
				fmt.Printf("🔧 加載配置文件: %s\n", getConfigPath())
			}
		},
	}
)

// 共用輔助函數
func printSeparator(length int) {
	for i := 0; i < length; i++ {
		fmt.Print("=")
	}
	fmt.Println()
}

func maskString(s string, showLen int) string {
	if len(s) <= showLen {
		return "***"
	}
	return s[:min(showLen, len(s))] + "***"
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "配置文件路徑 (默認: ./config.yaml 或 /etc/cfddns/config.yaml)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "詳細輸出")

	// 添加所有子命令
	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(installCmd)
	rootCmd.AddCommand(uninstallCmd)
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(webhookCmd)
	rootCmd.AddCommand(testCmd)
	rootCmd.AddCommand(validateCmd)
}

func getConfigPath() string {
	if cfgFile != "" {
		return cfgFile
	}
	return config.GetDefaultConfigPath()
}

func getConfig() (*config.Config, error) {
	cfg, err := config.LoadConfig(getConfigPath())
	if err != nil {
		return nil, err
	}

	// 驗證配置
	if err := cfg.Validate(); err != nil && verbose {
		fmt.Printf("⚠️  配置驗證警告: %v\n", err)
	}

	return cfg, nil
}
