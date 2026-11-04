# macOS 可清理目录速查卡

> 整理日期：2026-11-04（初版 2026-10-29）｜ 数据来源：**本机（Kenneth 的 Mac）只读实测**，非通用示例值
> 数据卷：总计 **250.8 GB** / 已用 **204.0 GB** / 可用 **20.1 GB**（81%）
> 本档可回收约 **26.7 GB（绿区）+ 6~8 GB（黄区）**

**关于「可用空间」的两个口径**（初版只写了一个数，容易对照错）：

| 来源 | 可用 | 说明 |
|---|---|---|
| `df` / 磁盘工具 | 20.1 GB | `statfs` 的 `Bavail`，只算此刻就能分配的块 |
| Finder / 系统设置 | 23.1 GB | 含系统按需回收的 3.0 GB（APFS 预留、可清理缓存等） |

下文所有目录大小都用 `du -sh`（实际占用），与 Finder 的「容量」口径一致。

---

## 一、分类总览：三个维度

同一批目录按三个维度切分，任一维度都能快速判断"该不该动"：

| 维度 | 回答问题 | 用途 |
| --- | --- | --- |
| **归属方** | 谁在管理它？ | 决定你有没有权限/该不该动 |
| **风险等级** | 删了会怎样？ | 决定敢不敢删（绿/黄/红） |
| **再生成本** | 多久能恢复？ | 决定清得值不值 |

---

## 二、按「归属方」分类

| 类别 | 典型目录 | 本机实测 | 清理权 |
| --- | --- | --- | --- |
| **A. 用户级缓存** | `~/Library/Caches/*`、`~/.cache/*` 中的非工具项 | 1.9 GB | 自己可删，系统/应用自动重建 |
| **B. 开发工具链缓存** | `~/.m2`、`~/.gradle`、`~/.npm`、`~/go/pkg/mod`、`go-build`、`pip`，以及上面两个缓存根里的工具项 | **22.5 GB** | 可删，下次构建重拉 |
| **C. 应用沙盒容器** | `~/Library/Containers/*`（微信 8.8G、Docker 2.9G 实占、腾讯会议 554M） | 12 GB | ⚠️ 只能进容器内清缓存，**不可整目录删**（删容器=丢应用数据） |
| **D. 应用支持数据** | `~/Library/Application Support/*`（Chrome 8.6G、微信开发者工具 1.1G、Slack 1.0G） | 16 GB | ⚠️ 混合区，仅明确标 cache 的子目录可动 |
| **E. 系统缓存与日志** | `/Library/Caches`、`/private/var/log`、`~/Library/Logs` | 148 MB | 需 sudo 或系统自管，收益低 |
| **F. 系统保留区** | `/System`、`/private/var/folders`、`/private/var/db`、`/Volumes` | — | ⛔ 绝对禁区 |
| **G. 用户文档区** | Downloads / Desktop / Documents / Movies / Pictures | 自行盘点 | 非缓存，只能人工判断价值 |

**A 与 B 的界线**：`~/Library/Caches` 与 `~/.cache` 里混着应用缓存和开发工具缓存。
按目录名判定——命中 `go-build`、`JetBrains`、`opencode`、`hyperframes`、`node`、`uv` 等
工具名前缀的归 B，其余归 A。本机实测：`~/Library/Caches`（2.37 GB）里 A 占 1.87 GB / B 占 0.55 GB；
`~/.cache`（3.48 GB）里 A 占 0.05 GB / B 占 3.46 GB。

---

## 三、按「风险等级」分类

| 等级 | 判定 | 目录 |
| --- | --- | --- |
| 🟢 绿 | 删了无损，自动重建 | A 全部、B 全部、C 内 `Caches/` 子目录、`~/Library/Logs` |
| 🟡 黄 | 先确认再删 | D 中带 cache/temp 字样的子目录、Docker 镜像、微信媒体缓存、`~/Library/Developer`（装 Xcode 时）、APFS 快照 |
| 🔴 红 | 永不触碰 | `Application Support/Google`（Chrome 用户数据）、`Application Support/MobileSync`（iOS 备份）、微信 `db_storage`、`/System`、`/private/var/folders`、`~/Pictures`（照片图库）、`~/Library/Mail`、`~/Library/Keychains` |

---

## 四、按「再生成本」分类

| 再生成本 | 目录 | 清理心理负担 |
| --- | --- | --- |
| 秒级（本地计算） | `~/Library/Caches`、`go-build`、JetBrains 索引、缩略图缓存 | 零 |
| 分钟级（需网络重下） | `~/.m2`、`~/.npm`、`~/.gradle`、`~/go/pkg/mod`、`pip` | 低（弱网时注意） |
| 需重新配置 | Docker 镜像、IDE 索引、微信开发者工具 | 中 |
| **不可再生** | `Application Support/Google`、微信 `db_storage`、文档区 | ⛔ 不清 |

---

## 五、三问判定法（陌生目录通用）

1. **能再生成吗？** 不能 → 红区，停手
2. **是缓存还是用户数据？** 路径含 `Caches`/`cache`/`tmp`/`logs` 才考虑，否则黄区
3. **谁管理？** 系统（`/System`、`var/folders`）不碰；应用沙盒内进容器清，不整删

---

## 六、逐目录明细（本机实测）

| 路径 | 类别 | 风险 | 实测 | 说明 | 安全清理方式 |
| --- | --- | --- | --- | --- | --- |
| `~/.m2/repository` | B | 🟢 | **9.6 GB** | Maven 依赖缓存 | 删 repository，下次构建重拉 |
| `~/go/pkg/mod` | B | 🟢 | 4.0 GB | Go 模块缓存 | `go clean -modcache` |
| `~/.npm` | B | 🟢 | 3.7 GB | npm 缓存（`_cacache` 1.1 GB） | `npm cache clean --force` |
| `~/.cache/opencode` | B | 🟢 | 2.1 GB | opencode 工具缓存 | 确认无会话数据后删目录 |
| `~/.cache/hyperframes` | B | 🟢 | 1.3 GB | 工具缓存 | 直接删 |
| `~/.gradle/caches` | B | 🟢 | 527 MB | Gradle 构建缓存 | `rm -rf ~/.gradle/caches` |
| `~/Library/Caches/go-build` | B | 🟢 | 354 MB | Go 编译缓存 | `go clean -cache` |
| `~/Library/Caches/JetBrains` | B | 🟢 | 149 MB | IDE 索引（重开会重建） | 退出 IDE 后删 |
| `~/Library/Caches/Yarn` | B | 🟢 | 55 MB | Yarn 缓存 | `yarn cache clean` |
| `~/Library/Caches/pnpm` | B | 🟢 | 17 MB | pnpm store 缓存 | `pnpm store prune` |
| `~/.cargo` | B | 🟢 | 56 MB | Rust crate 缓存 | 直接删 |
| `~/Library/Caches/pip` | B | — | **不存在** | 本机未用 pip | — |
| `~/Library/Caches/Google` | A | 🟢 | 1.1 GB | Chrome 缓存（≠ Application Support/Google） | 退出 Chrome 后清 `*/Cache*` |
| `~/Library/Caches/qoder-cn-updater` | A | 🟢 | 612 MB | Qoder 更新包缓存 | 直接删 |
| `~/Library/Caches/GeoServices` | A | 🟢 | 56 MB | 地图瓦片缓存 | 可删 |
| `~/.cache/node` | B | 🟢 | 63 MB | Node 工具缓存 | 可删 |
| `~/.cache/oh-my-opencode` | A | 🟢 | 53 MB | 工具缓存 | 可删 |
| `~/Library/Logs` | E | 🟢 | 29 MB | 用户级日志 | 可删 |
| `/private/var/log` | E | 🟡 | 113 MB | 系统日志 | `sudo` 清理，收益低可不做 |
| `/Library/Caches` | E | 🟡 | 5.8 MB | 系统级缓存 | 需 sudo，收益极低 |
| `~/Library/Containers/com.tencent.xinWeChat` | C | 🟡 | **8.8 GB** | 微信媒体（attach/video 按月池 + 孤儿） | 按月筛选 + 孤儿清理，**不动 db_storage** |
| `~/Library/Containers/com.docker.docker` | C | 🟡 | 2.9 GB 实占 | Docker 虚拟磁盘（**表观 64 GB，稀疏文件**） | `docker system prune -a`（会删镜像） |
| `~/Library/Containers/com.tencent.meeting` | C | 🟡 | 554 MB | 腾讯会议缓存 | 应用内清理设置 |
| `~/Library/Application Support/微信开发者工具` | D | 🟡 | 1.1 GB | 旧项目缓存 | 确认项目无引用后清 |
| `~/Library/Application Support/Slack` | D | 🟡 | 1.0 GB（`Cache` 子目录 0.54 GB） | 含 Cache/Service Worker 子目录 | 只清 `Cache`/`Code Cache` 子目录 |
| `~/Library/Application Support/SodaMusic` | D | 🟡 | 638 MB | 音乐应用缓存（`LunaCacheV2`） | 只清 `LunaCacheV2` 子目录 |
| `~/Library/Application Support/Google` | D | 🔴 | **8.6 GB** | Chrome 用户数据（书签/密码/扩展） | ⛔ 禁止删除 |
| `~/Library/Application Support/MobileSync` | D | 🔴 | — | iPhone 备份 | ⛔ 禁止删除 |
| APFS 快照 | — | 🟡 | **0 个** | `tmutil listlocalsnapshots /` 为空 | 有快照时 `sudo tmutil deletelocalsnapshots <日期>` |

**两个易错点**：

- **Docker 的 64 GB 是假的**。`Docker.raw` 是稀疏文件，`stat` 报 64 GB（表观大小），
  实际只占 2.9 GB。用 `du -sh` 或 `ls -s` 看才是真占用；用 `find -size` 或按 `st_size`
  统计会虚高 61 GB。清理它只回收约 2.9 GB，不是 64 GB。
- **`~/Library/Caches/Google` 与 `~/Library/Application Support/Google` 是两回事**。
  前者是 Chrome 的纯缓存（可删，自动重建），后者是书签/密码/扩展（⛔ 红线）。

---

## 七、清理优先级建议（按收益/风险比）

| 顺序 | 动作 | 预计回收 | 耗时 | 风险 |
| --- | --- | --- | --- | --- |
| 1 | `~/.m2/repository` + `~/go/pkg/mod` + `~/.gradle/caches` | 14.8 GB | 秒级 | 无（重拉即可） |
| 2 | `~/.npm` + `~/.cache/opencode` + `~/.cache/hyperframes` | 7.1 GB | 秒级 | 无（opencode 先确认无会话数据） |
| 3 | `~/Library/Caches` 里的 A 类（Chrome 缓存、qoder 更新包等） | 1.9 GB | 1 分钟 | 无 |
| 4 | `go-build` + JetBrains + Yarn/pnpm + 杂项 | 0.7 GB | 秒级 | 无（IDE 重建索引稍慢） |
| 5 | `Application Support` 下的 Cache 子目录 | 2.2 GB | 5 分钟 | 低（只动 Cache 子目录） |
| 6 | 微信媒体（按月 + 孤儿） | 3~5 GB | 10 分钟 | 低（废纸篓可恢复） |
| 7 | Docker 无用镜像 | 按需 1~2 GB | 5 分钟 | 中（镜像需重拉） |

**绿区合计约 26.6 GB**（1~5 项），可用空间从 20.1 GB 回到 46 GB 左右；
加上黄区的微信媒体与 Docker，总计约 30~34 GB。

> 注意：本机的 `~/.m2` 已加入 disk-cleaner 的排除列表（`~/.config/disk-cleaner/config.yaml`），
> 所以工具的扫描结果里看不到它（会提示「已排除 1 项（9.3 GiB）」）。上面第 1 项要靠手动清。

---

## 八、红线清单（请勿删除）

- `~/Library/Application Support/Google`（8.6 GB）— Chrome 书签/密码/扩展，删了等于重置浏览器
- `~/Library/Application Support/MobileSync` — iPhone 备份
- 微信 `xwechat_files/<wxid>/db_storage` — 聊天记录数据库
- `~/Library/Containers/com.docker.docker/Data/vms` — Docker 全部镜像与容器
- `/System`、`/private/var/folders`、`/private/var/db` — 系统保留
- `~/Pictures/*.photoslibrary`、`~/Library/Mail`、`~/Library/Keychains` — 不可再生数据
- `~/Downloads`、`~/Desktop`、`~/Documents` — 你的文件，工具无权判断

---

## 九、执行与验证

**验证命令**（清理前后各跑一次对比）：

```bash
df -h /System/Volumes/Data                                  # 可用空间（statfs 口径）
du -sh ~/.m2 ~/Library/Caches ~/.cache ~/.npm ~/go/pkg/mod  # 各目录实际占用
tmutil listlocalsnapshots /                                 # 本地快照
```

> `du -sh` 与 `df` 的差额来自：文件系统元数据、APFS 预留空间、以及本机其他用户/系统占用。
> 两者不必相等，看趋势即可。

**安全删除原则**：

1. 优先走废纸篓（macOS 可装 `trash` CLI：`brew install trash`，用法 `trash <path>`）
2. 不用 `rm -rf` 直接删个人目录；确需命令删除时，务必逐条确认路径拼写
3. 一次只做一类，删完验证空间变化，再继续下一类
4. 涉及 `sudo` 的系统目录（`/Library/Caches`、`/private/var/log`）收益低，建议直接跳过
5. 稀疏文件（Docker.raw、虚拟机镜像）别看表观大小，用 `du -sh` 确认实际收益

> 本档仅为清单与判定方法，不含自动执行脚本。如需脚本化，应逐步执行、每步列出将删内容并等待确认。
