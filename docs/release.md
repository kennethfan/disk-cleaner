# 发版指南

本文说明如何发布一个新版本。发版由推送 tag 自动完成，但**改版本号和本地验证必须在推 tag 之前做对**——tag 一旦推上去，workflow 会立刻开始构建，版本校验不通过就会在第一个 job 直接失败。

## 流程总览

```
改 Makefile 的 VERSION
   ↓
make app 本地验证（版本号进了 Info.plist、签名通过）
   ↓
提交 + 推送 main
   ↓
打 annotated tag vX.Y.Z + 推送 tag
   ↓
Release workflow 自动构建两个架构并发布
```

顺序不能颠倒：`release.yml` 的 `version` job 会校验 tag 与 `Makefile` 的 `VERSION` 一致，不一致直接失败，两个 macOS runner 白跑。

## 一、定版本号

版本号只有一处：`Makefile` 的 `VERSION`。

```make
VERSION := 0.1.1
```

`.app` 的 `Info.plist` 在 `make app` 时从它替换 `@VERSION@` 生成，所以改完必须重新打包，`dist/` 里的旧产物不会自动跟着变。

语义化版本的选择：

| 变更性质 | 版本 | 例子 |
|---|---|---|
| 修 bug、改文案、调口径 | 补丁位 `x.y.Z` | 可用空间改用系统口径 → `0.1.1` |
| 加功能、加扫描器、加依赖 | 次版本 `x.Y.0` | 新增容器缓存扫描器 → `0.2.0` |
| 不兼容的界面/CLI 变更 | 主版本 `X.0.0` | 移除某个 `clean` 参数 → `1.0.0` |

## 二、本地验证

推 tag 之前先自己打包一遍，确认版本号真的进了产物：

```bash
make app
/usr/libexec/PlistBuddy -c "Print :CFBundleShortVersionString" "dist/磁盘清理.app/Contents/Info.plist"
codesign --verify --deep --strict "dist/磁盘清理.app"
```

期望 `CFBundleShortVersionString` 与 `Makefile` 里的版本一致，签名校验无输出即通过。

再跑一遍完整检查（CI 会跑同样的目标，提前跑能省一次失败重试）：

```bash
make check     # gofmt + vet + 全部测试（带 -race）
```

## 三、提交与打 tag

```bash
git add -A
git commit -m "chore: 版本号从 0.1.0 改为 0.1.1"
git push origin main

git tag -a v0.1.1 -m "v0.1.1

概览页的可用空间改用与 Finder、系统设置一致的口径。"
git push origin v0.1.1
```

tag 用 annotated（`-a`）而不是轻量 tag：`git describe` 和 Release 页面都依赖它携带的说明与作者信息。

> 本仓库直连 `github.com:443` 可能不通，需要临时走本机代理：
> `git -c http.proxy=http://127.0.0.1:7897 push origin main`（推送 tag 同理）。用一次性参数，不要写进 `git config`。

## 四、workflow 会做什么

推送 `v*` 形式的 tag 触发 [`.github/workflows/release.yml`](../.github/workflows/release.yml)，三个 job 串成一条闸门链：

### 1. `version`

从 tag 名剥掉 `v`，与 `Makefile` 的 `VERSION` 比对。不一致立刻失败——这一步放在最前面，就是为了不让两个 runner 白跑十几分钟才报错。

### 2. `build`（矩阵，两个架构并行）

| runner | 架构 | 说明 |
|---|---|---|
| `macos-15` | `arm64` | Apple Silicon |
| `macos-15-intel` | `x86_64` | Intel，GitHub 最后一个 x86_64 镜像 |

两个架构**各在自己原生架构的 runner 上构建**：WebView 壳依赖 cgo，从 arm64 交叉编译 x86_64 要额外摆弄 SDK 与链接器，不如让 Intel 机器自己跑。

标签钉在 `macos-15` 而不是 `macos-latest`，是因为 `macos-latest`（现为 macOS 26）只有 arm64；两边同一代系统，SDK 与最低系统版本才一致。

每个 runner 做这些检查：

- `make check`（只在 arm64 上跑一遍，main 的 CI 已经在跑，这里只确认打 tag 的 commit 没坏）
- `make build` + `make app`
- `./bin/disk-cleaner help` 能运行
- `lipo -archs` 断言产物架构与矩阵声明一致（runner 标签写错时当场发现，而不是等用户下载后闪退）
- `codesign --verify --deep --strict`

打包用 `ditto -c -k --keepParent` 而不是 `zip`：`zip` 会丢掉符号链接与扩展属性，解压出来的 `.app` 过不了 `codesign`，双击报「已损坏」。打完还会解压回临时目录再验一次签，确认**附件本身**而不只是构建产物是完好的。

附件命名 `disk-cleaner-v<版本>-macos-<架构>-adhoc.app.zip`，保持 ASCII，避免下载链接与日志里的编码问题。

### 3. `release`

- 用 [`.github/release-notes-template.md`](../.github/release-notes-template.md) 渲染正文，把 `@VERSION@` 换成版本号
- 自动追加「本版变更」：从上一个 tag 到当前 tag 的 `git log`（首个版本没有上界，回退到全部历史）
- `gh release create --verify-tag` 上传两个 zip，一次发布

不用 `gh --generate-notes` 是因为本仓库直推、不走 PR，那种历史下生成出来只剩一行 Full Changelog。

## 五、发版后验证

别只看 workflow 显示绿色就结束——下载附件本身验一遍：

```bash
cd /tmp && rm -rf v && mkdir v && cd v
curl -sLO https://github.com/kennethfan/disk-cleaner/releases/download/v0.1.1/disk-cleaner-v0.1.1-macos-x86_64-adhoc.app.zip
ditto -x -k disk-cleaner-v0.1.1-macos-x86_64-adhoc.app.zip out
codesign --verify --deep --strict "out/磁盘清理.app"
/usr/libexec/PlistBuddy -c "Print :CFBundleShortVersionString" "out/磁盘清理.app/Contents/Info.plist"
lipo -archs "out/磁盘清理.app/Contents/MacOS/disk-cleaner"
"out/磁盘清理.app/Contents/MacOS/disk-cleaner" help
```

四项都对上（签名通过、版本号正确、架构与附件名一致、二进制能跑），才算真的发对了。

想确认某个修复确实进了包，可以直接在二进制里找它引入的字符串：

```bash
LC_ALL=C grep -c "含系统可回收" "out/磁盘清理.app/Contents/MacOS/disk-cleaner"
```

## 常见问题

**workflow 在 `version` job 就失败了**
tag 与 `Makefile` 的 `VERSION` 不一致。删掉 tag 重来：

```bash
git tag -d v0.1.1                 # 删本地
git push origin :refs/tags/v0.1.1 # 删远程
```

然后改对 `VERSION`、重新提交、重新打 tag。**不要**用 `git tag -f` 强推覆盖已发布的版本——附件会与 tag 指向的 commit 对不上。

**产物架构与附件名不一致**
矩阵里的 `runner` 标签写错，或 GitHub 换了镜像的默认架构。`build` job 里的 `lipo -archs` 断言会拦住它。

**Release 页面「本版变更」是空的**
`git describe --tags --abbrev=0 "v${VERSION}^"` 找不到上一个 tag（首次发版属正常，会回退到全部历史）。若确实有历史 tag 却仍为空，检查 workflow 的 `fetch-depth: 0` 是否还在——浅克隆拿不到完整历史。

**改完版本号，本地 `dist/` 里的 `.app` 还是旧版本**
`make app` 才会重新渲染 `Info.plist`。`make clean` 会连 `dist/` 一起删掉（保留已提交的前端绑定）。
