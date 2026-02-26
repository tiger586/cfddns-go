package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "驗證配置檔案和環境變量",
	Long:  "驗證 config.yaml 和 .env 檔案的配置是否正確",
	Run: func(cmd *cobra.Command, args []string) {

		// 檢查檔案是否存在
		// fmt.Println()
		fmt.Println("📁 檔案檢查:")
		checkFileExists(".env")
		checkFileExists(getConfigPath())

		cfg, err := getConfig()
		if err != nil {
			fmt.Printf("❌ 加載配置失敗: %v\n", err)
			return
		}

		fmt.Println()
		fmt.Println("🔍 驗證配置...")
		printSeparator(50)

		// 驗證配置
		if err := cfg.Validate(); err != nil {
			fmt.Printf("❌ 配置驗證失敗: \n%v", err)
			return
		}

		fmt.Println("✅ 配置驗證成功!")
		fmt.Println()

		if _, err := os.Stat(".env"); err == nil {
			// 檢查環境變量
			fmt.Println("🌍 環境變量檢查:")
			checkEnvVar("CF_API_TOKEN")
			checkEnvVar("WEBHOOK_URL")
			checkEnvVar("WEBHOOK_CHAT_ID")
			fmt.Println()
		}

		// 顯示配置來源
		fmt.Println("📋 配置來源:")
		sources := cfg.GetConfigSource()
		fmt.Printf("   Cloudflare API Token: %s\n", sources["cloudflare.api_token"])
		fmt.Printf("   Webhook URL: %s\n", sources["webhook.url"])
		fmt.Printf("   Webhook Chat ID: %s\n", sources["webhook.chat_id"])
		fmt.Println()

		// 顯示配置摘要（隱藏敏感信息）
		fmt.Println("📋 配置摘要:")
		fmt.Printf("   Cloudflare API Token: %s\n", maskString(cfg.Cloudflare.APIToken, 8))
		fmt.Printf("   DNS 記錄數量: %d\n", len(cfg.DNSRecords))
		for i, record := range cfg.DNSRecords {
			ttlDesc := "自動"
			if record.TTL != 1 {
				ttlDesc = formatTTL(record.TTL)
			}
			fmt.Printf("     %d. %s (%s) - TTL: %s\n", i+1, record.Name, record.Type, ttlDesc)
		}
		fmt.Printf("   Webhook 啟用: %v\n", cfg.Webhook.Enabled)
		if cfg.Webhook.Enabled {
			fmt.Printf("   Webhook 類型: %s\n", cfg.Webhook.Type)
			fmt.Printf("   Webhook URL: %s\n", maskString(cfg.Webhook.URL, 20))
			if cfg.Webhook.ChatID != "" {
				fmt.Printf("   Chat ID: %s\n", maskString(cfg.Webhook.ChatID, 4))
			}
		}
		fmt.Printf("   檢查間隔: %d 秒\n", cfg.Global.CheckInterval)

	},
}

func checkEnvVar(name string) {
	value := os.Getenv(name)
	if value == "" {
		fmt.Printf("   ❌ %s: 未設置\n", name)
	} else {
		fmt.Printf("   ✅ %s: 已設置 (%s)\n", name, maskString(value, 8))
	}
}

func checkFileExists(filename string) {
	if _, err := os.Stat(filename); err != nil {
		fmt.Printf("   ❌ %s: 檔案不存在\n", filename)
	} else {
		fmt.Printf("   ✅ %s: 檔案存在\n", filename)
	}
}
