# SSO 单点登录配置

通过标准 OIDC Discovery 接入 Google、Keycloak、authentik、Logto 或 Casdoor 等第三方身份提供商。

## 1. 前置准备

| 准备项 | 说明 |
| :--- | :--- |
| 服务器访问地址 | 在 **系统管理** > **系统设置** > **系统配置** > **通用设置** 配置，须与用户访问协议、域名和端口一致 |
| 认证源名称 | 唯一标识（如 `company-oidc`），仅允许字母、数字、`-` 与 `_`，且以字母或数字开头 |
| Client ID / Secret | 第三方身份提供商分配的应用凭证 |
| OIDC Discovery URL | 通常以 `/.well-known/openid-configuration` 结尾 |

所有认证源统一使用同一回调地址（Redirect URI），无需拼接认证源名称：

```text
<服务器访问地址>/login
```

## 2. 配置 OIDC 认证源

1. 在身份提供商创建 Web / Confidential 客户端并将 Redirect URI 设为 `<服务器访问地址>/login`。
2. 进入 **系统管理** > **系统设置** > **安全设置** > **认证源管理**，点击 **添加认证源** 并选择 `OIDC`。
3. 填写名称、展示名、Client ID、Client Secret、Discovery URL 与 Scope（默认 `openid profile email`），保存并启用；编辑时密钥留空可保留原值。

## 3. 绑定行为与排查

| 场景 | 行为 |
| :--- | :--- |
| 外部身份已绑定 | 直接登录对应本地账号 |
| 已登录用户发起授权 | 将外部身份绑定到当前本地账号 |
| 未绑定且开放注册 | 自动创建普通用户并完成绑定 |
| 未绑定且关闭注册 | 引导输入已有本地账号密码完成一次性绑定 |

外部身份通过 `auth_source + sub` 绑定本地用户，系统升级会保留现有绑定关系。

| 常见故障 | 处理方式 |
| :--- | :--- |
| 报 `invalid_scope` | 核对认证源 Scope 或在身份提供商允许 `openid profile email` |
| Redirect URI 不匹配 | 校验回调 URL 与 `<服务器访问地址>/login` 的协议、域名、端口和路径完全一致 |
| 回调后报错 | 查看浏览器 `POST /api/v1/oauth/callback` 响应并从 `/login` 重新发起登录，勿刷新过期回调链接 |
| 登录页无 OIDC 按钮 | 确认 **安全设置** 已开启 **允许 OIDC 登录**，且认证源已启用并保存完整凭证 |
| 列表中不显示 Client Secret | 正常行为，接口不回传已保存的明文密钥 |
