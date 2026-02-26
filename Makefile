# Cloudflare DDNS Makefile

.PHONY: build clean deps cross-build package release

# 版本信息
VERSION ?= $(shell git describe --tags --abbrev=0 2>/dev/null || echo "v1.0.0")
BUILD_TIME = $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

# 構建目標
build:
	@echo "🔨 構建 Cloudflare DDNS..."
	go mod tidy
	go build -o cfddns \
		-ldflags="-s -w -X 'main.Version=$(VERSION)' -X 'main.BuildTime=$(BUILD_TIME)'" \
		.

# 清理
clean:
	@echo "🧹 清理構建文件..."
	rm -f cfddns cfddns.exe
	rm -rf release/ dist/

# 安裝依賴
deps:
	@echo "📦 安裝依賴..."
	go mod tidy

# 跨平台構建
cross-build:
	@echo "🌍 跨平台構建..."
	mkdir -p ./release/cfddns-linux-amd64
	mkdir -p ./release/cfddns-linux-arm64
	mkdir -p ./release/cfddns-darwin-amd64
	mkdir -p ./release/cfddns-darwin-arm64

	# Linux x86-64
	GOOS=linux GOARCH=amd64 go build -o release/cfddns-linux-amd64/cfddns \
		-ldflags="-s -w -X 'main.Version=$(VERSION)' -X 'main.BuildTime=$(BUILD_TIME)'" .
	
	# Linux ARM
	GOOS=linux GOARCH=arm64 go build -o release/cfddns-linux-arm64/cfddns \
		-ldflags="-s -w -X 'main.Version=$(VERSION)' -X 'main.BuildTime=$(BUILD_TIME)'" .
	
	# macOS intel
	GOOS=darwin GOARCH=amd64 go build -o release/cfddns-darwin-amd64/cfddns \
		-ldflags="-s -w -X 'main.Version=$(VERSION)' -X 'main.BuildTime=$(BUILD_TIME)'" .
	
	# macOS Mx
	GOOS=darwin GOARCH=arm64 go build -o release/cfddns-darwin-arm64/cfddns \
		-ldflags="-s -w -X 'main.Version=$(VERSION)' -X 'main.BuildTime=$(BUILD_TIME)'" .
	
	# 複製配置文件
	echo release/cfddns-linux-amd64/ release/cfddns-linux-arm64 release/cfddns-darwin-amd64 release/cfddns-darwin-arm64 | xargs -n 1 cp -v README.md
	echo release/cfddns-linux-amd64/ release/cfddns-linux-arm64 release/cfddns-darwin-amd64 release/cfddns-darwin-arm64 | xargs -n 1 cp -v config.yaml.example
	echo release/cfddns-linux-amd64/ release/cfddns-linux-arm64 release/cfddns-darwin-amd64 release/cfddns-darwin-arm64 | xargs -n 1 cp -v .env.example
	echo release/cfddns-linux-amd64/ release/cfddns-linux-arm64 release/cfddns-darwin-amd64 release/cfddns-darwin-arm64 | xargs -n 1 cp -v docker-compose.yml 

# 創建本地發布包 
package: 
	@echo "📦 創建發布包..."
	cd release && \
	tar czf cfddns-$(VERSION)-linux-amd64.tar.gz cfddns-linux-amd64 && \
	tar czf cfddns-$(VERSION)-linux-arm64.tar.gz cfddns-linux-arm64  && \
	tar czf cfddns-$(VERSION)-darwin-amd64.tar.gz cfddns-darwin-amd64 && \
	tar czf cfddns-$(VERSION)-darwin-arm64.tar.gz cfddns-darwin-arm64 

# 準備發布
release: clean cross-build package
	@echo "✅ 發布準備完成!"
	@echo "📁 發布文件在 release/ 目錄"
	@ls -la release/

# 幫助
help:
	@echo "Cloudflare DDNS 構建命令:"
	@echo "  deps            - 安裝依賴"
	@echo "  build           - 構建當前平台二進制文件"
	@echo "  clean           - 清理構建文件"
	@echo "  cross-build     - 跨平台構建"
	@echo "  package         - 創建發布包"
	@echo "  release         - 準備發布（清理+構建+打包）"
	@echo ""
	@echo "使用示例:"
	@echo "  make build"
	@echo "  make release"
	