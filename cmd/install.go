package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"text/template"

	"github.com/spf13/cobra"
)

const serviceTemplate = `[Unit]
Description=Cloudflare DDNS Client
Documentation=https://github.com/tiger586/cfddns-go
After=network.target network-online.target
Wants=network-online.target
Requires=network-online.target

[Service]
Type=simple
User=root
Group=root
ExecStart={{.BinaryPath}} run --config {{.ConfigPath}}
ExecReload=/bin/kill -HUP $MAINPID
Restart=always
RestartSec=10
StandardOutput=journal
StandardError=journal
SyslogIdentifier=cfddns

# 等待網路完全就緒
# ExecStartPre=/bin/sleep 5
ExecStartPre=/bin/sh -c 'until ping -c1 8.8.8.8; do sleep 2; done'

# 安全設定
NoNewPrivileges=yes
PrivateTmp=yes
ProtectSystem=strict
ProtectHome=yes
ReadWritePaths=/etc/cfddns /var/cache/cfddns
ProtectKernelTunables=yes
ProtectKernelModules=yes
ProtectControlGroups=yes

# 環境設定
Environment=GOMAXPROCS=1

[Install]
WantedBy=multi-user.target
`

var installCmd = &cobra.Command{
	Use:   "install",
	Short: "安裝為係統服務",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("開始安裝 Cloudflare DDNS 服務...")

		// 獲取當前可執行文件路徑
		exePath, err := os.Executable()
		if err != nil {
			fmt.Printf("❌ 獲取可執行文件路徑失敗: %v\n", err)
			return
		}

		// 複製可執行文件到 /usr/local/bin/
		targetBinary := "/usr/local/bin/cfddns"
		fmt.Printf("📦 複製可執行文件到 %s...\n", targetBinary)

		if err := copyFile(exePath, targetBinary); err != nil {
			fmt.Printf("❌ 複製可執行文件失敗: %v\n", err)
			return
		}

		// 設定可執行權限
		if err := os.Chmod(targetBinary, 0755); err != nil {
			fmt.Printf("❌ 設定可執行權限失敗: %v\n", err)
			return
		}

		// 創建配置目錄
		configDir := "/etc/cfddns"
		fmt.Printf("📁 創建配置目錄 %s...\n", configDir)
		if err := os.MkdirAll(configDir, 0755); err != nil {
			fmt.Printf("❌ 創建配置目錄失敗: %v\n", err)
			return
		}

		// 創建暫存目錄
		cacheDir := "/var/cache/cfddns"
		fmt.Printf("📁 創建暫存目錄 %s...\n", cacheDir)
		if err := os.MkdirAll(cacheDir, 0755); err != nil {
			fmt.Printf("❌ 創建暫存目錄失敗: %v\n", err)
			return
		}

		// 設置暫存目錄權限
		if err := os.Chown(cacheDir, 0, 0); err != nil {
			fmt.Printf("⚠️  設置暫存目錄所有者失敗: %v\n", err)
		}
		if err := os.Chmod(cacheDir, 0755); err != nil {
			fmt.Printf("⚠️  設置暫存目錄權限失敗: %v\n", err)
		}

		// 確定配置文件路徑
		configPath := cfgFile
		if configPath == "" {
			configPath = filepath.Join(configDir, "config.yaml")
			// 如果預設配置文件不存在，創建示例配置
			if _, err := os.Stat(configPath); os.IsNotExist(err) {
				if err := createExampleConfig(configPath); err != nil {
					fmt.Printf("⚠️ 創建範例配置文件失敗: %v\n", err)
				} else {
					fmt.Printf("📄 創建範例配置文件: %s\n", configPath)
				}
			}
		}

		// // 創建快取目錄
		// cachePath := "/var/cache/cfddns"
		// fmt.Printf("📁 創建快取目錄 %s...\n", cachePath)
		// if err := os.MkdirAll(cachePath, 0755); err != nil {
		// 	fmt.Printf("❌ 創建快取目錄失敗: %v\n", err)
		// 	return
		// }

		// 創建服務文件
		serviceData := struct {
			BinaryPath string
			ConfigPath string
		}{
			BinaryPath: targetBinary,
			ConfigPath: configPath,
		}

		serviceDir := "/etc/systemd/system"
		servicePath := filepath.Join(serviceDir, "cfddns.service")

		fmt.Printf("🔧 創建服務文件 %s...\n", servicePath)

		tmpl, err := template.New("service").Parse(serviceTemplate)
		if err != nil {
			fmt.Printf("❌ 解析服務模闆失敗: %v\n", err)
			return
		}

		file, err := os.Create(servicePath)
		if err != nil {
			fmt.Printf("❌ 創建服務文件失敗: %v\n", err)
			return
		}
		defer file.Close()

		if err := tmpl.Execute(file, serviceData); err != nil {
			fmt.Printf("❌ 生成服務文件失敗: %v\n", err)
			return
		}

		// 重載 systemd
		fmt.Println("🔄 重載 systemd 配置...")
		if err := exec.Command("systemctl", "daemon-reload").Run(); err != nil {
			fmt.Printf("❌ 重載 systemd 失敗: %v\n", err)
			return
		}

		// 啓用服務
		fmt.Println("✅ 啓用服務...")
		if err := exec.Command("systemctl", "enable", "cfddns.service").Run(); err != nil {
			fmt.Printf("❌ 啓用服務失敗: %v\n", err)
			return
		}

		fmt.Println("\n🎉 服務安裝成功!")
		fmt.Printf("📁 配置文件路徑: %s\n", configPath)
		fmt.Printf("⚙️  可執行文件: %s\n", targetBinary)
		fmt.Println("\n📋 管理命令:")
		fmt.Println("   啓動服務: systemctl start cfddns")
		fmt.Println("   停止服務: systemctl stop cfddns")
		fmt.Println("   重啓服務: systemctl restart cfddns")
		fmt.Println("   檢視狀態: systemctl status cfddns")
		fmt.Println("   檢視日誌: journalctl -u cfddns -f")
		fmt.Println("\n💡 請編輯配置文件後啓動服務:")
		fmt.Printf("   sudo nano %s\n", configPath)
		fmt.Println("   sudo systemctl start cfddns")
	},
}

func copyFile(src, dst string) error {
	input, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, input, 0755)
}

func createExampleConfig(configPath string) error {
	exampleConfig := `# Cloudflare DDNS 配置示例
# 請根據實際情況修改以下配置

# 全局配置
global:
  check_interval: 600  # 檢查間隔(秒)
  ip_check_urls:       # 檢查 IP 的網站（可自行增加）
    - "https://api.ipify.org"
    - "https://icanhazip.com"
    - "https://ident.me"
    - "https://4.ipw.cn"

# Cloudflare 配置
cloudflare:
  api_token: ""  # 替換為您的 API Token

# DNS 記錄配置
dns_records:
  - name: "www.example.com"
    type: "A"
    proxied: true   # Proxy 狀態：打開小雲朵 true，關閉 = false
    ttl: 1          # 1 = 自動 TTL，1 分鐘 = 60（秒數）

# Webhook 配置
webhook:
  enabled: true
  type: "telegram"  # 新增：telegram 或 generic
  url: ""
  chat_id: ""
  on_success: true
  on_failure: true
  template: "text"  # 改為 text, markdown, 或 html  
`

	return os.WriteFile(configPath, []byte(exampleConfig), 0644)
}
