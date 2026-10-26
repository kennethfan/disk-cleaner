# disk-cleaner

macOS 磁盘空间清理工具。扫描可清理内容，预览确认后移入废纸篓（可恢复）。

当前进度：**阶段 1-3 已完成** —— 扫描引擎、安全删除、单元测试、Fyne 图形界面全部就绪，可打包为双击运行的 `.app`。

## 设计原则

**1. 扫描与删除彻底分离**

扫描阶段只读，产出可清理项列表；删除是独立动作，必须显式触发。

**2. 默认不删，删除可恢复**

- 默认 `--dry-run` 式预览，不做任何改动
- 执行前需交互确认，或显式传 `--yes`
- 删除 = 移动到 `~/.Trash`，随时可从废纸篓恢复
- 废纸篓重名自动追加序号（`name 2`、`name 3`），**绝不覆盖**

**3. 白名单 + 双重闸门**

所有删除必须通过 `internal/guard` 校验：

- 拒绝任何含 `..` 的路径（不依赖 `Clean` 结果，字面拦截）
- 只允许预定义根目录下的路径，且不能是根目录本身
- 硬性保护 `/System`、`/usr`、`~/Documents`、`~/Library/Keychains`、`~/.ssh` 等
- 前缀比较带 `/` 后缀，防止 `~/Library2` 被误判为 `~/Library` 子路径

**4. 风险分级**

| 等级 | 含义 | 例子 |
|---|---|---|
| 低 | 纯缓存，自动重建 | `~/Library/Caches`、`~/.npm/_cacache` |
| 中 | 需重新下载或生成 | 模拟器镜像、Maven 仓库、大文件 |
| 高 | 可能丢失用户数据 | Docker 虚拟机数据、废纸篓、APFS 快照 |

**5. 永久删除项单独隔离**

废纸篓内容、APFS 快照标记为 `Permanent`，**不走**废纸篓流程（它们已在废纸篓里，或需 sudo），由专用流程处理。

## 安装与构建

```bash
make build          # 编译到 bin/disk-cleaner
make app            # 打包为 dist/磁盘清理.app（含 ad-hoc 签名）
make test           # 单元测试
make cover          # 覆盖率
make check          # 格式 + vet + 测试
```

> 注意：本机 Go 模块缓存位于 `~/.gvm`。若在受限沙箱中构建，需提升文件访问权限，否则会报 `open .../sumdb/...: operation not permitted`。

## 图形界面

```bash
make app && open "dist/磁盘清理.app"
# 或
./bin/disk-cleaner gui
```

无参数启动也直接进入图形界面（`./bin/disk-cleaner`），这是双击 `.app` 的启动路径。

界面能力：

- 启动即自动扫描，按类别分组展示（开发缓存 / 用户缓存 / 系统缓存 / 垃圾文件 / 大文件 / APFS 快照）
- 每项显示体积与彩色风险标记（● 低 绿 / ● 中 橙 / ● 高 红）
- 支持逐项勾选，或按类别「全选本类 / 全不选」
- 顶部实时汇总：总项数、总大小、已选项数与体积
- 清理前弹二次确认，高风险项额外警告
- 清理后自动重新扫描刷新列表
- 永久删除项（废纸篓、APFS 快照）**置灰不可勾选**，强制走专用流程

界面与 CLI 共用同一套 `scan` / `clean` 引擎，安全语义完全一致。

### 关于签名

`make app` 会对产物做 ad-hoc 签名（`codesign -s -`）。未签名的 `.app` 会被 Gatekeeper 拒绝，表现为双击无反应。

**本地自用足够**；对外分发仍需 Apple 开发者证书 + `notarytool` 公证，否则用户需手动在「系统设置 → 隐私与安全性」中放行。

## 使用

### 扫描（只读）

```bash
disk-cleaner scan                 # 默认大文件阈值 500 MiB
disk-cleaner scan --large 1024    # 阈值改为 1 GiB
disk-cleaner scan --json          # JSON 输出，便于脚本消费
```

### 清理

```bash
# 先 dry-run 看会做什么
disk-cleaner clean --category 用户缓存 --dry-run

# 确认后执行（会交互确认）
disk-cleaner clean --category 用户缓存

# 同时清理多个类别
disk-cleaner clean --category 用户缓存 --category 开发缓存

# 脚本场景跳过确认（谨慎）
disk-cleaner clean --category 开发缓存 --yes
```

可用类别：`用户缓存`、`系统缓存`、`开发缓存`、`大文件`、`垃圾文件`、`APFS 快照`。

类别名拼错会明确报错，不会静默当作「无匹配」。

## 扫描覆盖范围

| 类别 | 覆盖内容 |
|---|---|
| 用户缓存 | `~/Library/Caches`、`~/Library/Logs`、`~/Library/Saved Application State` |
| 系统缓存 | `/Library/Caches`、`/private/var/log/asl` |
| 开发缓存 | Xcode DerivedData/DeviceSupport/模拟器、npm/yarn/pnpm/bun、Gradle/Maven、Cargo、Go modules、pip、`~/.cache`、Docker |
| 大文件 | Downloads/Documents/Desktop/Movies/Pictures/Application Support 下超阈值文件（最多 50 条） |
| 垃圾文件 | `~/.Trash`、iOS 备份、下载目录中 90 天未修改的文件 |
| APFS 快照 | `tmutil listlocalsnapshots` |

所有目标在**运行时探测存在性** —— 不存在的目录不会产生任何条目，也不会报错。

## 已知限制

- **需要完全磁盘访问权限的目录**（`~/Library/Mail`、`~/Library/Messages` 等）扫描时会被跳过，并在大小统计中体现为偏小
- **APFS 快照删除需要 sudo**，工具不在扫描阶段提权，只提示手动命令 `tmutil deletelocalsnapshots <日期>`
- **`/Library/Caches` 部分子目录需要管理员权限**，无权限时跳过该子树
- 快照类项目 `Size` 为 0，因为 `tmutil` 不提供单个快照的体积

## 项目结构

```
disk-cleaner/
├── cmd/disk-cleaner/        CLI 入口（main / json / categories）
├── packaging/Info.plist     .app 打包配置
└── internal/
    ├── model/               统一数据模型（Item / Risk / Category / Report）
    ├── guard/               删除安全闸门（白名单 + 保护路径）
    ├── trash/               移入废纸篓（重名不覆盖、跨卷回退）
    ├── scan/                六个扫描器 + 并发调度 + 去重
    ├── clean/               引擎：guard + trash 组合执行
    └── ui/                  Fyne 图形界面（Controller + mainView）
```

依赖方向单向：`cmd → ui → clean → {guard, trash} → model`，`cmd → scan → model`。`scan` 不依赖 `clean`，确保扫描层无法执行删除。

UI 层通过 `Controller` 访问引擎，不直接操作文件系统；扫描与清理均在其他 goroutine 执行，UI 更新统一经 `fyne.Do` 回到主线程。

## 测试

30+ 个单元测试，重点覆盖安全边界：

- **guard**：`..` 穿越、受保护路径、根目录本身、白名单外、前缀混淆（`Library2` vs `Library`）
- **trash**：重名不覆盖、目录递归、符号链接保留、跨卷复制、序号递增
- **clean**：白名单拒绝、永久删除项隔离、单项失败不中断整批
- **scan**：并发容错、去重与类别优先级、阈值筛选、废纸篓标记
- **ui**：永久删除项禁用、勾选统计、选中项排序、空报告占位

覆盖率：`clean` 92.5%、`guard` 85.4%、`scan` 76.7%、`trash` 62.7%、`ui` 49.3%。

界面截图可通过环境变量生成，便于人工核对布局：

```bash
UI_SNAPSHOT=/tmp/ui.png go test ./internal/ui/ -run TestRenderMainView -v
```

## 后续阶段

- **阶段 4（剩余）**：应用图标、代码签名证书、公证流程
