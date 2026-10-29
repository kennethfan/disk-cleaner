# macOS 可清理目录速查卡

> 整理日期：2026-10-29 ｜ 数据来源：**本机（Kenneth 的 Mac）只读实测**，非通用示例值
> 数据卷现状：已用 186 GB / 可用 25 GB（89%）｜本档可回收约 **26 GB（绿区）+ 3~5 GB（黄区）**

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
| **A. 用户级缓存** | `~/Library/Caches/*`、`~/.cache/*` | 9.9 GB | 自己可删，系统/应用自动重建 |
| **B. 开发工具链缓存** | `~/.m2`、`~/.gradle/caches`、`~/.npm`、`go-build`、`pip`、`zig` | **22.1 GB** | 可删，下次构建重拉 |
| **C. 应用沙盒容器** | `~/Library/Containers/*`（微信 8.6G、Docker 2.9G、腾讯会议 555M） | 12.1 GB | ⚠️ 只能进容器内清缓存，**不可整目录删**（删容器=丢应用数据） |
| **D. 应用支持数据** | `~/Library/Application Support/*`（Chrome 8.5G、Slack 1.0G） | 12.1 GB | ⚠️ 混合区，仅明确标 cache 的子目录可动 |
| **E. 系统缓存与日志** | `/Library/Caches`、`/private/var/log`、`~/Library/Logs` | 161 MB | 需 sudo 或系统自管，收益低 |
| **F. 系统保留区** | `/System`、`/private/var/folders`、`/private/var/db`、`/Volumes` | — | ⛔ 绝对禁区 |
| **G. 用户文档区** | Downloads / Desktop / Documents / Movies / Pictures | 自行盘点 | 非缓存，只能人工判断价值 |

---

## 三、按「风险等级」分类

| 等级 | 判定 | 目录 |
| --- | --- | --- |
| 🟢 绿 | 删了无损，自动重建 | A 全部、B 全部、C 内 `Caches/` 子目录、`~/Library/Logs` |
| 🟡 黄 | 先确认再删 | D 中带 cache/temp 字样的子目录、Docker 镜像、微信媒体缓存、`~/Library/Developer`（装 Xcode 时） |
| 🔴 红 | 永不触碰 | `Application Support/Google`（Chrome 用户数据）、微信 `db_storage`、`/System`、`/private/var/folders`、`~/Pictures`（照片图库）、`~/Library/Mail` |

---

## 四、按「再生成本」分类

| 再生成本 | 目录 | 清理心理负担 |
| --- | --- | --- |
| 秒级（本地计算） | `~/Library/Caches`、`go-build`、JetBrains 索引、缩略图缓存 | 零 |
| 分钟级（需网络重下） | `~/.m2`、`~/.npm`、`~/.gradle`、`pip` | 低（弱网时注意） |
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
| `~/.m2/repository` | B | 🟢 | **9.7 GB** | Maven 依赖缓存 | 删 repository，下次构建重拉 |
| `~/Library/Caches/Google` | A | 🟢 | 3.3 GB | Chrome 缓存（≠ Application Support/Google） | 退出 Chrome 后清 `*/Cache*` |
| `~/.gradle/caches` | B | 🟢 | 3.3 GB | Gradle 构建缓存 | `rm -rf ~/.gradle/caches` |
| `~/.npm` | B | 🟢 | 3.1 GB | npm 缓存 | `npm cache clean --force` |
| `~/.cache/opencode` | A | 🟢 | 2.1 GB | opencode 工具缓存（确认无会话数据后清） | 直接删目录 |
| `~/Library/Caches/go-build` | B | 🟢 | 1.6 GB | Go 编译缓存 | `go clean -cache` |
| `~/Library/Caches/JetBrains` | B | 🟢 | 1.1 GB | IDE 索引（重开会重建） | 退出 IDE 后删 |
| `~/.cache/hyperframes` | A | 🟢 | 1.1 GB | 工具缓存 | 直接删 |
| `~/Library/Caches/pnpm` | B | 🟢 | 80 MB | pnpm store 缓存 | `pnpm store prune` |
| `~/Library/Caches/Yarn` | B | 🟢 | 55 MB | Yarn 缓存 | `yarn cache clean` |
| `~/Library/Caches/pip` | B | 🟢 | 29 MB | pip 缓存 | `pip cache purge` |
| `~/Library/Caches/GeoServices` | A | 🟢 | 56 MB | 地图瓦片缓存 | 可删 |
| `~/.cache/zig`、`~/.cache/node` 等 | B | 🟢 | 0.3 GB | 各类工具缓存 | 可删 |
| `~/Library/Logs` | E | 🟢 | 62 MB | 用户级日志 | 可删 |
| `/private/var/log` | E | 🟡 | 93 MB | 系统日志 | `sudo` 清理，收益低可不做 |
| `~/Library/Containers/com.tencent.xinWeChat` | C | 🟡 | **8.6 GB** | 微信媒体（attach/video 按月池 + 孤儿） | 用微清原型逻辑：按月筛选 + 孤儿清理，**不动 db_storage** |
| `~/Library/Containers/com.docker.docker` | C | 🟡 | 2.9 GB | Docker 虚拟磁盘 | `docker system prune -a`（会删镜像） |
| `~/Library/Containers/com.tencent.meeting` | C | 🟡 | 555 MB | 腾讯会议缓存 | 应用内清理设置 |
| `~/Library/Application Support/微信开发者工具` | D | 🟡 | 1.1 GB | 旧项目缓存 | 确认项目无引用后清 |
| `~/Library/Application Support/Slack` | D | 🟡 | 1.0 GB | 含 Cache/Service Worker 子目录 | 只清 `Cache`/`Code Cache` 子目录 |
| `~/Library/Application Support/Google` | D | 🔴 | **8.5 GB** | Chrome 用户数据（书签/密码/扩展） | ⛔ 禁止删除 |
| `/Library/Caches` | E | 🟡 | 6.2 MB | 系统级缓存 | 需 sudo，收益极低 |

---

## 七、清理优先级建议（按收益/风险比）

| 顺序 | 动作 | 预计回收 | 耗时 | 风险 |
| --- | --- | --- | --- | --- |
| 1 | `~/.m2/repository` + `~/.gradle/caches` | 13.0 GB | 秒级 | 无（重拉即可） |
| 2 | Chrome 缓存（退出浏览器后） | 3.3 GB | 1 分钟 | 无 |
| 3 | `~/.npm` + pnpm/yarn/pip 缓存 | 3.3 GB | 秒级 | 无 |
| 4 | `go-build` + JetBrains + `~/.cache` 杂项 | 5.0 GB | 秒级 | 无（IDE 重建索引稍慢） |
| 5 | 微信媒体（按月 + 孤儿） | 3~5 GB | 10 分钟 | 低（废纸篓可恢复） |
| 6 | Docker 无用镜像 | 按需 1~2 GB | 5 分钟 | 中（镜像需重拉） |

**合计可回收约 29~31 GB**，可用空间从 25 GB 回到 55 GB 左右。

---

## 八、红线清单（请勿删除）

- `~/Library/Application Support/Google`（8.5 GB）— Chrome 书签/密码/扩展，删了等于重置浏览器
- `~/Library/Application Support/MobileSync` — iPhone 备份
- 微信 `xwechat_files/<wxid>/db_storage`（545 MB）— 聊天记录数据库
- `/System`、`/private/var/folders`、`/private/var/db` — 系统保留
- `~/Pictures/*.photoslibrary`、`~/Library/Mail`、`~/Library/Keychains` — 不可再生数据
- `~/Downloads`、`~/Desktop`、`~/Documents` — 你的文件，工具无权判断

---

## 九、执行与验证

**验证命令**（清理前后各跑一次对比）：

```bash
df -h /System/Volumes/Data          # 查看可用空间
du -sh ~/.m2 ~/Library/Caches ~/.cache ~/.npm 2>/dev/null   # 查看各目录占用
```

**安全删除原则**：

1. 优先走废纸篓（macOS 可装 `trash` CLI：`brew install trash`，用法 `trash <path>`）
2. 不用 `rm -rf` 直接删个人目录；确需命令删除时，务必逐条确认路径拼写
3. 一次只做一类，删完验证空间变化，再继续下一类
4. 涉及 `sudo` 的系统目录（`/Library/Caches`、`/private/var/log`）收益低，建议直接跳过

> 本档仅为清单与判定方法，不含自动执行脚本。如需脚本化，应逐步执行、每步列出将删内容并等待确认。
