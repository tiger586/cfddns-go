package webhook

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type WebhookMessage struct {
	Title      string    `json:"title"`
	Message    string    `json:"message"`
	Timestamp  time.Time `json:"timestamp"`
	Level      string    `json:"level"`
	IPAddress  string    `json:"ip_address,omitempty"`
	RecordName string    `json:"record_name,omitempty"`
}

type TelegramMessage struct {
	ChatID    string `json:"chat_id"`
	Text      string `json:"text"`
	ParseMode string `json:"parse_mode,omitempty"` // 空值錶示 text, 或 Markdown, HTML
}

type WebhookClient struct {
	url       string
	chatID    string
	enabled   bool
	hookType  string // generic 或 telegram
	template  string // text, markdown 或 html
	onSuccess bool
	onFailure bool
	client    *http.Client
}

func NewClient(url, chatID, hookType, template string, enabled, onSuccess, onFailure bool) *WebhookClient {
	return &WebhookClient{
		url:       url,
		chatID:    chatID,
		enabled:   enabled,
		hookType:  hookType,
		template:  template,
		onSuccess: onSuccess,
		onFailure: onFailure,
		client:    &http.Client{Timeout: 10 * time.Second},
	}
}

func (w *WebhookClient) SendSuccess(DNSip, ip, recordName string) error {
	if !w.enabled || !w.onSuccess {
		return nil
	}

	title := "✅ DDNS 更新成功"
	// message := fmt.Sprintf("DNS 記錄 %s 已成功更新", recordName)
	// details := fmt.Sprintf("新 IP 地址: %s\n記錄名稱: %s\n時間: %s",
	// 	 ip, recordName, time.Now().Format("2006-01-02 15:04:05"))
	message := fmt.Sprintf("DNS 記錄 %s 發生變化", recordName)
	// details := fmt.Sprintf("原 IP 地址: %s \n新 IP 地址: %s\n時間: %s",
	details := fmt.Sprintf("%s → %s\n時間: %s",
		DNSip, ip, time.Now().Format("2006-01-02 15:04:05"))

	return w.sendMessage(title, message, details, "success")
}

func (w *WebhookClient) SendFailure(recordName, errorMsg string) error {
	if !w.enabled || !w.onFailure {
		return nil
	}

	title := "❌ DDNS 更新失敗"
	message := fmt.Sprintf("更新 DNS 記錄 %s 時發生錯誤", recordName)
	details := fmt.Sprintf("記錄名稱: %s\n錯誤信息: %s\n時間: %s",
		recordName, errorMsg, time.Now().Format("2006-01-02 15:04:05"))

	return w.sendMessage(title, message, details, "error")
}

func (w *WebhookClient) SendInfo(customMessage string) error {
	if !w.enabled {
		return nil
	}

	title := "ℹ️ DDNS 信息"
	message := customMessage
	details := fmt.Sprintf("時間: %s", time.Now().Format("2006-01-02 15:04:05"))

	return w.sendMessage(title, message, details, "info")
}

func (w *WebhookClient) SendCustom(title, message, level string) error {
	if !w.enabled {
		return nil
	}

	details := fmt.Sprintf("時間: %s", time.Now().Format("2006-01-02 15:04:05"))
	return w.sendMessage(title, message, details, level)
}

func (w *WebhookClient) SendTest() error {
	if !w.enabled {
		return nil
	}

	title := "🧪 DDNS 測試通知"
	message := "這是一條測試訊息，用於驗證 Webhook 配置是否正確"
	details := fmt.Sprintf("服務: Cloudflare DDNS\n類型: %s\n時間: %s",
		w.hookType, time.Now().Format("2006-01-02 15:04:05"))

	return w.sendMessage(title, message, details, "info")
}

func (w *WebhookClient) sendMessage(title, message, details, level string) error {
	switch w.hookType {
	case "telegram":
		return w.sendTelegramMessage(title, message, details, level)
	default:
		return w.sendGenericMessage(title, message, details, level)
	}
}

func (w *WebhookClient) sendGenericMessage(title, message, details, level string) error {
	webhookMsg := WebhookMessage{
		Title:     title,
		Message:   message + "\n" + details,
		Timestamp: time.Now(),
		Level:     level,
	}

	jsonData, err := json.Marshal(webhookMsg)
	if err != nil {
		return err
	}

	resp, err := w.client.Post(w.url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("webhook 調用失敗，狀態碼: %d", resp.StatusCode)
	}

	return nil
}

func (w *WebhookClient) sendTelegramMessage(title, message, details, level string) error {
	// 根據模闆類型構建消息內容
	var text string
	var parseMode string

	switch w.template {
	case "html":
		parseMode = "HTML"
		text = fmt.Sprintf("<b>%s</b>\n%s\n\n<pre>%s</pre>",
			escapeHTML(title), escapeHTML(message), escapeHTML(details))
	case "markdown", "markdownv2":
		parseMode = "MarkdownV2"
		text = fmt.Sprintf("*%s*\n%s\n\n```\n%s\n```",
			escapeMarkdown(title), escapeMarkdown(message), escapeMarkdown(details))
	default: // text 或未知類型
		parseMode = "" // 空值錶示純文本
		text = fmt.Sprintf("%s\n%s\n\n%s", title, message, details)
	}

	tgMessage := TelegramMessage{
		ChatID:    w.chatID,
		Text:      text,
		ParseMode: parseMode, // 如果是空字符串，Telegram 會當作純文本處理
	}

	jsonData, err := json.Marshal(tgMessage)
	if err != nil {
		return err
	}

	resp, err := w.client.Post(w.url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		// 讀取錯誤響應以獲得更多信息
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Telegram API 調用失敗，狀態碼: %d, 響應: %s", resp.StatusCode, string(body))
	}

	return nil
}

// 純文本不需要轉義，但為了安全起見還是保留
func escapeText(text string) string {
	// 純文本情況下，隻需要處理可能破壞格式的字符
	return strings.ReplaceAll(text, "```", "'''")
}

func escapeMarkdown(text string) string {
	chars := []string{"_", "*", "[", "]", "(", ")", "~", "`", ">", "#", "+", "-", "=", "|", "{", "}", ".", "!"}
	for _, char := range chars {
		text = strings.ReplaceAll(text, char, "\\"+char)
	}
	return text
}

func escapeHTML(text string) string {
	text = strings.ReplaceAll(text, "&", "&amp;")
	text = strings.ReplaceAll(text, "<", "&lt;")
	text = strings.ReplaceAll(text, ">", "&gt;")
	return text
}
