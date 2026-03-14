package service

import (
	"cfddns/cloudflare"
	"cfddns/config"
	"cfddns/webhook"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type DDNSService struct {
	config    *config.Config
	cfClient  *cloudflare.CloudflareClient
	webhook   *webhook.WebhookClient
	currentIP string            // 當前的公共 IP
	dnsIPs    map[string]string // 記錄名稱 -> DNS 記錄中的 IP
	cacheFile string            // IP 暫存檔案路徑
	stopChan  chan bool
	lastCheck time.Time
	nextCheck time.Time
}

// IP 暫存資料結構
type IPCache struct {
	LastIP     string            `json:"last_ip"`
	LastUpdate time.Time         `json:"last_update"`
	DNSRecords map[string]string `json:"dns_records"` // 記錄名稱 -> 最後已知的 DNS IP
}

var verbose bool

func NewDDNSService(cfg *config.Config) *DDNSService {
	cfClient := cloudflare.NewClient(&cfg.Cloudflare)

	webhookClient := webhook.NewClient(
		cfg.Webhook.URL,
		cfg.Webhook.ChatID,
		cfg.Webhook.Type,
		cfg.Webhook.Template,
		cfg.Webhook.Enabled,
		cfg.Webhook.OnSuccess,
		cfg.Webhook.OnFailure,
	)

	// 設定暫存檔案路徑
	cacheFile := getCacheFilePath()

	now := time.Now()
	service := &DDNSService{
		config:    cfg,
		cfClient:  cfClient,
		webhook:   webhookClient,
		dnsIPs:    make(map[string]string),
		cacheFile: cacheFile,
		stopChan:  make(chan bool),
		lastCheck: now,
		nextCheck: now.Add(time.Duration(cfg.Global.CheckInterval) * time.Second),
	}

	// 載入暫存的 IP 資料
	service.loadIPCache()

	return service
}

// 取得暫存檔案路徑
func getCacheFilePath() string {
	// 優先使用 /var/cache/cfddns/ (系統服務)
	if _, err := os.Stat("/var/cache/cfddns"); err == nil {
		return "/var/cache/cfddns/ip_cache.json"
	}

	// 使用當前目錄
	return "ip_cache.json"
}

// 等待網路連線
func waitForNetwork() {
	time.Sleep(5 * time.Second)

	for {
		conn, err := net.DialTimeout("tcp", "1.1.1.1:53", 3*time.Second)
		if err == nil {
			conn.Close()
			fmt.Println("network ready")
			return
		}
		fmt.Println("network not ready, retrying...")
		time.Sleep(2 * time.Second)
	}
}

// 載入 IP 暫存資料
func (d *DDNSService) loadIPCache() {
	if _, err := os.Stat(d.cacheFile); err != nil {
		if verbose {
			fmt.Printf("📁 暫存檔案不存在: %s\n", d.cacheFile)
		}
		return
	}

	data, err := os.ReadFile(d.cacheFile)
	if err != nil {
		fmt.Printf("⚠️  讀取暫存檔案失敗: %v\n", err)
		return
	}

	var cache IPCache
	if err := json.Unmarshal(data, &cache); err != nil {
		fmt.Printf("⚠️  解析暫存資料失敗: %v\n", err)
		return
	}

	// 檢查暫存資料是否過期（超過 24 小時）
	if time.Since(cache.LastUpdate) > 24*time.Hour {
		if verbose {
			fmt.Printf("🕒 暫存資料已過期 (超過 24 小時)\n")
		}
		return
	}

	d.currentIP = cache.LastIP
	d.dnsIPs = cache.DNSRecords

	if verbose {
		fmt.Printf("📁 載入暫存 IP: %s\n", d.currentIP)
		fmt.Printf("📋 暫存記錄數量: %d\n", len(d.dnsIPs))
	}
}

// 儲存 IP 暫存資料
func (d *DDNSService) saveIPCache() { // 修正：移除多餘的 N
	// 確保暫存目錄存在
	cacheDir := filepath.Dir(d.cacheFile)
	if cacheDir != "." {
		if err := os.MkdirAll(cacheDir, 0755); err != nil {
			fmt.Printf("⚠️  創建暫存目錄失敗: %v\n", err)
			return
		}
	}

	cache := IPCache{
		LastIP:     d.currentIP,
		LastUpdate: time.Now(),
		DNSRecords: d.dnsIPs,
	}

	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		fmt.Printf("⚠️  序列化暫存資料失敗: %v\n", err)
		return
	}

	if err := os.WriteFile(d.cacheFile, data, 0644); err != nil {
		fmt.Printf("⚠️  寫入暫存檔案失敗: %v\n", err)
		return
	}

	if verbose {
		fmt.Printf("💾 暫存 IP 資料: %s\n", d.currentIP)
	}
}

func SetVerbose(v bool) {
	verbose = v
}

func (d *DDNSService) GetCurrentIP() (string, error) {
	var lastErr error

	if verbose {
		fmt.Printf("🔍 正在檢查公共 IP...\n")
	}

	for i, url := range d.config.Global.IPCheckURLs {
		if verbose {
			fmt.Printf("   嘗試服務 %d: %s\n", i+1, url)
		}

		resp, err := http.Get(url)
		if err != nil {
			lastErr = fmt.Errorf("服務 %s 失敗: %w", url, err)
			if verbose {
				fmt.Printf("   ❌ %v\n", lastErr)
			}
			continue
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			lastErr = fmt.Errorf("讀取響應失敗: %w", err)
			if verbose {
				fmt.Printf("   ❌ %v\n", lastErr)
			}
			continue
		}

		ip := strings.TrimSpace(string(body))
		if isValidIP(ip) {
			if verbose {
				fmt.Printf("   ✅ 從 %s 獲取到有效 IP: %s\n", url, ip)
			}
			return ip, nil
		}

		lastErr = fmt.Errorf("從 %s 獲取的 IP 無效: %s", url, ip)
		if verbose {
			fmt.Printf("   ❌ %v\n", lastErr)
		}
	}

	return "", fmt.Errorf("所有 IP 檢查服務都失敗: %w", lastErr)
}

// 檢查 IP 地址是否有效
func isValidIP(ip string) bool {
	if ip == "" {
		return false
	}

	// 簡單的 IPv4 驗證
	parts := strings.Split(ip, ".")
	if len(parts) != 4 {
		return false
	}

	for _, part := range parts {
		if len(part) == 0 || len(part) > 3 {
			return false
		}

		// 檢查是否為數字
		for _, char := range part {
			if char < '0' || char > '9' {
				return false
			}
		}

		// 檢查數字範圍
		num, err := strconv.Atoi(part)
		if err != nil || num < 0 || num > 255 {
			return false
		}

		// 檢查前導零（但允許 "0"）
		if len(part) > 1 && part[0] == '0' {
			return false
		}
	}

	return true
}

func (d *DDNSService) UpdateDNSRecords() error {
	now := time.Now()
	d.lastCheck = now
	d.nextCheck = now.Add(time.Duration(d.config.Global.CheckInterval) * time.Second)

	// 獲取當前公共 IP
	currentIP, err := d.GetCurrentIP()
	if err != nil {
		return fmt.Errorf("獲取當前 IP 失敗: %w", err)
	}

	// 檢查 IP 是否發生變化
	ipChanged := d.currentIP != currentIP
	if ipChanged {
		fmt.Printf("🌐 檢測到 IP 變化: %s → %s\n", d.currentIP, currentIP)
		d.currentIP = currentIP

		// IP 變化時才需要更新 DNS 記錄
		return d.updateAllDNSRecords(currentIP)
	} else {
		// IP 未變化，只檢查 DNS 記錄同步狀態
		if verbose {
			fmt.Printf("💤 公共 IP 未變化: %s\n", currentIP)
		}
		return d.verifyDNSRecordsSync(currentIP)
	}
}

// 更新所有 DNS 記錄（只有 IP 變化時呼叫）
func (d *DDNSService) updateAllDNSRecords(newIP string) error {
	successCount := 0
	failureCount := 0
	updatedCount := 0

	for _, record := range d.config.DNSRecords {
		updated, err := d.updateSingleRecord(&record, newIP)
		if err != nil {
			failureCount++
			fmt.Printf("❌ 更新記錄 %s 失敗: %v\n", record.Name, err)
		} else {
			successCount++
			if updated {
				updatedCount++
			}
		}
	}

	// 顯示檢查結果和下次檢查時間
	d.printCheckResult(updatedCount, failureCount)

	// 儲存暫存資料
	d.saveIPCache()

	if failureCount > 0 {
		return fmt.Errorf("部分記錄更新失敗: %d 成功, %d 失敗", successCount, failureCount)
	}

	return nil
}

// 驗證 DNS 記錄是否同步（IP 未變化時呼叫）
func (d *DDNSService) verifyDNSRecordsSync(currentIP string) error {
	outOfSyncRecords := []string{}
	needsUpdate := false

	for _, record := range d.config.DNSRecords {
		// 檢查暫存中的 DNS IP 是否與當前 IP 一致
		cachedDNSIP, exists := d.dnsIPs[record.Name]
		if !exists || cachedDNSIP != currentIP {
			// 暫存資料不一致，需要實際檢查 Cloudflare
			actualDNSIP, err := d.cfClient.GetDNSRecordIP(record.Name, record.Type)
			if err != nil {
				fmt.Printf("⚠️  檢查記錄 %s 同步狀態失敗: %v\n", record.Name, err)
				continue
			}

			// 更新暫存
			d.dnsIPs[record.Name] = actualDNSIP

			// 檢查是否同步
			if actualDNSIP != currentIP {
				outOfSyncRecords = append(outOfSyncRecords, record.Name)
				needsUpdate = true
				if verbose {
					fmt.Printf("⚠️  記錄 %s 不同步: %s ≠ %s\n", record.Name, actualDNSIP, currentIP)
				}
			} else {
				// 實際檢查發現已同步，更新暫存
				if verbose {
					fmt.Printf("✅ 記錄 %s 已同步 (實際檢查)\n", record.Name)
				}
			}
		} else {
			// 暫存資料顯示已同步
			if verbose {
				fmt.Printf("✅ 記錄 %s 已同步 (暫存驗證)\n", record.Name)
			}
		}
	}

	// 如果有不同步的記錄，進行更新
	if needsUpdate {
		fmt.Printf("⚠️  發現 %d 個不同步的記錄，進行更新...\n", len(outOfSyncRecords))
		for _, recordName := range outOfSyncRecords {
			fmt.Printf("   - %s\n", recordName)
		}
		return d.updateOutOfSyncRecords(currentIP, outOfSyncRecords)
	}

	// 顯示檢查結果
	d.printCheckResult(0, 0) // 沒有更新，沒有失敗

	// 儲存暫存資料
	d.saveIPCache()

	return nil
}

// 更新不同步的記錄（只更新指定的記錄）
func (d *DDNSService) updateOutOfSyncRecords(newIP string, recordNames []string) error {
	successCount := 0
	failureCount := 0
	updatedCount := 0

	// 只更新不同步的記錄
	for _, recordName := range recordNames {
		// 找到對應的記錄配置
		var recordConfig *config.DNSRecord
		for _, record := range d.config.DNSRecords {
			if record.Name == recordName {
				recordConfig = &record
				break
			}
		}

		if recordConfig == nil {
			fmt.Printf("❌ 找不到記錄配置: %s\n", recordName)
			failureCount++
			continue
		}

		updated, err := d.updateSingleRecord(recordConfig, newIP)
		if err != nil {
			failureCount++
			fmt.Printf("❌ 更新記錄 %s 失敗: %v\n", recordName, err)
		} else {
			successCount++
			if updated {
				updatedCount++
			}
		}
	}

	// 顯示檢查結果和下次檢查時間
	d.printCheckResult(updatedCount, failureCount)

	// 儲存暫存資料
	d.saveIPCache()

	if failureCount > 0 {
		return fmt.Errorf("部分記錄更新失敗: %d 成功, %d 失敗", successCount, failureCount)
	}

	return nil
}

// 顯示檢查結果和下次檢查時間
func (d *DDNSService) printCheckResult(updatedCount, failureCount int) {
	now := time.Now()

	// 顯示基本結果
	if updatedCount > 0 {
		fmt.Printf("✅ 檢查完成: %d 個記錄已更新", updatedCount)
	} else if failureCount > 0 {
		fmt.Printf("⚠️  檢查完成: %d 個記錄更新失敗", failureCount)
	} else {
		fmt.Printf("✅ 檢查完成: 所有記錄已同步")
	}

	// 顯示下次檢查時間
	timeUntilNext := d.nextCheck.Sub(now)
	if timeUntilNext > 0 {
		fmt.Printf(" | 下次檢查: %s (%.0f秒後)\n",
			d.nextCheck.Format("15:04:05"),
			timeUntilNext.Seconds())
	} else {
		fmt.Printf(" | 下次檢查: %s\n", d.nextCheck.Format("15:04:05"))
	}
}

func (d *DDNSService) updateSingleRecord(record *config.DNSRecord, newIP string) (bool, error) {
	// 獲取記錄當前的 DNS IP
	currentDNSIP, err := d.cfClient.GetDNSRecordIP(record.Name, record.Type)
	if err != nil {
		errorMsg := fmt.Sprintf("獲取當前 DNS IP 失敗: %v", err)
		d.webhook.SendFailure(record.Name, errorMsg)
		return false, fmt.Errorf("獲取記錄 %s 的當前 IP 失敗: %w", record.Name, err)
	}

	// 檢查是否需要更新
	if currentDNSIP == newIP {
		if verbose {
			fmt.Printf("✅ 記錄 %s 已是最新 IP: %s\n", record.Name, newIP)
		}
		d.dnsIPs[record.Name] = newIP
		return false, nil // 已經是最新 IP，不需要更新
	}

	// DNS 記錄不同步，需要更新
	fmt.Printf("🔄 更新記錄 %s: %s → %s\n", record.Name, currentDNSIP, newIP)

	// 獲取記錄 ID
	recordID, err := d.cfClient.GetDNSRecordID(record.Name, record.Type)
	if err != nil {
		errorMsg := fmt.Sprintf("獲取記錄 ID 失敗: %v", err)
		d.webhook.SendFailure(record.Name, errorMsg)
		return false, fmt.Errorf("獲取記錄 ID 失敗 (%s): %w", record.Name, err)
	}

	// 更新記錄
	if err := d.cfClient.UpdateDNSRecord(recordID, record, newIP); err != nil {
		errorMsg := fmt.Sprintf("更新記錄失敗: %v", err)
		d.webhook.SendFailure(record.Name, errorMsg)
		return false, fmt.Errorf("更新 DNS 記錄失敗 (%s): %w", record.Name, err)
	}

	// 更新本地暫存
	d.dnsIPs[record.Name] = newIP
	d.webhook.SendSuccess(currentDNSIP, newIP, record.Name)
	fmt.Printf("✅ 成功更新記錄 %s → %s\n", record.Name, newIP)

	return true, nil
}

func (d *DDNSService) Start() error {
	// 等待網路連線
	waitForNetwork()

	ticker := time.NewTicker(time.Duration(d.config.Global.CheckInterval) * time.Second)
	defer ticker.Stop()

	fmt.Println("🚀 啟動 Cloudflare DDNS 服務...")
	fmt.Printf("⏰ 檢查間隔: %d 秒\n", d.config.Global.CheckInterval)
	fmt.Printf("📊 監控記錄數: %d\n", len(d.config.DNSRecords))
	fmt.Printf("🌐 IP 檢查服務: %d 個\n", len(d.config.Global.IPCheckURLs))
	fmt.Printf("💾 暫存檔案: %s\n", d.cacheFile)

	// 顯示暫存狀態
	if d.currentIP != "" {
		fmt.Printf("📁 載入暫存 IP: %s\n", d.currentIP)
	} else {
		fmt.Printf("📁 暫存 IP: 無\n")
	}

	// 初始化當前 IP（如果暫存中沒有）
	if d.currentIP == "" {
		fmt.Println("\n🔍 初始 IP 檢查...")
		initialIP, err := d.GetCurrentIP()
		if err != nil {
			fmt.Printf("❌ 初始 IP 獲取失敗: %v\n", err)
			// 不立即退出，繼續嘗試
		} else {
			d.currentIP = initialIP
			fmt.Printf("✅ 當前公共 IP: %s\n", initialIP)
			d.saveIPCache()
		}
	} else {
		fmt.Printf("✅ 當前公共 IP: %s (從暫存)\n", d.currentIP)
	}

	// 立即執行一次檢查
	fmt.Println("\n🔧 執行初始檢查...")
	if err := d.UpdateDNSRecords(); err != nil {
		fmt.Printf("❌ 初始檢查失敗: %v\n", err)
	} else {
		fmt.Printf("✅ 初始檢查完成\n")
	}

	fmt.Printf("\n🎯 服務啟動完成，開始監控...\n")

	checkCounter := 1

	for {
		select {
		case <-ticker.C:
			checkCounter++

			if verbose {
				fmt.Printf("\n--- 第 %d 次檢查 (%s) ---\n",
					checkCounter, time.Now().Format("15:04:05"))
			} else {
				fmt.Printf("\n[%s] ", time.Now().Format("15:04:05"))
			}

			// 檢查配置文件是否變更
			if changed, err := d.config.HasChanged(); err == nil && changed {
				fmt.Println("📁 檢測到配置文件變更，重新加載...")
				if err := d.config.Reload(); err != nil {
					fmt.Printf("❌ 重新加載配置文件失敗: %v\n", err)
				} else {
					// 只更新 Webhook 客戶端，不發送訊息
					d.webhook = webhook.NewClient(
						d.config.Webhook.URL,
						d.config.Webhook.ChatID,
						d.config.Webhook.Type,
						d.config.Webhook.Template,
						d.config.Webhook.Enabled,
						d.config.Webhook.OnSuccess,
						d.config.Webhook.OnFailure,
					)
					fmt.Printf("✅ 配置文件重新加載完成\n")
				}
			}

			// 執行 DNS 記錄更新檢查
			if err := d.UpdateDNSRecords(); err != nil {
				fmt.Printf("❌ 第 %d 次檢查失敗: %v\n", checkCounter, err)
			}

		case <-d.stopChan:
			fmt.Println("\n🛑 收到停止信號，正在停止 DDNS 服務...")
			d.webhook.SendInfo("DDNS 服務已停止")
			return nil
		}
	}
}

func (d *DDNSService) Stop() {
	fmt.Println("\n⏹️  正在停止服務...")
	d.stopChan <- true
}

// 獲取服務狀態信息
func (d *DDNSService) GetStatus() map[string]any {
	status := make(map[string]any)
	status["current_ip"] = d.currentIP
	status["dns_records"] = d.dnsIPs
	status["last_check"] = d.lastCheck.Format("2006-01-02 15:04:05")
	status["next_check"] = d.nextCheck.Format("2006-01-02 15:04:05")
	status["monitored_records"] = len(d.config.DNSRecords)
	status["check_interval"] = d.config.Global.CheckInterval
	status["cache_file"] = d.cacheFile

	// 計算剩餘時間
	timeUntilNext := time.Until(d.nextCheck)
	status["seconds_until_next"] = int(timeUntilNext.Seconds())

	return status
}

// 手動觸發立即檢查
func (d *DDNSService) ForceUpdate() error {
	fmt.Println("🔧 手動觸發立即檢查...")
	return d.UpdateDNSRecords()
}

// 檢查特定記錄的狀態
func (d *DDNSService) CheckRecordStatus(recordName string) (map[string]string, error) {
	result := make(map[string]string)

	// 查找記錄配置
	var recordConfig *config.DNSRecord
	for _, record := range d.config.DNSRecords {
		if record.Name == recordName {
			recordConfig = &record
			break
		}
	}

	if recordConfig == nil {
		return nil, fmt.Errorf("未找到記錄: %s", recordName)
	}

	// 獲取當前公共 IP
	currentIP, err := d.GetCurrentIP()
	if err != nil {
		return nil, err
	}
	result["current_ip"] = currentIP

	// 獲取 DNS 記錄 IP
	dnsIP, err := d.cfClient.GetDNSRecordIP(recordName, recordConfig.Type)
	if err != nil {
		return nil, err
	}
	result["dns_ip"] = dnsIP

	// 檢查同步狀態
	if currentIP == dnsIP {
		result["status"] = "同步"
		result["sync_status"] = "✅"
	} else {
		result["status"] = "不同步"
		result["sync_status"] = "⚠️"
	}

	result["record_name"] = recordName
	result["record_type"] = recordConfig.Type
	result["proxied"] = fmt.Sprintf("%v", recordConfig.Proxied)
	result["ttl"] = fmt.Sprintf("%d", recordConfig.TTL)
	result["next_check"] = d.nextCheck.Format("15:04:05")

	return result, nil
}
