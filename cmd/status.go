package cmd

import (
	"cfddns/cloudflare"
	"cfddns/service"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "檢視 DNS 記錄狀態",
	Long:  "顯示設定的 DNS 記錄當前狀態和同步情況",
	Run: func(cmd *cobra.Command, args []string) {
		cfg, err := getConfig()
		if err != nil {
			fmt.Printf("❌ 加載配置失敗: %v\n", err)
			return
		}

		service.SetVerbose(verbose)
		cloudflare.SetVerbose(verbose)

		// 創建服務實例
		ddnsService := service.NewDDNSService(cfg)

		fmt.Println("🌐 DNS 記錄狀態檢查")
		printSeparator(50)

		// 獲取當前公共 IP
		currentIP, err := ddnsService.GetCurrentIP()
		if err != nil {
			fmt.Printf("❌ 獲取當前 IP 失敗: %v\n", err)
			currentIP = "未知"
		} else {
			fmt.Printf("📡 當前公共 IP: %s\n\n", currentIP)
		}

		// 顯示設定的 DNS 記錄狀態
		fmt.Println("📋 設定的 DNS 記錄狀態:")

		if len(cfg.DNSRecords) == 0 {
			fmt.Println("❌ 未設定任何 DNS 記錄")
			return
		}

		// 使用 tabwriter 來美化輸出
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
		fmt.Fprintln(w, "名稱\t類型\t代理\tTTL\tDNS IP\t狀態\t同步")
		fmt.Fprintln(w, "----\t----\t----\t---\t-------\t----\t----")

		cfClient := cloudflare.NewClient(&cfg.Cloudflare)
		successCount := 0
		totalCount := len(cfg.DNSRecords)

		for _, record := range cfg.DNSRecords {
			// 獲取 Cloudflare 中的實際記錄
			cfRecord, err := cfClient.GetDNSRecord(record.Name, record.Type)

			var dnsIP string
			var status string
			var syncStatus string

			if err != nil {
				dnsIP = "❌ 獲取失敗"
				status = "缺失"
				syncStatus = "❌"
			} else {
				dnsIP = cfRecord.Content
				status = "存在"

				// 檢查同步狀態
				if currentIP != "未知" && cfRecord.Content == currentIP {
					syncStatus = "✅"
					successCount++
				} else if currentIP != "未知" {
					syncStatus = "⚠️"
				} else {
					syncStatus = "❓"
				}
			}

			// 代理狀態
			proxiedStatus := "關閉"
			if record.Proxied {
				proxiedStatus = "開啟"
			}

			// 配置的 TTL
			configTTL := formatTTL(record.TTL)
			if record.TTL == 1 {
				configTTL = "自動"
			}

			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
				record.Name,
				record.Type,
				proxiedStatus,
				configTTL, // 使用配置的 TTL
				dnsIP,
				status,
				syncStatus)
		}
		w.Flush()

		// 顯示摘要信息
		fmt.Printf("\n📊 摘要: ")
		if successCount == totalCount && currentIP != "未知" {
			fmt.Printf("✅ 所有記錄已同步 (%d/%d)\n", successCount, totalCount)
		} else if currentIP != "未知" {
			fmt.Printf("⚠️  %d/%d 個記錄已同步\n", successCount, totalCount)
		} else {
			fmt.Printf("❓ 無法檢查同步狀態 (IP 獲取失敗)\n")
		}

		fmt.Printf("⏰ 檢查時間: %s\n", time.Now().Format("2006-01-02 15:04:05"))
	},
}

// 格式化 TTL 顯示
func formatTTL(ttl int) string {
	if ttl == 1 {
		return "自動"
	}

	// 轉換為更易讀的格式
	if ttl < 60 {
		return fmt.Sprintf("%d秒", ttl)
	} else if ttl < 3600 {
		return fmt.Sprintf("%d分", ttl/60)
	} else if ttl < 86400 {
		return fmt.Sprintf("%d時", ttl/3600)
	} else {
		return fmt.Sprintf("%d天", ttl/86400)
	}
}
