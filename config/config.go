package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type GlobalConfig struct {
	CheckInterval int      `yaml:"check_interval"`
	IPCheckURLs   []string `yaml:"ip_check_urls"`
}

type CloudflareConfig struct {
	APIToken string `yaml:"api_token"`
}

type DNSRecord struct {
	Name    string `yaml:"name"`
	Type    string `yaml:"type"`
	Proxied bool   `yaml:"proxied"`
	TTL     int    `yaml:"ttl"`
}

type WebhookConfig struct {
	Enabled   bool   `yaml:"enabled"`
	Type      string `yaml:"type"`
	URL       string `yaml:"url"`
	ChatID    string `yaml:"chat_id"`
	Template  string `yaml:"template"`
	OnSuccess bool   `yaml:"on_success"`
	OnFailure bool   `yaml:"on_failure"`
}

type Config struct {
	Global       GlobalConfig     `yaml:"global"`
	Cloudflare   CloudflareConfig `yaml:"cloudflare"`
	DNSRecords   []DNSRecord      `yaml:"dns_records"`
	Webhook      WebhookConfig    `yaml:"webhook"`
	ConfigPath   string           `yaml:"-"`
	LastModified time.Time        `yaml:"-"`
}

func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("讀取配置文件失敗: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("解析配置文件失敗: %w", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("獲取文件信息失敗: %w", err)
	}

	config.ConfigPath = path
	config.LastModified = info.ModTime()

	// 設置默認值
	if config.Global.CheckInterval == 0 {
		config.Global.CheckInterval = 300
	}
	if len(config.Global.IPCheckURLs) == 0 {
		config.Global.IPCheckURLs = []string{
			"https://api.ipify.org",
			"https://icanhazip.com",
			"https://ident.me",
			"https://4.ipw.cn",
		}
	}
	if config.Webhook.Template == "" {
		config.Webhook.Template = "text"
	}

	// 從環境變量加載敏感資料（.env 優先）
	config.loadFromEnv()

	return &config, nil
}

// 從環境變量加載敏感資料（.env 優先）
func (c *Config) loadFromEnv() {
	// 加載 .env 檔案（如果存在）
	c.loadDotEnv()

	// Cloudflare API Token: .env 優先，如果未設置則使用 config.yaml
	if envToken := os.Getenv("CF_API_TOKEN"); envToken != "" {
		c.Cloudflare.APIToken = envToken
	}

	// Webhook URL: .env 優先，如果未設置則使用 config.yaml
	if envURL := os.Getenv("WEBHOOK_URL"); envURL != "" {
		c.Webhook.URL = envURL
	}

	// Webhook Chat ID: .env 優先，如果未設置則使用 config.yaml
	if envChatID := os.Getenv("WEBHOOK_CHAT_ID"); envChatID != "" {
		c.Webhook.ChatID = envChatID
	}

	// 如果配置了 Webhook URL，自動啟用 Webhook（除非明確設置為 false）
	if c.Webhook.URL != "" && !c.Webhook.Enabled {
		// 隻有在 config.yaml 中沒有明確設置 enabled 時才自動啟用
		// 這裡我們檢查 URL 是否來自 .env，如果是則自動啟用
		if os.Getenv("WEBHOOK_URL") != "" {
			c.Webhook.Enabled = true
		}
	}
}

// 加載 .env 檔案
func (c *Config) loadDotEnv() {
	envPath := ".env"
	if _, err := os.Stat(envPath); err != nil {
		// .env 檔案不存在，跳過
		return
	}

	data, err := os.ReadFile(envPath)
	if err != nil {
		fmt.Printf("⚠️  讀取 .env 檔案失敗: %v\n", err)
		return
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])

			// 移除值的引號（如果有的話）
			if len(value) > 1 && ((value[0] == '"' && value[len(value)-1] == '"') ||
				(value[0] == '\'' && value[len(value)-1] == '\'')) {
				value = value[1 : len(value)-1]
			}

			os.Setenv(key, value)
		}
	}
}

// 獲取配置來源信息（用於調試）
func (c *Config) GetConfigSource() map[string]string {
	source := make(map[string]string)

	// Cloudflare API Token 來源
	if os.Getenv("CF_API_TOKEN") != "" {
		source["cloudflare.api_token"] = ".env"
	} else if c.Cloudflare.APIToken != "" {
		source["cloudflare.api_token"] = "config.yaml"
	} else {
		source["cloudflare.api_token"] = "未設置"
	}

	// Webhook URL 來源
	if os.Getenv("WEBHOOK_URL") != "" {
		source["webhook.url"] = ".env"
	} else if c.Webhook.URL != "" {
		source["webhook.url"] = "config.yaml"
	} else {
		source["webhook.url"] = "未設置"
	}

	// Webhook Chat ID 來源
	if os.Getenv("WEBHOOK_CHAT_ID") != "" {
		source["webhook.chat_id"] = ".env"
	} else if c.Webhook.ChatID != "" {
		source["webhook.chat_id"] = "config.yaml"
	} else {
		source["webhook.chat_id"] = "未設置"
	}

	return source
}

// 驗證配置是否完整
func (c *Config) Validate() error {
	msg := ""
	if c.Cloudflare.APIToken == "" {
		// return fmt.Errorf("Cloudflare API Token 未設置")
		msg += "   Cloudflare API Token 未設置\n"
	}

	if len(c.DNSRecords) == 0 {
		// return fmt.Errorf("未配置任何 DNS 記錄")
		msg += "   未配置任何 DNS 記錄\n"
	}

	// 檢查 DNS 記錄的 TTL 設置
	for _, record := range c.DNSRecords {
		if record.TTL != 1 && (record.TTL < 60 || record.TTL > 86400) {
			// return fmt.Errorf("記錄 %s 的 TTL 值無效: %d (必須為 1=自動 或 60-86400 秒)", record.Name, record.TTL)
			msg += fmt.Sprintf("   記錄 %s 的 TTL 值無效: %d (必須為 1=自動 或 60-86400 秒)\n", record.Name, record.TTL)
		}
	}

	// 檢查 Webhook 配置
	if c.Webhook.Enabled {
		if c.Webhook.URL == "" {
			// return fmt.Errorf("Webhook 已啟用但未設置 URL")
			msg += "   Webhook 已啟用但未設置 URL\n"
		}
		if c.Webhook.Type == "telegram" && c.Webhook.ChatID == "" {
			// return fmt.Errorf("Telegram Webhook 需要設置 Chat ID")
			msg += "   Telegram Webhook 需要設置 Chat ID\n"
		}
	}

	if msg != "" {
		return fmt.Errorf(msg)
	} else {
		return nil
	}
}

func (c *Config) HasChanged() (bool, error) {
	info, err := os.Stat(c.ConfigPath)
	if err != nil {
		return false, err
	}
	return info.ModTime().After(c.LastModified), nil
}

func (c *Config) Reload() error {
	newConfig, err := LoadConfig(c.ConfigPath)
	if err != nil {
		return err
	}
	*c = *newConfig
	return nil
}

func GetDefaultConfigPath() string {
	if _, err := os.Stat("config.yaml"); err == nil {
		return "config.yaml"
	}

	if _, err := os.Stat("/etc/cfddns/config.yaml"); err == nil {
		return "/etc/cfddns/config.yaml"
	}

	return "config.yaml"
}

// 從 DNS 記錄名稱中提取主域名
func ExtractZoneNameFromDNS(dnsName string) string {
	parts := strings.Split(dnsName, ".")
	if len(parts) >= 2 {
		return parts[len(parts)-2] + "." + parts[len(parts)-1]
	}
	return dnsName
}
