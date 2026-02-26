package cmd

import (
	"cfddns/service"
	"cfddns/webhook"
	"fmt"

	"github.com/spf13/cobra"
)

var (
	webhookMessage string
	webhookType    string
)

var webhookCmd = &cobra.Command{
	Use:   "webhook",
	Short: "發送 Webhook 測試訊息",
	Long:  "發送測試訊息到配置的 Webhook URL，用於測試通知功能",
	Run: func(cmd *cobra.Command, args []string) {
		cfg, err := getConfig()
		if err != nil {
			fmt.Printf("❌ 加載配置失敗: %v\n", err)
			return
		}

		if !cfg.Webhook.Enabled {
			fmt.Println("❌ Webhook 功能未啟用")
			return
		}

		// 先檢查訊息類型
		validTypes := map[string]bool{
			"info":    true,
			"success": true,
			"error":   true,
			"":        true, // 空字符串也視為 info
		}

		if !validTypes[webhookType] {
			fmt.Printf("❌ 不支援的訊息類型: %s\n", webhookType)
			fmt.Println("✅ 支援的類型: info, success, error")
			return
		}

		// 現在才創建 webhookClient，確保一定會被使用
		webhookClient := webhook.NewClient(
			cfg.Webhook.URL,
			cfg.Webhook.ChatID,
			cfg.Webhook.Type,
			cfg.Webhook.Template,
			cfg.Webhook.Enabled,
			cfg.Webhook.OnSuccess,
			cfg.Webhook.OnFailure,
		)

		// 其餘代碼保持不變...
		ddnsService := service.NewDDNSService(cfg)
		currentIP, ipErr := ddnsService.GetCurrentIP()
		if ipErr != nil && verbose {
			fmt.Printf("⚠️  獲取當前 IP 失敗: %v\n", ipErr)
		}

		fmt.Printf("🔔 發送 Webhook 測試訊息到: %s\n", cfg.Webhook.URL)

		var sendErr error
		message := webhookMessage
		if message == "" {
			message = "這是一條測試訊息來自 Cloudflare DDNS 客戶端"
		}

		switch webhookType {
		case "success":
			sendErr = webhookClient.SendSuccess(currentIP, currentIP, "test.example.com")
			fmt.Println("📤 發送成功通知...")
		case "error":
			sendErr = webhookClient.SendFailure("test.example.com", "這是一個測試錯誤訊息")
			fmt.Println("📤 發送錯誤通知...")
		default: // 包括 "info" 和空字符串
			if webhookMessage == "" {
				message = "DDNS 服務測試通知"
			}
			sendErr = webhookClient.SendInfo(message)
			fmt.Println("📤 發送信息通知...")
		}

		if sendErr != nil {
			fmt.Printf("❌ 發送 Webhook 失敗: %v\n", sendErr)
			return
		}

		fmt.Println("✅ Webhook 訊息發送成功!")
		fmt.Printf("📝 訊息類型: %s\n", webhookType)
		fmt.Printf("💬 訊息內容: %s\n", message)
		if currentIP != "" {
			fmt.Printf("🌐 當前 IP: %s\n", currentIP)
		}
	},
}

func init() {
	webhookCmd.Flags().StringVarP(&webhookMessage, "message", "m", "", "自定義訊息內容")
	webhookCmd.Flags().StringVarP(&webhookType, "type", "t", "info", "訊息類型 (info|success|error)")
}
