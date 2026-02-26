package cmd

import (
	"cfddns/cloudflare"
	"fmt"

	"github.com/spf13/cobra"
)

var testCmd = &cobra.Command{
	Use:   "test",
	Short: "測試 Cloudflare API 連接",
	Long:  "測試 Cloudflare API 令牌和 DNS 記錄訪問權限",
	Run: func(cmd *cobra.Command, args []string) {
		cfg, err := getConfig()
		if err != nil {
			fmt.Printf("❌ 加載配置失敗: %v\n", err)
			return
		}

		fmt.Println("🧪 Cloudflare API 測試工具")
		printSeparator(50)

		// 測試 API 連接
		cfClient := cloudflare.NewClient(&cfg.Cloudflare)
		cloudflare.SetVerbose(verbose)

		// 1. 測試 API Token
		fmt.Println("\n1. 🔗 測試 API Token...")
		if err := cfClient.TestConnection(); err != nil {
			fmt.Printf("❌ API Token 測試失敗: %v\n", err)
			return
		}

		// 2. 測試 DNS 記錄讀取
		fmt.Println("\n2. 📋 測試 DNS 記錄訪問...")
		if len(cfg.DNSRecords) == 0 {
			fmt.Println("⚠️  配置文件中沒有定義 DNS 記錄")
		} else {
			for i, record := range cfg.DNSRecords {
				fmt.Printf("   記錄 %d: %s (%s)... ", i+1, record.Name, record.Type)
				cfRecord, err := cfClient.GetDNSRecord(record.Name, record.Type)
				if err != nil {
					fmt.Printf("❌ 訪問失敗: %v\n", err)
				} else {
					fmt.Printf("✅ 成功 (IP: %s)\n", cfRecord.Content)
				}
			}
		}

		fmt.Println("\n🎉 所有測試完成!")
	},
}
