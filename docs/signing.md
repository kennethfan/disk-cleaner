# 签名与公证指南

本文说明如何为 `磁盘清理.app` 做正式签名与公证，使其在他人 Mac 上无需手动放行即可打开。

## 为什么需要签名与公证

macOS 的 Gatekeeper 会拦截未签名或未公证的应用：

| 状态 | 双击表现 | `spctl` 结果 |
|---|---|---|
| 未签名 | 无反应 / 提示「已损坏」 | `rejected`，`no usable signature` |
| ad-hoc 签名 | 本机可运行，换机器仍被拦 | `rejected`（无 Team ID） |
| Developer ID 签名 | 首次打开提示「无法验证开发者」 | `rejected` 或 `accepted`（视网络校验） |
| 签名 + 公证 + 装订 | 直接打开，无提示 | `accepted`，`source=Notarized Developer ID` |

当前项目默认走 **ad-hoc 签名**，只够本机自用。要让别人也能双击打开，需要 Apple 开发者账号。

## 前置条件

1. **Apple Developer Program 会员**（个人或公司，约 ¥688/年）
2. 在 [developer.apple.com](https://developer.apple.com/account/resources/certificates) 创建 **Developer ID Application** 证书
3. 把证书导入钥匙串，确认可用：

```bash
security find-identity -v -p codesigning
# 应输出类似：
# 1) ABC123... "Developer ID Application: Your Name (TEAMID)"
```

4. 获取 **Team ID**（在账号 Membership 页面，10 位字母数字）
5. 生成 **App 专用密码**：访问 [appleid.apple.com](https://appleid.apple.com) → 登录与安全 → App 专用密码 → 生成
   > 注意：这不是你的 Apple ID 登录密码，而是形如 `abcd-efgh-ijkl-mnop` 的专用密码。

## 步骤一：正式签名

```bash
make app SIGN_IDENTITY="Developer ID Application: Your Name (TEAMID)"
```

关键点：`--options runtime` 启用 **Hardened Runtime**，这是公证的硬性要求。Makefile 已包含该参数。

验证：

```bash
make verify-sign
# 期望看到：
#   flags=0x10000(runtime)
#   TeamIdentifier=TEAMID
#   valid on disk
#   satisfies its Designated Requirement
```

## 步骤二：保存公证凭据（只需一次）

```bash
make notary-setup \
  APPLE_ID="you@example.com" \
  TEAM_ID="ABCDE12345" \
  APP_PASSWORD="abcd-efgh-ijkl-mnop"
```

凭据会存入钥匙串的 profile `disk-cleaner-notary`，后续公证无需重复输入。

## 步骤三：提交公证并装订

```bash
make notarize
```

流程：打包 ZIP → 提交 Apple → 等待审核（通常 1-5 分钟）→ 装订票据到 `.app`。

成功标志：

```bash
xcrun stapler validate "dist/磁盘清理.app"
# 输出: The validate action worked!
```

最终复核：

```bash
make verify-sign
# Gatekeeper 评估应变为:
#   accepted
#   source=Notarized Developer ID
```

## 常见问题

### `spctl` 一直 rejected，但签名有效

检查是否**装订了票据**。公证后必须执行 `stapler staple`，否则离线环境下仍会被拦。

```bash
xcrun stapler validate "dist/磁盘清理.app"
```

### 公证失败：`The executable does not have the hardened runtime enabled`

签名时漏了 `--options runtime`。Makefile 的 `SIGN_IDENTITY` 分支已包含，若手写命令需自行加上。

### 公证失败：`The signature of the binary is invalid`

bundle 内有文件在签名后被修改。顺序必须是：**先放好所有资源（含图标），再签名**。Makefile 已保证此顺序。

### 公证失败：`You must first sign the relevant documents with a Developer ID certificate`

用的是 ad-hoc 或自签名证书，不是 Developer ID。

### 换机器后仍提示「无法验证开发者」

- 确认票据已装订：`xcrun stapler validate`
- 用户首次打开时若仍被拦，可在「系统设置 → 隐私与安全性」中点「仍要打开」，这只需一次

## 本机当前状态

```bash
security find-identity -v -p codesigning
#   0 valid identities found
```

**本机没有代码签名证书**，因此：
- `make app` 只能产出 ad-hoc 签名版本
- `make notarize` 会失败（缺 Developer ID 证书与凭据）

拿到证书后，按上面三步操作即可，Makefile 与 Info.plist 无需改动。

## 不公证的分发方式

如果不想买开发者账号，可选：

1. **源码分发** —— 让用户自行 `make app`，本机构建的应用不受 Gatekeeper 拦截
2. **手动放行指引** —— 用户首次打开时右键 →「打开」，或在隐私设置中放行
3. **Homebrew Cask** —— 仍需公证，否则 cask 安装后同样被拦

对本项目这类需要访问用户磁盘的清理工具，**建议正式签名+公证**：用户看到「未验证开发者」时会更警惕，反而降低信任。
