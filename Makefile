# disk-cleaner — macOS 磁盘空间清理工具
#
# 说明：本机 Go 模块缓存位于 ~/.gvm，若在受限沙箱中构建需提升文件权限。

BINARY      := disk-cleaner
BINARY_WEB  := disk-cleaner-web
BUILD       := bin
PKG         := ./cmd/disk-cleaner
PKG_WEB     := ./cmd/disk-cleaner-web
APP         := dist/磁盘清理.app
VERSION := 0.4.0

# WebView 壳的绑定输出目录（由 wails3 生成，见 bindings 目标）
WEBUI_DIR    := internal/webui
BINDINGS_DIR := $(WEBUI_DIR)/assets/bindings

# 正式签名用的开发者身份。留空则使用 ad-hoc 签名。
# 用法：make app SIGN_IDENTITY="Developer ID Application: Your Name (TEAMID)"
SIGN_IDENTITY ?=

# 公证用的凭据 profile 名（由 make notary-setup 创建）
NOTARY_PROFILE ?= disk-cleaner-notary

.PHONY: all build build-web run-web bindings test cover vet fmt fmt-check check clean scan dry gui \
        app app-unsigned icon notary-setup notarize verify-sign help

all: check build

## build: 编译两个二进制到 bin/（Fyne 版 + WebView 版）
build:
	@mkdir -p $(BUILD)
	go build -trimpath -ldflags "-s -w" -o $(BUILD)/$(BINARY) $(PKG)
	go build -trimpath -ldflags "-s -w" -o $(BUILD)/$(BINARY_WEB) $(PKG_WEB)
	@echo "已生成 $(BUILD)/$(BINARY) 与 $(BUILD)/$(BINARY_WEB)"

## build-web: 再生 bindings 后编译 WebView 壳（依赖 wails3 CLI）
#
# 绑定文件是从 Service 方法生成的接口层，改过 internal/webui/service.go
# 后必须再生，否则前端调到旧接口（不再生构建也不会发现）。
build-web: bindings
	@mkdir -p $(BUILD)
	go build -trimpath -ldflags "-s -w" -o $(BUILD)/$(BINARY_WEB) $(PKG_WEB)
	@echo "已生成 $(BUILD)/$(BINARY_WEB)"

## bindings: 重新生成 WebView 前端绑定（改 service.go 后需要；build-web 会自动调用）
bindings:
	cd $(WEBUI_DIR) && wails3 generate bindings -b -d assets/bindings -silent
	@echo "已再生 $(BINDINGS_DIR)"

## run-web: 编译并启动 WebView 图形界面（原型样式）
run-web: build-web
	./$(BUILD)/$(BINARY_WEB)

## gui: 编译并启动图形界面
gui: build
	./$(BUILD)/$(BINARY) gui

## run: 编译并运行 scan
run: build
	./$(BUILD)/$(BINARY) scan

## scan: 直接扫描（只读）
scan: build
	./$(BUILD)/$(BINARY) scan

## dry: 用户缓存的 dry-run 演示
dry: build
	./$(BUILD)/$(BINARY) clean --category 用户缓存 --dry-run

## icon: 从 packaging/icon-source.png 重新生成 AppIcon.icns
#
# 需要 Python3 + Pillow。图标规范要求圆角外透明，
# icon-source.png 已按此处理（圆角外 alpha=0）。
icon:
	@python3 packaging/make_icon.py

## app: 打包为可双击的 .app（含图标与签名）
#
# 未签名的 .app 会被 macOS Gatekeeper 拒绝，表现为双击无反应。
# 默认走 ad-hoc 签名，仅够本机自用；
# 传 SIGN_IDENTITY="Developer ID Application: ..." 则用正式证书签名。
app: build
	@rm -rf "$(APP)"
	@mkdir -p "$(APP)/Contents/MacOS" "$(APP)/Contents/Resources"
	@cp $(BUILD)/$(BINARY) "$(APP)/Contents/MacOS/$(BINARY)"
	@sed 's/@VERSION@/$(VERSION)/g' packaging/Info.plist > "$(APP)/Contents/Info.plist"
	@cp packaging/AppIcon.icns "$(APP)/Contents/Resources/AppIcon.icns"
	@if [ -n "$(SIGN_IDENTITY)" ]; then \
		echo "使用证书签名: $(SIGN_IDENTITY)"; \
		codesign --force --deep --options runtime --timestamp \
			--sign "$(SIGN_IDENTITY)" "$(APP)"; \
	else \
		echo "使用 ad-hoc 签名（仅本机自用）"; \
		codesign --force --deep --sign - "$(APP)" 2>&1 \
			| grep -v 'replacing existing signature' || true; \
	fi
	@echo "已生成 $(APP)  v$(VERSION)"
	@echo "运行: open \"$(APP)\""

## app-unsigned: 打包但不签名（用于排查签名相关问题）
app-unsigned: build
	@rm -rf "$(APP)"
	@mkdir -p "$(APP)/Contents/MacOS" "$(APP)/Contents/Resources"
	@cp $(BUILD)/$(BINARY) "$(APP)/Contents/MacOS/$(BINARY)"
	@sed 's/@VERSION@/$(VERSION)/g' packaging/Info.plist > "$(APP)/Contents/Info.plist"
	@cp packaging/AppIcon.icns "$(APP)/Contents/Resources/AppIcon.icns"
	@echo "已生成未签名的 $(APP)"

## verify-sign: 检查 .app 的签名与 Gatekeeper 状态
verify-sign:
	@echo "=== 签名信息 ==="
	@codesign -dv --verbose=2 "$(APP)" 2>&1 | head -12 || true
	@echo
	@echo "=== 签名有效性 ==="
	@codesign --verify --deep --strict --verbose=2 "$(APP)" 2>&1 || true
	@echo
	@echo "=== Gatekeeper 评估 ==="
	@spctl -a -vvv -t exec "$(APP)" 2>&1 || true
	@echo
	@echo "=== 公证票据 ==="
	@xcrun stapler validate "$(APP)" 2>&1 || true

## notary-setup: 把公证凭据存入钥匙串（只需执行一次）
#
# 需要：Apple ID、Team ID、App 专用密码
# （在 https://appleid.apple.com 生成 App 专用密码，不是账号密码）
notary-setup:
	@if [ -z "$(APPLE_ID)" ] || [ -z "$(TEAM_ID)" ]; then \
		echo "用法: make notary-setup APPLE_ID=you@example.com TEAM_ID=ABCDE12345"; \
		echo "随后按提示输入 App 专用密码"; \
		exit 1; \
	fi
	xcrun notarytool store-credentials "$(NOTARY_PROFILE)" \
		--apple-id "$(APPLE_ID)" \
		--team-id "$(TEAM_ID)" \
		--password "$(APP_PASSWORD)"

## notarize: 提交公证并装订票据（需先完成 notary-setup 与正式签名）
notarize:
	@echo "打包 ZIP 供公证..."
	@rm -f dist/notarize.zip
	@ditto -c -k --keepParent "$(APP)" dist/notarize.zip
	@echo "提交公证（可能需要几分钟）..."
	xcrun notarytool submit dist/notarize.zip \
		--keychain-profile "$(NOTARY_PROFILE)" --wait
	@echo "装订票据..."
	xcrun stapler staple "$(APP)"
	@rm -f dist/notarize.zip
	@echo "公证完成，可用 make verify-sign 复核"

## test: 运行全部单元测试
test:
	go test ./...

## cover: 输出各包覆盖率
cover:
	go test ./... -cover

## vet: 静态检查
vet:
	go vet ./...

## fmt: 格式化
fmt:
	gofmt -w .

## fmt-check: 检查格式是否合规
fmt-check:
	@out=$$(gofmt -l .); \
	if [ -n "$$out" ]; then echo "以下文件格式不合规:"; echo "$$out"; exit 1; fi
	@echo "格式检查通过"

## check: 格式 + 静态检查 + 测试
check: fmt-check vet test

## clean: 清理构建产物与生成的绑定
clean:
	rm -rf $(BUILD) dist $(BINDINGS_DIR)

## help: 显示可用目标
help:
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/## /  /'
