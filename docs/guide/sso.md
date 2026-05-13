# SSO 登录配置

OpenFlare 支持通过认证源配置第三方登录入口。当前支持 GitHub OAuth 与标准 OIDC Provider，例如 Logto、authentik、Keycloak、Casdoor 等。

认证源配置完成并启用后，会显示在登录页的第三方账号登录区域。用户可以通过第三方账号登录，也可以在已登录状态下把第三方账号绑定到当前本地账号。

## 使用前准备

你需要先准备：

| 项目 | 说明 |
| --- | --- |
| OpenFlare 访问地址 | 用户浏览器实际访问的地址，例如 `https://openflare.example.com` |
| 认证源名称 | OpenFlare 内部唯一标识，例如 `github`、`company-oidc` |
| Client ID | 第三方平台创建应用后提供 |
| Client Secret | 第三方平台创建应用后提供 |
| OIDC Discovery URL | 仅 OIDC 需要，例如 `https://idp.example.com/.well-known/openid-configuration` |

认证源名称只能包含字母、数字、短横线或下划线，并且必须以字母或数字开头。认证源名称会出现在回调地址中，保存后如需修改名称，也必须同步修改第三方平台中的回调地址。

## 回调地址

第三方平台中的 Redirect URI / Callback URL 填写格式为：

```text
<OpenFlare 访问地址>/oauth/<认证源名称>
```

示例：

```text
https://openflare.example.com/oauth/github
https://openflare.example.com/oauth/company-oidc
```

在管理端新增或修改认证源时，表单会根据当前浏览器访问地址和你输入的认证源名称自动显示应填写的回调地址。

## 配置 GitHub 登录

1. 在 GitHub 创建 OAuth App。
2. `Homepage URL` 填写 OpenFlare 访问地址。
3. `Authorization callback URL` 填写 OpenFlare 显示的回调地址，例如 `https://openflare.example.com/oauth/github`。
4. 复制 GitHub 提供的 Client ID 和 Client Secret。
5. 登录 OpenFlare 管理端，进入“设置 -> 系统设置 -> 配置认证源”。
6. 新增认证源，类型选择 `GitHub`。
7. 填写认证源名称、展示名称、Client ID、Client Secret。
8. Scope 默认使用 `user:email`，通常无需修改。
9. 保存并启用认证源。

启用后，登录页会显示对应的 GitHub 登录按钮。

## 配置 OIDC 登录

1. 在 OIDC Provider 中创建应用或客户端。
2. 应用类型选择 Web / Confidential Client。
3. Redirect URI / Callback URL 填写 OpenFlare 显示的回调地址，例如 `https://openflare.example.com/oauth/company-oidc`。
4. 复制 Client ID 和 Client Secret。
5. 获取 Provider 的 Discovery URL，通常以 `/.well-known/openid-configuration` 结尾。
6. 登录 OpenFlare 管理端，进入“设置 -> 系统设置 -> 配置认证源”。
7. 新增认证源，类型选择 `OIDC`。
8. 填写认证源名称、展示名称、Client ID、Client Secret、OIDC Discovery URL。
9. Scope 默认使用 `openid profile email`。如果 Provider 限制了 scope，请按 Provider 允许的值调整。
10. 保存并启用认证源。

启用后，登录页会显示对应的 OIDC 登录按钮。

## 登录与绑定行为

第三方账号回到 OpenFlare 后按以下规则处理：

| 场景 | 行为 |
| --- | --- |
| 第三方账号已绑定本地用户 | 直接登录 |
| 用户已登录并发起第三方授权 | 绑定到当前本地用户 |
| 第三方账号未绑定，且允许注册 | 自动创建普通用户并绑定 |
| 第三方账号未绑定，且关闭注册 | 要求输入已有本地账号密码完成绑定 |

如果希望只允许已有用户使用 SSO，可以关闭用户注册。未绑定的第三方账号会进入绑定已有账号流程。

## 修改认证源

修改认证源时，Client Secret 输入框留空表示保留已有密钥；填写新值则会覆盖保存。

如果修改了认证源名称，回调地址也会随之变化。你必须到第三方平台同步修改 Redirect URI / Callback URL，否则第三方平台会拒绝回调或返回错误。

## 常见问题

### 返回 `invalid_scope`

说明第三方平台不允许当前配置的 Scope。OIDC 默认 Scope 是 `openid profile email`，GitHub 默认 Scope 是 `user:email`。请到认证源编辑页调整 Scope，或在第三方平台放行对应 Scope。

### 提示回调地址不匹配

检查第三方平台中配置的 Redirect URI / Callback URL 是否与 OpenFlare 表单提示完全一致。协议、域名、端口和路径都必须一致。

### 登录页没有显示第三方登录按钮

检查认证源是否已启用，并确认 Client ID 和 Client Secret 已保存。启用认证源前，OpenFlare 会校验这些字段。

### 已经保存 Client Secret，但列表不显示明文

这是预期行为。OpenFlare 不会通过 API 回显 Client Secret，只显示该密钥是否已配置。
