# SSO 登录配置

你会学到：如何为 OpenFlare 配置 OIDC 第三方登录入口、填写回调地址，以及第三方账号如何绑定本地用户。

OpenFlare 通过 OIDC 认证源接入第三方登录。任意提供标准 OIDC Discovery 的服务（如 Google、Keycloak、authentik、Logto、Casdoor 等）都可以接入。

认证源配置完成并启用后，会显示在登录页的第三方账号登录区域。用户可以通过第三方账号登录，也可以在已登录状态下把第三方账号绑定到当前本地账号。

## 使用前准备

| 项目 | 说明 |
| --- | --- |
| 服务器访问地址 | 在管理端「系统设置」->「系统设置」选项卡 ->「通用设置」中配置，须与用户浏览器实际访问的地址一致（协议、域名、端口） |
| 认证源名称 | OpenFlare 内部唯一标识，例如 `company-oidc` |
| Client ID | 第三方平台创建应用后提供 |
| Client Secret | 第三方平台创建应用后提供 |
| OIDC Discovery URL | 例如 `https://idp.example.com/.well-known/openid-configuration` |

认证源名称只能包含字母、数字、短横线或下划线，并且必须以字母或数字开头。

## 回调地址

第三方平台中的 Redirect URI / Callback URL 固定填写：

```text
<服务器访问地址>/login
```

例如服务器访问地址为 `https://openflare.example.com` 时：

```text
https://openflare.example.com/login
```

回调地址只与「服务器访问地址」相关，不包含认证源名称。第三方平台授权完成后会跳转到该地址，OpenFlare 登录页携带授权码完成登录或绑定。

## 配置 OIDC 登录

1. 在 OIDC Provider 中创建应用或客户端，应用类型选择 Web / Confidential Client。
2. Redirect URI / Callback URL 填写 `<服务器访问地址>/login`。
3. 复制 Client ID 和 Client Secret。
4. 获取 Provider 的 Discovery URL，通常以 `/.well-known/openid-configuration` 结尾。
5. 登录 OpenFlare 管理端，进入左侧导航 **「系统设置」**，选择 **「安全设置」** 选项卡，在 **「认证源管理」** 栏目中新增认证源。
6. 类型选择 `OIDC`，填写认证源名称、展示名称、Client ID、Client Secret、OIDC Discovery URL。
7. Scope 默认使用 `openid profile email`。如果 Provider 限制了 scope，请按 Provider 允许的值调整。
8. 保存并启用认证源。

启用后，登录页会显示对应的第三方登录按钮。

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

修改认证源名称不会影响回调地址，无需同步修改第三方平台配置。

## 常见问题

### 返回 `invalid_scope`

说明第三方平台不允许当前配置的 Scope。OIDC 默认 Scope 是 `openid profile email`。请到认证源编辑页调整 Scope，或在第三方平台放行对应 Scope。

### 提示回调地址不匹配

检查第三方平台中配置的 Redirect URI / Callback URL 是否与 `<服务器访问地址>/login` 完全一致。协议、域名、端口和路径都必须一致。

### 登录页没有显示第三方登录按钮

检查认证源是否已启用，并确认 Client ID 和 Client Secret 已保存。启用认证源前，OpenFlare 会校验这些字段。

### 已经保存 Client Secret，但列表不显示明文

这是预期行为。OpenFlare 不会通过 API 回显 Client Secret，只显示该密钥是否已配置。
