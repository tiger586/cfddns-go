# Cloudflare DDNS Client [![Go version](https://img.shields.io/github/go-mod/go-version/tiger586/cfddns-go)](https://github.com/tiger586/cfddns-go/blob/main/go.mod)

用 Go 語言開發的 Cloudflare 動態 DNS 客戶端，具備自動偵測 IP、即時更新 DNS 紀錄，以及透過 Webhook 傳送通知的功能。

## 功能特性

- ✅ 自動檢測公共 IP 變化
- ✅ Cloudflare DNS 記錄自動更新（無需 Zone ID）
- ✅ 支持 Telegram、Discord 等 Webhook 通知
- ✅ 配置文件熱重載
- ✅ systemd 服務支持
- ✅ 環境變量優先配置（.env 檔案）
- ✅ TTL 自動/手動設定
- ✅ 詳細的狀態監控和日誌

## 專案結構

```text
cfddns/
│
├── cloudflare/            # Cloudflare API 客戶端
│   └── client.go
├── cmd/                   # CLI 命令
│   ├── root.go            # 根命令
│   ├── run.go             # 執行服務
│   ├── status.go          # 狀態查看
│   ├── validate.go        # 配置驗證
│   ├── test.go            # API 測試
│   ├── webhook.go         # Webhook 測試
│   ├── install.go         # 服務安裝
│   ├── uninstall.go       # 服務卸載
│   └── version.go         # 版本信息
├── config/                # 配置管理
│   └── config.go          
├── service/               # DDNS 服務核心
│   └── ddns.go            
├── webhook/               # Webhook 功能
│   └── webhook.go         
├── .env.example           # 環境變數範例檔案
├── config.yaml.example    # 主配置文件（範例）
├── go.mod                 # Go 套件管理
├── go.sum                 # 依賴驗證
├── LICENSE                # MIT License
├── main.go                # 主程式
└── README.md              # 專案說明檔

```

## 快速開始

### 1. 安裝依賴

```bash
go mod tidy
```
### 2. 配置檔案設置
複製範例檔案
```bash
cp config.yaml.example config.yaml
cp .env.example .env
```
編輯設定檔案 config.yaml
```yaml
global:
  check_interval: 600  # 檢查間隔(秒)
  ip_check_urls:       # 檢查 IP 的網站（可自行增加）
    - "https://api.ipify.org"
    - "https://icanhazip.com"
    - "https://ident.me"
    - "https://4.ipw.cn"

# Cloudflare 配置
cloudflare:
  api_token: "your-cloudflare-api-token-here"  # 可選：如果 .env 未設置則使用此值

# DNS 記錄配置，可以多筆紀錄
dns_records:
  - name: "www.example1.com"
    type: "A"
    proxied: false  # Proxy 狀態：打開小雲朵 true，關閉 false
    ttl: 1          # 1 = 自動 TTL，1 分鐘 = 60（秒數）
  - name: "www.example2.com"
    type: "A"
    proxied: false  # Proxy 狀態：打開小雲朵 true，關閉 false
    ttl: 1          # 1 = 自動 TTL，1 分鐘 = 60（秒數）
  
# Webhook 配置（可選）
webhook:
  enabled: true
  type: "telegram"
  url: "https://discord.com/api/webhooks/your-webhook-url"  # 可選
  chat_id: "your_chat_id_here"  # 可選
  template: "text"
  on_success: true
  on_failure: true
  template: "text"  # 改為 text, markdown, 或 html  
```

編輯環境變數 .env（推薦）系統優先使用
```env
# Cloudflare API 配置
CF_API_TOKEN=your_actual_cloudflare_api_token_here

# Telegram Webhook 配置
WEBHOOK_URL=https://api.telegram.org/botYOUR_BOT_TOKEN/sendMessage
WEBHOOK_CHAT_ID=your_actual_chat_id_here
```

### 3. 構建程序
```bash
go build -o cfddns
```

### 4. 測試配置
```bash
# 驗證配置，加上-v 可以顯示更多資訊
./cfddns validate

# 測試 Cloudflare API 連接，加上 -v 可以顯示更多資訊
./cfddns test

# 查看當前狀態，加上 -v 可以顯示更多資訊
./cfddns status

# 測試 Webhook 通知
./cfddns webhook --type success
```

### 5. 執行程式
```bash
./cfddns run
```

## 系統服務安裝
### 安裝為 systemd 服務
```bash
# 安裝服務
sudo ./cfddns install

# 啟動服務
sudo systemctl start cfddns

# 查看服務狀態
sudo systemctl status cfddns

# 查看服務日誌
sudo journalctl -u cfddns -f

# 設置為開機自動啟動
sudo systemctl enable cfddns
```

### 卸載服務
```bash
# 停止服務
sudo systemctl stop cfddns

# 卸載服務
sudo ./cfddns uninstall
```

## 命令列表
|命令|	功能|	示例|
|---|---|---|
|run|	運行 DDNS 服務 | ./cfddns run -v|
|status | 查看 DNS 記錄狀態 | ./cfddns status |
|validate |	驗證配置檔案 | ./cfddns validate |
|test |	測試 Cloudflare API |	./cfddns test |
|webhook |	測試 Webhook 通知 |	./cfddns webhook --type success |
|install |	安裝系統服務 |	sudo ./cfddns install |
|uninstall |	卸載系統服務 |	sudo ./cfddns uninstall |
|version |	顯示版本信息 |	./cfddns version |


## 配置詳解
### Cloudflare API Token 權限
### 創建 API Token 時需要以下權限：
權限  
選擇套用編輯或讀取權限至您使用此 Token 的帳戶或網站。
 - 區域 → DNS → 編輯

區域資源  
選擇要包含或排除的區域。
 - 包含 → 特定區域 → 選擇網域（例如：example.com）

### TTL 設定 
|TTL 值	| 說明 | 範例 |
|-------|-----|-----|
|1 |	自動 TTL | ttl: 1 |
|60 |	1 分鐘 | ttl: 60 |
|300 |	5 分鐘 |	ttl: 300 |
|3600 |	1 小時 |	ttl: 3600 |
|86400 |	24 小時 |	ttl: 86400 |

### Webhook 支持
Telegram: type: "telegram"

Discord: type: "generic"

自定義 Webhook: type: "generic"

## 配置優先權
- 最高優先權: .env 環境變數
- 次高優先權: config.yaml 配置文件
- 最低優先權: 程序默認值

### 範例配置流程
```bash
# 1. 創建基本配置
cp config.yaml.example config.yaml

# 2. 設置敏感資料（安全）
cp .env.example .env
nano .env

# 3. 驗證配置
./cfddns validate

# 輸出示例：
📋 配置來源:
   Cloudflare API Token: .env
   Webhook URL: .env
   Webhook Chat ID: .env
```

## 日誌範例
### 正常運行日誌
```text
🚀 啟動 Cloudflare DDNS 服務...
⏰ 檢查間隔: 300 秒
📊 監控記錄數: 2
🌐 IP 檢查服務: 3 個

[14:30:25] ✅ 檢查完成: 所有記錄已同步 | 下次檢查: 14:35:25 (300秒後)

[14:35:25] 🌐 檢測到 IP 變化: 192.168.1.100 → 203.0.113.50
🔄 更新記錄 home.example.com: 192.168.1.100 → 203.0.113.50
✅ 成功更新記錄 home.example.com → 203.0.113.50
✅ 檢查完成: 1 個記錄已更新 | 下次檢查: 14:40:25 (300秒後)
```

### 狀態檢查輸出
```text
🌐 DNS 記錄狀態檢查
==================================================
📡 當前公共 IP: 203.0.113.50
⏰ 檢查間隔: 300 秒
📊 監控記錄數: 2
🕒 最後檢查: 2024-01-15 14:35:25
⏱️ 下次檢查: 2024-01-15 14:40:25 (150秒後)

📋 設定的 DNS 記錄狀態:
名稱               類型  代理  TTL     DNS IP        狀態  同步
----              ----  ----  ---     -------       ----  ----
www.example.com   A     關閉  自動    203.0.113.50  存在  ✅
vpn.example.com   A     開啟  1時     203.0.113.50  存在  ✅

📊 摘要: ✅ 所有記錄已同步 (2/2)
```

## ✨ 使用 Docker compose 啟動
### 1. 建立資料夾 app，將檔案複製到 app 資料夾裡
```bash
mkdir app
cp config.yaml.example app/config.yaml
cp .env.example app/.env 
cp cfddns app/
```
### 2. 執行 Docker
```bash
docker compose up -d
```

## 故障排除
### 常見問題
1. API Token 錯誤  
./cfddns test  
檢查 Token 權限是否正確  

2. DNS 記錄不同步  
./cfddns status  
查看當前狀態和同步情況  

3. Webhook 通知失敗  
./cfddns webhook --type info  
測試 Webhook 連接  

4. 配置文件錯誤  
./cfddns validate  
驗證配置檔案語法  

## 調試模式
### 使用 -v 參數啟用詳細日誌：

```bash
./cfddns run -v
./cfddns status -v
./cfddns test -v
```

### 安全建議
1. 保護 .env 檔案

```bash
chmod 600 .env
```

2. 不要提交敏感資料

```bash
# 在 .gitignore 中添加
echo ".env" >> .gitignore
echo "config.local.yaml" >> .gitignore
```

3. 定期輪換 API Token

每 3-6 個月更新一次 Cloudflare API Token

## 許可證
本項目採用 MIT 許可證 - 詳見 LICENSE 檔案。

