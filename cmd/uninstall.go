package cmd

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
)

var uninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "卸載係統服務",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("開始卸載 Cloudflare DDNS 服務...")

		// 停止服務
		fmt.Println("🛑 停止服務...")
		exec.Command("systemctl", "stop", "cfddns.service").Run()

		// 禁用服務
		fmt.Println("❌ 禁用服務...")
		if err := exec.Command("systemctl", "disable", "cfddns.service").Run(); err != nil {
			fmt.Printf("⚠️  禁用服務失敗: %v\n", err)
		}

		// 刪除服務文件
		servicePath := "/etc/systemd/system/cfddns.service"
		fmt.Printf("🗑️  刪除服務文件 %s...\n", servicePath)
		if err := os.Remove(servicePath); err != nil {
			fmt.Printf("⚠️  刪除服務文件失敗: %v\n", err)
		}

		// 刪除可執行文件
		binaryPath := "/usr/local/bin/cfddns"
		fmt.Printf("🗑️  刪除可執行文件 %s...\n", binaryPath)
		if err := os.Remove(binaryPath); err != nil {
			fmt.Printf("⚠️  刪除可執行文件失敗: %v\n", err)
		}

		// 重載 systemd
		fmt.Println("🔄 重載 systemd 配置...")
		if err := exec.Command("systemctl", "daemon-reload").Run(); err != nil {
			fmt.Printf("⚠️  重載 systemd 失敗: %v\n", err)
		}

		// 重置失敗的服務狀態
		exec.Command("systemctl", "reset-failed").Run()

		fmt.Println("\n✅ 服務卸載完成!")
		fmt.Println("💡 配置文件 /etc/cfddns/config.yaml 需要手動刪除")
	},
}
