## 下载

| 文件 | 架构 | 签名 |
|---|---|---|
| `@ASSET@` | @ARCH@ | ad-hoc（未公证） |

解压得到「磁盘清理.app」，拖进 `/Applications` 即可。**首次打开会被 Gatekeeper 拦下**，因为该包没有 Apple 公证，任选一种方式放行：

- Finder 里右键 →「打开」，再确认一次；或
- `xattr -dr com.apple.quarantine "/Applications/磁盘清理.app"`

架构不匹配（不是 @ARCH@）请从源码构建——本工具不做交叉编译，产物与构建机架构一致。

## 源码构建

```bash
git clone https://github.com/kennethfan/disk-cleaner.git
cd disk-cleaner
make build                    # 产出 bin/disk-cleaner，CLI 与图形界面同一二进制
./bin/disk-cleaner            # 无参数启动即打开图形界面
```

## 环境要求与提示

- macOS 11（Big Sur）及以上
- 首次运行会弹权限提示。想在结果里看到 `~/Library/Mail`、`~/Library/Messages` 的占用，需到「系统设置 → 隐私与安全性 → 完全磁盘访问权限」里手动添加终端或本应用并重启它；不加也能用，只是这些目录被跳过、大小统计偏小
- 删除 = 移入废纸篓，可随时恢复；`/System`、`/usr`、`~/.ssh`、`~/Documents` 等路径受白名单保护，删不掉
- 废纸篓内容与 APFS 快照不主动删除，只提示需要 sudo 的手动命令

## 本版变更
