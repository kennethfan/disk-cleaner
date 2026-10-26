# disk-cleaner — macOS 磁盘空间清理工具
#
# 说明：本机 Go 模块缓存位于 ~/.gvm，若在受限沙箱中构建需提升文件权限。

BINARY  := disk-cleaner
BUILD   := bin
PKG     := ./cmd/disk-cleaner
APP     := dist/磁盘清理.app

.PHONY: all build run test cover vet fmt fmt-check check clean scan dry gui app icon

all: check build

## build: 编译二进制到 bin/
build:
	@mkdir -p $(BUILD)
	go build -trimpath -ldflags "-s -w" -o $(BUILD)/$(BINARY) $(PKG)
	@echo "已生成 $(BUILD)/$(BINARY)"

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

## app: 打包为可双击的 .app
#
# 未签名的 .app 会被 macOS Gatekeeper 直接拒绝（spctl 返回
# "rejected / no usable signature"），表现为双击无反应。
# 这里用 ad-hoc 签名（-s -）解决本地运行问题；
# 对外分发仍需 Apple 开发者证书 + notarytool 公证。
app: build
	@rm -rf "$(APP)"
	@mkdir -p "$(APP)/Contents/MacOS" "$(APP)/Contents/Resources"
	@cp $(BUILD)/$(BINARY) "$(APP)/Contents/MacOS/$(BINARY)"
	@sed 's/@VERSION@/0.3.0/g' packaging/Info.plist > "$(APP)/Contents/Info.plist"
	@codesign --force --deep --sign - "$(APP)" 2>&1 | grep -v 'replacing existing signature' || true
	@echo "已生成 $(APP)（ad-hoc 签名）"
	@echo "运行: open \"$(APP)\""

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

## clean: 清理构建产物
clean:
	rm -rf $(BUILD) dist

## help: 显示可用目标
help:
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/## /  /'
