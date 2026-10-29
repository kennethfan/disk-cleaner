# disk-cleaner

macOS 磁盘空间清理工具。扫描可清理内容，预览确认后移入废纸篓（可恢复）。

单一二进制，命令行与图形界面共用同一套扫描/清理引擎；不装守护进程，不联网，不上传任何文件信息。

- **只读扫描**：`scan` 只产出清单，不动任何文件
- **删前必看**：清理前逐项确认；删除 = 移入废纸篓，可一键撤销
- **不该删的删不掉**：所有删除经 `internal/guard` 白名单校验，`/System`、`/usr`、`~/.ssh`、`~/Documents` 等硬性保护
- **风险分级**：低（纯缓存，自动重建）/ 中（需重新下载或生成）/ 高（可能丢用户数据）

## 快速上手

```bash
git clone https://github.com/kennethfan/disk-cleaner.git
cd disk-cleaner
make build                 # 产出 bin/disk-cleaner
./bin/disk-cleaner         # 不带参数直接打开图形界面
```

图形界面里三步：

1. 点「✨ 开始智能扫描」等进度跑完（全盘通常几十秒，大目录可随时「暂停」）
2. 在「清理建议」里看分类与明细，勾选要清的项（「全选推荐项」只覆盖推荐分类且只选低风险条目）
3. 点「🗑 安全清理」，二次确认后逐项移入废纸篓；后悔就点「↩︎ 撤销本次清理」

命令行等价流程：

```bash
./bin/disk-cleaner scan                                  # 只读预览
./bin/disk-cleaner clean --category 用户缓存 --dry-run    # 看清会做什么
./bin/disk-cleaner clean --category 用户缓存              # 交互确认后执行
```

## 环境要求

| 项目 | 要求 |
|---|---|
| 系统 | macOS 11（Big Sur）及以上 |
| Go | 1.26+（见 `go.mod`） |
| 构建工具 | Xcode Command Line Tools（`xcode-select --install`）；WebView 壳依赖 cgo |
| 可选 | 重新生成前端绑定需要 `wails3` CLI；重新生成图标需要 Python3 + Pillow |

Intel 与 Apple Silicon 均可构建，未做交叉编译，产物与构建机架构一致。

## 安装

每次发版会在 [Releases](https://github.com/kennethfan/disk-cleaner/releases) 附上打包好的 `.app`，也可以从源码构建。产物是单一二进制，不向系统目录写入任何东西。

### 方式一：下载 .app（最省事）

从 [Releases](https://github.com/kennethfan/disk-cleaner/releases) 下载 `disk-cleaner-v*-macos-<arch>-adhoc.app.zip`：

- `<arch>` 是构建架构：Intel Mac 用 `x86_64`，Apple Silicon 用 `arm64`。工具不交叉编译，下错架构打不开
- 解压得到「磁盘清理.app」，拖进 `/Applications`
- 附件是 **ad-hoc 签名、未公证** 的，首次打开会被 Gatekeeper 拦下：Finder 里右键 →「打开」，或执行 `xattr -dr com.apple.quarantine "/Applications/磁盘清理.app"`

### 方式二：命令行工具（推荐自己构建）

```bash
make build
sudo install -m 755 bin/disk-cleaner /usr/local/bin/
# 或者把 bin/ 加进 PATH，不想污染系统目录时更省事
export PATH="$PWD/bin:$PATH"

disk-cleaner scan
```

### 方式三：自己打包可双击的 .app

```bash
make app
open "dist/磁盘清理.app"         # 或把 .app 拖进 /Applications
```

`make app` 产出的是 **ad-hoc 签名**（未公证）：自己构建、自己这台机器打开正常；把 .app 拷给别人时，对方首次打开会被 Gatekeeper 拦下，需在 Finder 里右键 →「打开」，或执行：

```bash
xattr -dr com.apple.quarantine "/path/to/磁盘清理.app"
```

有 Apple 开发者账号时可正式签名并公证（`make app SIGN_IDENTITY=...` → `make notary-setup` → `make notarize`），细节见 [docs/signing.md](docs/signing.md)。

### 首次运行

macOS 会在访问其他应用数据时弹权限提示。要在结果里看到 `~/Library/Mail`、`~/Library/Messages` 这类目录的占用，需到 **系统设置 → 隐私与安全性 → 完全磁盘访问权限** 里手动添加终端（或「磁盘清理.app」）并重启它。不加也能用，只是这些目录被跳过、大小统计偏小。

### 卸载

```bash
rm -f /usr/local/bin/disk-cleaner      # 方式二装进去的二进制
rm -rf "/Applications/磁盘清理.app"      # 方式一或三得到的 .app
rm -rf ~/.config/disk-cleaner          # 排除规则与扫描范围配置
```

清理掉的内容都在废纸篓里，确认无用后清空废纸篓才真正释放空间。

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
| 低 | 纯缓存，自动重建 | `~/Library/Caches`、`~/.npm/_cacache`、Maven 本地仓库 |
| 中 | 需重新下载或生成 | Xcode DerivedData、模拟器镜像、Docker 虚拟机数据、大文件 |
| 高 | 可能丢失用户数据 | 废纸篓内容、APFS 快照 |

**5. 永久删除项单独隔离**

废纸篓内容、APFS 快照标记为 `Permanent`，**不走**废纸篓流程（它们已在废纸篓里，或需 sudo），由专用流程处理。

## 图形界面

```bash
make app && open "dist/磁盘清理.app"
# 或
./bin/disk-cleaner gui
```

无参数启动也直接进入图形界面（`./bin/disk-cleaner`），这是双击 `.app` 的启动路径。

界面是 Wails v3 的 WebView 壳（`internal/webui`）：前端是纯 HTML/CSS/JS，
通过绑定直接调 Go 侧的 `controller.Controller`，不经过 HTTP 端口，
也没有任何前端构建步骤。

界面能力：

- **概览**：磁盘环形图 + 可用容量 + 各分类占用条；按配置恢复扫描范围（全盘 / 指定目录）
- **扫描中**：环形进度与当前扫描器名称；**扫描可暂停**，「继续」从原处接着扫，「取消」彻底终止
- **清理建议**：按类别分组、可展开明细；逐项勾选、类别级全选、「全选推荐项」（只覆盖低风险分类）、行内排除
- **大文件**：独立列表，显示位置与修改时间，可多选后移入废纸篓
- **排除列表**：增删规则即时保存并生效，扫描与清理共用同一套规则
- **清理流程**：二次确认（清单摘要 + 废纸篓说明）→ 逐项进度 → 完成页可一键撤销
- 永久删除项（废纸篓、APFS 快照）与指定目录结果**不可勾选**，避免误操作
- 页面刷新（Cmd+R）后仍恢复最近一次扫描结果，不退回「从未扫描」

### 暂停扫描

扫描大目录时（如 `~/Library/Caches` 6GB）可能耗时几十秒，期间可以随时暂停。

- 扫描中点「暂停」冻结进度，按钮变「继续」，进度以扫描器完成数为粒度
- 暂停期间各遍历 goroutine 阻塞在门闩上，**CPU 占用降为 0**，不会继续读盘
- 继续时从原处接着扫，**已完成的进度不丢失**
- 取消会终止扫描并保留上一次的结果列表

实现上用的是 `internal/gate` 门闩，而非 `context` 取消——因为取消是终止性的、不可恢复，而暂停需要可逆。检查点插在所有遍历循环里（目录大小统计、大文件遍历、子目录展开、指定目录扫描），最耗时的 `dirSize` 每个文件系统条目都会检查一次。

CLI 不支持暂停（终端交互收益低），用 Ctrl-C 直接取消即可。

**勾选语义**：类别与明细行互不联动。勾类别 = 选中该类别下所有可勾选项；明细行只能逐项勾选，避免因为类别级全选而意外删掉整片缓存。

界面与 CLI 共用同一套 `scan` / `clean` 引擎，安全语义完全一致。

### 前端开发

前端资源在 `internal/webui/assets/`，由 `go:embed` 编进二进制，改完 HTML/CSS/JS 重新 `make build` 即可，无需打包工具。

绑定文件（`assets/bindings-v2/`）由 `wails3 generate bindings` 从 `internal/webui/service.go` 的导出方法生成，**改过 service.go 的方法或结构体后必须执行 `make bindings`**，否则前端会继续调旧接口。

## 应用图标

图标由 AGNES API 生成（硬盘 + 琥珀色扫除笔触），存于 `packaging/`：

- `icon-source.png` —— 1024×1024 源图，圆角外已置透明
- `AppIcon.icns` —— 10 个尺寸的完整图标集

`make icon` 可从源图重新生成 `.icns`（需 Python3 + Pillow）。

## 签名与公证

默认 `make app` 产出 **ad-hoc 签名**版本，仅够本机自用。

要让他人双击即可打开，需 Apple 开发者账号：

```bash
# 1. 正式签名（启用 Hardened Runtime，公证的硬性要求）
make app SIGN_IDENTITY="Developer ID Application: Your Name (TEAMID)"

# 2. 保存公证凭据（只需一次）
make notary-setup APPLE_ID="you@example.com" TEAM_ID="ABCDE12345" APP_PASSWORD="xxxx-xxxx-xxxx-xxxx"

# 3. 提交公证并装订票据
make notarize

# 4. 复核
make verify-sign
```

详细说明、失败排查与不公证的分发方式见 [docs/signing.md](docs/signing.md)。

> 没有证书时 `make app` 仍可用，产出 ad-hoc 版本（仅本机可双击）；拿到证书后上述命令可直接使用，无需改代码。检查本机现有证书可用 `security find-identity -v -p codesigning`。

## 使用

### 扫描（只读）

```bash
disk-cleaner scan                 # 默认大文件阈值 500 MiB
disk-cleaner scan --large 1024    # 阈值改为 1 GiB
disk-cleaner scan --json          # JSON 输出，便于脚本消费

# 只扫描指定目录（忽略预设扫描器，结果仅可查看）
disk-cleaner scan --path ~/Downloads
disk-cleaner scan --path ~/Downloads --path ~/Movies

# 临时排除（叠加配置文件中的规则）
disk-cleaner scan --exclude node_modules --exclude "*.log"
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

## 排除列表

把不想看到的路径排除掉，避免它们出现在扫描结果里、也避免误删。

**三种写法**：

| 写法 | 示例 | 语义 |
|---|---|---|
| 绝对路径 | `~/Library/Caches/JetBrains` | 该目录及其**全部子项**都排除 |
| 目录/文件名 | `node_modules` | 排除**任意层级**中同名的目录 |
| 通配符 | `*.log`、`*.tmp` | 匹配路径最后一段（文件名） |

支持 `~` 展开；`#` 开头视为注释；空行忽略。

**配置位置**：`~/.config/disk-cleaner/config.yaml`

```yaml
exclude:
  - ~/Library/Caches/JetBrains   # JetBrains 索引缓存，删了要重建
  - node_modules
  - "*.log"
```

刻意不用 macOS 的 `~/Library/Application Support`：CLI 工具用 `~/.config` 更符合惯例，也方便手动编辑。

**生效环节**（两处，缺一不可）：

1. **扫描结果过滤**：6 个扫描器产出的项若命中规则，直接丢弃
2. **子目录展开过滤**：展开目录时，被排除的子项同样不出现

两处都做是因为：只过滤扫描结果的话，被排除的目录虽然不在初始列表里，展开父目录时仍会重新冒出来。

**界面提示**：扫描完成提示会附带「已排除 N 项（X GB）」，让你确认规则确实生效，不会误以为扫描漏了内容。

**GUI 管理**：侧边栏「排除列表」页可增删规则，改动即时保存；也可以在清理建议里对某一行点「排除」，直接把它所在目录加入规则并从当前结果中移除（无需重新扫描）。

添加规则有两种方式：

- **手写**：输入框里写路径、目录名或通配符
- **行内排除**：在清理建议的文件行上点「排除」

**规则写错会报错**，不会静默失效——否则你以为排除了，实际没排除，可能导致误删。

## 指定目录扫描

只想看某个目录占了多少时用。

```bash
disk-cleaner scan --path ~/Downloads
```

- 指定 `--path` 后**跳过全部预设扫描器**，只列出该目录下的内容
- 可重复指定多个 `--path`
- 结果按体积降序，目录递归统计大小

**GUI**：概览页把扫描范围切到「指定目录」并添加目录即可；切回「全盘扫描」恢复。范围写入配置文件，重开应用不丢失。

**安全边界**：指定目录的结果一律标记为**只读**——可以浏览、看大小，但复选框位置显示「只读」标记、不可勾选。删除仍只对 guard 白名单内的路径开放。

这样做是因为：允许浏览任意目录很有用，但把任意目录变成"可删除"风险太高。想清理某个缓存目录，用默认扫描；想了解某个目录的构成，用指定目录扫描。

## 扫描覆盖范围

| 类别 | 覆盖内容 |
|---|---|
| 用户缓存 | `~/Library/Caches`、`~/.cache`、`~/Library/Saved Application State`（按子目录拆分后归入非开发类缓存） |
| 系统缓存 | `/Library/Caches`、`/private/var/log/asl`、`~/Library/Logs` |
| 开发缓存 | Xcode DerivedData/DeviceSupport/模拟器、npm/yarn/pnpm/bun、Gradle/Maven、Cargo、Go modules、pip、`~/.cache`、Docker |
| 大文件 | Downloads/Documents/Desktop/Movies/Pictures/Application Support 下超阈值文件（最多 50 条） |
| 垃圾文件 | `~/.Trash`、iOS 备份、下载目录中 90 天未修改的文件 |
| APFS 快照 | `tmutil listlocalsnapshots` |

所有目标在**运行时探测存在性** —— 不存在的目录不会产生任何条目，也不会报错。

`~/Library/Caches` 与 `~/.cache` 混着应用缓存和开发者工具缓存，拆分时按子目录名归类：命中开发者工具（`AndroidStudio2.3`、`go-build`、`zig`、`npm`、`opencode` 等）的归「开发缓存」，其余归「用户缓存」。判定只认确定性高的工具名前缀 —— 宁可漏判成应用缓存，也不把应用数据误标成可随构建重建的缓存。

## 常见问题

**双击 .app 没反应 / 提示「已损坏或无法验证」**
ad-hoc 签名 + Gatekeeper 的典型表现。右键 →「打开」，或 `xattr -dr com.apple.quarantine`；排查签名状态用 `make verify-sign`。

**扫描结果比 Finder 显示的小**
部分目录需要「完全磁盘访问权限」，无权时被跳过（见下方已知限制）。

**构建报 `open .../sumdb/...: operation not permitted`**
Go 模块缓存所在目录被沙箱限制，给构建环境放宽该目录的访问权限即可，与代码无关。

**APFS 快照删不掉**
工具不主动提权，快照项只提供命令：`sudo tmutil deletelocalsnapshots <日期>`。

**改了 `internal/webui/service.go` 但界面没变化**
前端绑定是生成物，需 `make bindings` 再生，否则界面仍在调旧接口。

## 已知限制

- **需要完全磁盘访问权限的目录**（`~/Library/Mail`、`~/Library/Messages` 等）扫描时会被跳过，并在大小统计中体现为偏小
- **APFS 快照删除需要 sudo**，工具不在扫描阶段提权，只提示手动命令 `tmutil deletelocalsnapshots <日期>`
- **`/Library/Caches` 部分子目录需要管理员权限**，无权限时跳过该子树
- 快照类项目 `Size` 为 0，因为 `tmutil` 不提供单个快照的体积

## 项目结构

```
disk-cleaner/
├── cmd/disk-cleaner/        CLI + 图形界面入口（main / json / categories / exclude）
├── docs/signing.md          签名与公证完整指南
├── packaging/
│   ├── Info.plist           .app 打包配置
│   ├── AppIcon.icns         图标集（10 尺寸）
│   └── icon-source.png      1024×1024 图标源图
└── internal/
    ├── model/               统一数据模型（Item / Risk / Category / Report）
    ├── guard/               删除安全闸门（白名单 + 保护路径）
    ├── trash/               移入废纸篓（重名不覆盖、跨卷回退）
    ├── gate/                扫描暂停门闩（可逆，区别于 ctx 取消）
    ├── exclude/             排除规则匹配器（路径 / 名称 / 通配符）
    ├── config/              ~/.config/disk-cleaner/config.yaml 读写
    ├── scan/                六个扫描器 + 指定目录 + 并发 + 去重 + 子项懒加载
    ├── clean/               引擎：guard + trash 组合执行
    ├── controller/          引擎门面：扫描/清理/排除/范围的并发安全入口
    └── webui/               Wails v3 WebView 界面（assets/ 为前端资源与绑定）
```

依赖方向单向：`cmd → webui → controller → clean → {guard, trash} → model`，`cmd → scan → model`。`scan` 不依赖 `clean`，确保扫描层无法执行删除。

界面层通过 `controller.Controller` 访问引擎，不直接操作文件系统；扫描与清理都在后台 goroutine 执行，进度经 CustomEvent 推给前端，界面只按事件更新 DOM。

## 测试

30+ 个单元测试，重点覆盖安全边界（`make check` 一次跑完格式检查 + vet + 全部测试；测试带 `-race`，因为扫描、清理、暂停都在后台 goroutine 上跑）：

- **guard**：`..` 穿越、受保护路径、根目录本身、白名单外、前缀混淆（`Library2` vs `Library`）
- **trash**：重名不覆盖、目录递归、符号链接保留、跨卷复制、序号递增
- **clean**：白名单拒绝、永久删除项隔离、单项失败不中断整批
- **scan**：并发容错、去重与类别优先级、阈值筛选、废纸篓标记
- **webui**：报表视图的分组与大文件拆分、key 取回真实项、行内排除过滤、永久/只读项拒绝、清理顺序

## 构建目标速查

| 命令 | 作用 |
|---|---|
| `make build` | 编译单一二进制（CLI + 图形界面） |
| `make bindings` | 从 `service.go` 重新生成前端绑定（改接口后必须执行） |
| `make app` | 打包 `.app`（图标 + ad-hoc 签名） |
| `make app SIGN_IDENTITY=...` | 用开发者证书签名 |
| `make icon` | 重新生成 `.icns` |
| `make verify-sign` | 检查签名、Gatekeeper、公证票据 |
| `make notary-setup` | 保存公证凭据到钥匙串 |
| `make notarize` | 提交公证并装订票据 |
| `make app-unsigned` | 打包但不签名（排查签名问题用） |
| `make check` | 格式 + vet + 全部测试 |
