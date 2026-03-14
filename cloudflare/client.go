package cloudflare

import (
	"bytes"
	"cfddns/config"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type CloudflareClient struct {
	apiToken string
	client   *http.Client
}

type Zone struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Status string `json:"status"`
}

type ZoneResponse struct {
	Result  []Zone     `json:"result"`
	Success bool       `json:"success"`
	Errors  []APIError `json:"errors"`
}

type DNSRecordResponse struct {
	Result  []DNSRecord `json:"result"`
	Success bool        `json:"success"`
	Errors  []APIError  `json:"errors"`
}

type DNSRecord struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Type     string `json:"type"`
	Content  string `json:"content"`
	Proxied  bool   `json:"proxied"`
	TTL      int    `json:"ttl"`
	ZoneID   string `json:"zone_id"`
	ZoneName string `json:"zone_name,omitempty"`
}

type UpdateRecordRequest struct {
	Type    string `json:"type"`
	Name    string `json:"name"`
	Content string `json:"content"`
	Proxied bool   `json:"proxied"`
	TTL     int    `json:"ttl"`
}

type APIResponse struct {
	Success bool       `json:"success"`
	Errors  []APIError `json:"errors"`
}

type APIError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type TokenVerifyResponse struct {
	Success bool `json:"success"`
	Result  struct {
		ID     string `json:"id"`
		Status string `json:"status"`
		Email  string `json:"email"`
	} `json:"result"`
	Errors []APIError `json:"errors"`
}

var verbose bool

func NewClient(cfg *config.CloudflareConfig) *CloudflareClient {
	return &CloudflareClient{
		apiToken: strings.TrimSpace(cfg.APIToken),
		client:   &http.Client{Timeout: 30 * time.Second},
	}
}

func SetVerbose(v bool) {
	verbose = v
}

// 自動發現 Zone ID
func (c *CloudflareClient) AutoDiscoverZoneID(dnsRecordName string) (string, error) {
	// 從 DNS 記錄名稱中提取域名
	targetZoneName := extractZoneNameFromDNS(dnsRecordName)
	if targetZoneName == "" {
		return "", fmt.Errorf("無法從記錄名稱中提取域名: %s", dnsRecordName)
	}

	if verbose {
		fmt.Printf("🔍 自動發現 Zone ID for: %s\n", targetZoneName)
	}

	zones, err := c.GetZones()
	if err != nil {
		return "", fmt.Errorf("獲取區域列錶失敗: %w", err)
	}

	// 查找匹配的域名
	var matchedZone *Zone
	for _, zone := range zones {
		if strings.EqualFold(zone.Name, targetZoneName) {
			matchedZone = &zone
			break
		}
	}

	if matchedZone == nil {
		// 嘗試部分匹配
		for _, zone := range zones {
			if strings.HasSuffix(strings.ToLower(dnsRecordName), strings.ToLower("."+zone.Name)) {
				matchedZone = &zone
				break
			}
		}
	}

	if matchedZone == nil {
		return "", fmt.Errorf("未找到域名 %s 對應的區域，可用區域: %v",
			targetZoneName, getZoneNames(zones))
	}

	if verbose {
		fmt.Printf("✅ 發現 Zone ID: %s for %s\n", matchedZone.ID, matchedZone.Name)
	}

	return matchedZone.ID, nil
}

// 獲取用戶可訪問的所有區域
func (c *CloudflareClient) GetZones() ([]Zone, error) {
	url := "https://api.cloudflare.com/client/v4/zones?per_page=1000"

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("創建請求失敗: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.apiToken)
	req.Header.Set("Content-Type", "application/json")

	if verbose {
		fmt.Printf("🔍 獲取區域列錶...\n")
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("網絡請求失敗: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("讀取響應失敗: %w", err)
	}

	if verbose && resp.StatusCode != 200 {
		fmt.Printf("⚠️  狀態碼: %d, 響應: %s\n", resp.StatusCode, string(body))
	}

	var result ZoneResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("解析 JSON 失敗: %w", err)
	}

	if !result.Success {
		if len(result.Errors) > 0 {
			return nil, fmt.Errorf("API 錯誤: %v", result.Errors)
		}
		return nil, fmt.Errorf("API 調用失敗，狀態碼: %d", resp.StatusCode)
	}

	return result.Result, nil
}

// 測試 API Token
func (c *CloudflareClient) TestConnection() error {
	url := "https://api.cloudflare.com/client/v4/user/tokens/verify"

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("創建請求失敗: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.apiToken)
	req.Header.Set("Content-Type", "application/json")

	fmt.Printf("🧪 測試 Cloudflare API Token...\n")
	fmt.Printf("   Token: %s...\n", maskString(c.apiToken, 10))

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("網絡請求失敗: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("讀取響應失敗: %w", err)
	}

	fmt.Printf("   狀態碼: %d\n", resp.StatusCode)

	var result TokenVerifyResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("解析 JSON 失敗: %w", err)
	}

	if !result.Success {
		if len(result.Errors) > 0 {
			return fmt.Errorf("Token 驗證失敗: %v", result.Errors)
		}
		return fmt.Errorf("Token 驗證失敗")
	}

	fmt.Printf("✅ Token 驗證成功!\n")
	fmt.Printf("   用戶 ID: %s\n", result.Result.ID)
	fmt.Printf("   用戶郵箱: %s\n", result.Result.Email)
	fmt.Printf("   狀態: %s\n", result.Result.Status)

	// 獲取區域列錶來顯示權限
	zones, err := c.GetZones()
	if err != nil {
		fmt.Printf("⚠️  獲取區域列錶失敗: %v\n", err)
	} else {
		fmt.Printf("   可訪問區域: %d 個\n", len(zones))
		for i, zone := range zones {
			if i < 5 {
				fmt.Printf("     - %s (%s)\n", zone.Name, zone.Status)
			}
		}
		if len(zones) > 5 {
			fmt.Printf("     ... 和 %d 個其他區域\n", len(zones)-5)
		}
	}

	return nil
}

// 獲取特定 DNS 記錄
func (c *CloudflareClient) GetDNSRecord(recordName, recordType string) (*DNSRecord, error) {
	zoneID, err := c.AutoDiscoverZoneID(recordName)
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("https://api.cloudflare.com/client/v4/zones/%s/dns_records?type=%s&name=%s",
		zoneID, recordType, recordName)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("創建請求失敗: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.apiToken)
	req.Header.Set("Content-Type", "application/json")

	if verbose {
		fmt.Printf("🔍 查找記錄: %s %s\n", recordName, recordType)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("網絡請求失敗: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("讀取響應失敗: %w", err)
	}

	var result DNSRecordResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("解析 JSON 失敗: %w", err)
	}

	if !result.Success {
		if len(result.Errors) > 0 {
			return nil, fmt.Errorf("API 錯誤: %v", result.Errors)
		}
		return nil, fmt.Errorf("API 調用失敗")
	}

	if len(result.Result) == 0 {
		return nil, fmt.Errorf("未找到DNS記錄: %s", recordName)
	}

	return &result.Result[0], nil
}

// 獲取 DNS 記錄的當前 IP
func (c *CloudflareClient) GetDNSRecordIP(recordName, recordType string) (string, error) {
	record, err := c.GetDNSRecord(recordName, recordType)
	if err != nil {
		return "", err
	}
	return record.Content, nil
}

// 獲取 DNS 記錄 ID
func (c *CloudflareClient) GetDNSRecordID(recordName, recordType string) (string, error) {
	record, err := c.GetDNSRecord(recordName, recordType)
	if err != nil {
		return "", err
	}
	return record.ID, nil
}

// 更新 DNS 記錄
func (c *CloudflareClient) UpdateDNSRecord(recordID string, record *config.DNSRecord, ip string) error {
	zoneID, err := c.AutoDiscoverZoneID(record.Name)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("https://api.cloudflare.com/client/v4/zones/%s/dns_records/%s",
		zoneID, recordID)

	// 根據 TTL 值設置正確的 API 參數
	ttl := record.TTL
	if ttl == 1 {
		// TTL=1 表示自動
		ttl = 1
	} else if ttl < 60 {
		// 如果設置了小於 60 的值，強制設為 60（Cloudflare 最小值）
		ttl = 60
	} else if ttl > 86400 {
		// 如果設置了大於 86400 的值，強制設為 86400（Cloudflare 最大值）
		ttl = 86400
	}

	updateReq := UpdateRecordRequest{
		Type:    record.Type,
		Name:    record.Name,
		Content: ip,
		Proxied: record.Proxied,
		TTL:     ttl,
	}

	jsonData, err := json.Marshal(updateReq)
	if err != nil {
		return fmt.Errorf("序列化請求失敗: %w", err)
	}

	req, err := http.NewRequest("PUT", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("創建請求失敗: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.apiToken)
	req.Header.Set("Content-Type", "application/json")

	if verbose {
		ttlDescription := "自動"
		if ttl > 1 {
			ttlDescription = fmt.Sprintf("%d 秒", ttl)
		}
		fmt.Printf("🔧 更新記錄: %s -> %s (TTL: %s)\n", record.Name, ip, ttlDescription)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("網絡請求失敗: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("讀取響應失敗: %w", err)
	}

	var result APIResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("解析 JSON 失敗: %w", err)
	}

	if !result.Success {
		if len(result.Errors) > 0 {
			return fmt.Errorf("Cloudflare API錯誤: %s", result.Errors[0].Message)
		}
		return fmt.Errorf("Cloudflare API調用失敗")
	}

	return nil
}

// 輔助函數
func extractZoneNameFromDNS(dnsName string) string {
	parts := strings.Split(dnsName, ".")
	if len(parts) >= 2 {
		return parts[len(parts)-2] + "." + parts[len(parts)-1]
	}
	return dnsName
}

func getZoneNames(zones []Zone) []string {
	var names []string
	for _, zone := range zones {
		names = append(names, zone.Name)
	}
	return names
}

func maskString(s string, showLen int) string {
	if len(s) <= showLen {
		return "***"
	}
	return s[:min(showLen, len(s))] + "..."
}
