import {type PolicySection} from "./types"
import {CodeBlock} from "@/components/ui/code-block"
import {
  DocsTable,
  DocsTableBody,
  DocsTableCell,
  DocsTableHead,
  DocsTableHeader,
  DocsTableRow,
} from "@/components/ui/docs-table"

export const DOCS_LAST_UPDATED = "2026-06-07"

/**
 * ------------------------------------------------------------------
 * API 文档
 * ------------------------------------------------------------------
 */
export const apiSections: PolicySection[] = [
  {
    value: "api-specs",
    title: "1. 接口规范与鉴权说明",
    content: (
      <div className="space-y-4 text-sm leading-relaxed">
        <div className="bg-muted/50 border border-border/50 rounded-lg px-3 py-2 mb-6">
          <p className="text-muted-foreground m-0">平台统一接口调用格式规范以及开发者访问令牌鉴权方式说明</p>
        </div>

        <h3 id="1-1-response-format" className="text-base md:text-lg font-semibold text-foreground mt-6 mb-3">1.1 统一响应格式</h3>
        <p>系统所有 API 接口均遵循标准 JSON 响应结构：</p>
        <DocsTable>
          <DocsTableHeader>
            <DocsTableRow>
              <DocsTableHead className="w-[120px]">字段</DocsTableHead>
              <DocsTableHead className="w-[100px]">类型</DocsTableHead>
              <DocsTableHead>说明</DocsTableHead>
            </DocsTableRow>
          </DocsTableHeader>
          <DocsTableBody>
            <DocsTableRow>
              <DocsTableCell className="font-mono text-xs">error_msg</DocsTableCell>
              <DocsTableCell>string</DocsTableCell>
              <DocsTableCell>错误信息。请求成功时为空字符串 `&quot;&quot;`，失败时包含错误详情描述。</DocsTableCell>
            </DocsTableRow>
            <DocsTableRow>
              <DocsTableCell className="font-mono text-xs">data</DocsTableCell>
              <DocsTableCell>any</DocsTableCell>
              <DocsTableCell>接口返回的具体数据内容。请求失败或无数据返回时为 `null`。</DocsTableCell>
            </DocsTableRow>
          </DocsTableBody>
        </DocsTable>

        <p className="mt-2">成功响应示例：</p>
        <CodeBlock
          code={`{
  "error_msg": "",
  "data": {
    "id": 1,
    "username": "ryan",
    "nickname": "Ryan"
  }
}`}
          language="json"
        />

        <p className="mt-2">失败响应示例：</p>
        <CodeBlock
          code={`{
  "error_msg": "用户密码错误",
  "data": null
}`}
          language="json"
        />

        <h3 id="1-2-authentication" className="text-base md:text-lg font-semibold text-foreground mt-6 mb-3">1.2 鉴权方式</h3>
        <p>除了公共公开接口（如登录、注册、配置）外，受保护的接口需要携带凭证才能正常访问：</p>
        <ul className="list-disc pl-5 space-y-2">
          <li><strong>Session 凭证：</strong>浏览器环境下支持利用常规 Session Cookie 会话保持登录。</li>
          <li><strong>AccessToken 令牌：</strong>供后台调用或第三方应用集成使用。客户端生成 API 访问令牌后，需要在请求头（Request Header）中携带以进行身份校验。</li>
        </ul>
        <div className="bg-muted border rounded-xl p-4 mt-2 space-y-2">
          <p className="font-bold text-xs">支持携带令牌的请求头格式（二选一）：</p>
          <ul className="list-disc pl-5 text-xs text-muted-foreground space-y-1">
            <li><code className="bg-muted-foreground/10 px-1 rounded text-[11px] font-mono">Authorization: Bearer at_xxx</code></li>
            <li><code className="bg-muted-foreground/10 px-1 rounded text-[11px] font-mono">X-Access-Token: at_xxx</code></li>
          </ul>
        </div>
      </div>
    ),
    children: [
      { value: "1-1-response-format", title: "1.1 统一响应格式" },
      { value: "1-2-authentication", title: "1.2 鉴权方式" },
    ]
  },
  {
    value: "auth-apis",
    title: "2. 用户与认证接口",
    content: (
      <div className="space-y-4 text-sm leading-relaxed">
        <h3 id="2-1-register" className="text-base md:text-lg font-semibold text-foreground mt-4 mb-2">2.1 用户注册</h3>
        <p><strong>接口：</strong>POST <code className="bg-muted px-1.5 py-0.5 rounded text-xs font-mono">/api/v1/user/register</code></p>
        <p><strong>说明：</strong>注册本地账户（在后台注册开关开启状态下）。</p>
        <DocsTable>
          <DocsTableHeader>
            <DocsTableRow>
              <DocsTableHead className="w-[120px]">参数</DocsTableHead>
              <DocsTableHead className="w-[80px]">必填</DocsTableHead>
              <DocsTableHead>类型</DocsTableHead>
              <DocsTableHead>说明</DocsTableHead>
            </DocsTableRow>
          </DocsTableHeader>
          <DocsTableBody>
            <DocsTableRow>
              <DocsTableCell className="font-mono text-xs">username</DocsTableCell>
              <DocsTableCell>是</DocsTableCell>
              <DocsTableCell>string</DocsTableCell>
              <DocsTableCell>用户名，必须唯一且无空格。</DocsTableCell>
            </DocsTableRow>
            <DocsTableRow>
              <DocsTableCell className="font-mono text-xs">password</DocsTableCell>
              <DocsTableCell>是</DocsTableCell>
              <DocsTableCell>string</DocsTableCell>
              <DocsTableCell>密码，长度必须大于等于 8 位。</DocsTableCell>
            </DocsTableRow>
            <DocsTableRow>
              <DocsTableCell className="font-mono text-xs">nickname</DocsTableCell>
              <DocsTableCell>否</DocsTableCell>
              <DocsTableCell>string</DocsTableCell>
              <DocsTableCell>昵称。未传时默认与用户名一致。</DocsTableCell>
            </DocsTableRow>
          </DocsTableBody>
        </DocsTable>

        <h3 id="2-2-login" className="text-base md:text-lg font-semibold text-foreground mt-6 mb-2">2.2 密码登录</h3>
        <p><strong>接口：</strong>POST <code className="bg-muted px-1.5 py-0.5 rounded text-xs font-mono">/api/v1/user/login</code></p>
        <p><strong>说明：</strong>通过常规用户名密码进行登录校验，成功后建立 Session Cookie 会话。</p>
        <DocsTable>
          <DocsTableHeader>
            <DocsTableRow>
              <DocsTableHead className="w-[120px]">参数</DocsTableHead>
              <DocsTableHead className="w-[80px]">必填</DocsTableHead>
              <DocsTableHead>类型</DocsTableHead>
              <DocsTableHead>说明</DocsTableHead>
            </DocsTableRow>
          </DocsTableHeader>
          <DocsTableBody>
            <DocsTableRow>
              <DocsTableCell className="font-mono text-xs">username</DocsTableCell>
              <DocsTableCell>是</DocsTableCell>
              <DocsTableCell>string</DocsTableCell>
              <DocsTableCell>用户名</DocsTableCell>
            </DocsTableRow>
            <DocsTableRow>
              <DocsTableCell className="font-mono text-xs">password</DocsTableCell>
              <DocsTableCell>是</DocsTableCell>
              <DocsTableCell>string</DocsTableCell>
              <DocsTableCell>密码</DocsTableCell>
            </DocsTableRow>
          </DocsTableBody>
        </DocsTable>

        <h3 id="2-3-logout" className="text-base md:text-lg font-semibold text-foreground mt-6 mb-2">2.3 退出登录</h3>
        <p><strong>接口：</strong>GET <code className="bg-muted px-1.5 py-0.5 rounded text-xs font-mono">/api/v1/user/logout</code></p>
        <p><strong>说明：</strong>销毁当前会话 Cookie 并退出登录状态。</p>

        <h3 id="2-4-profile" className="text-base md:text-lg font-semibold text-foreground mt-6 mb-2">2.4 获取个人资料</h3>
        <p><strong>接口：</strong>GET <code className="bg-muted px-1.5 py-0.5 rounded text-xs font-mono">/api/v1/user/self</code></p>
        <p><strong>说明：</strong>获取当前登录账户的基本数据模型（包含 ID、角色、昵称等）。</p>
      </div>
    ),
    children: [
      { value: "2-1-register", title: "2.1 用户注册" },
      { value: "2-2-login", title: "2.2 密码登录" },
      { value: "2-3-logout", title: "2.3 退出登录" },
      { value: "2-4-profile", title: "2.4 获取个人资料" },
    ]
  },
  {
    value: "token-apis",
    title: "3. 个人访问令牌 (AccessToken) 接口",
    content: (
      <div className="space-y-4 text-sm leading-relaxed">
        <div className="bg-muted/50 border border-border/50 rounded-lg px-3 py-2 mb-4">
          <p className="text-muted-foreground m-0">AccessToken 管理相关接口均要求通过 Session 登录后调用，支持普通用户权限。</p>
        </div>

        <h3 id="3-1-list-token" className="text-base md:text-lg font-semibold text-foreground mt-4 mb-2">3.1 获取令牌列表</h3>
        <p><strong>接口：</strong>GET <code className="bg-muted px-1.5 py-0.5 rounded text-xs font-mono">/api/v1/user/access-tokens</code></p>
        <p><strong>说明：</strong>查询当前用户已创建的所有令牌详情（令牌明文已被脱敏）。</p>

        <h3 id="3-2-create-token" className="text-base md:text-lg font-semibold text-foreground mt-6 mb-2">3.2 新建访问令牌</h3>
        <p><strong>接口：</strong>POST <code className="bg-muted px-1.5 py-0.5 rounded text-xs font-mono">/api/v1/user/access-tokens</code></p>
        <p><strong>参数：</strong>JSON Body <code className="bg-muted px-1.5 rounded text-xs font-mono">{`{"name": "token名称", "is_admin": false}`}</code></p>
        <p><strong>说明：</strong>生成一个全新访问令牌。返回体中包含一次性明文 Token，切勿遗失。</p>
        <p className="mt-1 text-xs text-muted-foreground"><code className="bg-muted px-1 rounded">is_admin</code>（可选，默认 <code className="bg-muted px-1 rounded">false</code>）：是否赋予令牌管理员权限，仅管理员用户可设置。非管理员令牌无法访问 <code className="bg-muted px-1 rounded">/admin/**</code> 端点。</p>
        <p className="mt-2">成功返回样例：</p>
        <CodeBlock
          code={`{
  "error_msg": "",
  "data": {
    "token": "at_628d022b7a95e26bcd8b29c9...",
    "record": {
      "id": 5,
      "user_id": 1,
      "name": "my-dev-key",
      "masked_token": "at_628d...29c9",
      "is_admin": false,
      "created_at": "2026-06-07T21:30:00+08:00"
    }
  }
}`}
          language="json"
        />

        <h3 id="3-3-delete-token" className="text-base md:text-lg font-semibold text-foreground mt-6 mb-2">3.3 撤销/删除令牌</h3>
        <p><strong>接口：</strong>DELETE <code className="bg-muted px-1.5 py-0.5 rounded text-xs font-mono">/api/v1/user/access-tokens/:id</code></p>
        <p><strong>说明：</strong>通过 ID 物理删除对应访问令牌，该令牌将立即失效。</p>

        <h3 id="3-4-rotate-token" className="text-base md:text-lg font-semibold text-foreground mt-6 mb-2">3.4 轮换令牌密钥</h3>
        <p><strong>接口：</strong>POST <code className="bg-muted px-1.5 py-0.5 rounded text-xs font-mono">/api/v1/user/access-tokens/:id/rotate</code></p>
        <p><strong>说明：</strong>轮换指定令牌的物理密钥值。系统将废弃原有密钥，返回新生成的明文 Token，令牌名称与 ID 保持一致。</p>
      </div>
    ),
    children: [
      { value: "3-1-list-token", title: "3.1 获取令牌列表" },
      { value: "3-2-create-token", title: "3.2 新建访问令牌" },
      { value: "3-3-delete-token", title: "3.3 撤销/删除令牌" },
      { value: "3-4-rotate-token", title: "3.4 轮换令牌密钥" },
    ]
  },
  {
    value: "config-apis",
    title: "4. 公共配置与管理接口",
    content: (
      <div className="space-y-4 text-sm leading-relaxed">
        <h3 id="4-1-public-config" className="text-base md:text-lg font-semibold text-foreground mt-4 mb-2">4.1 公共系统配置</h3>
        <p><strong>接口：</strong>GET <code className="bg-muted px-1.5 py-0.5 rounded text-xs font-mono">/api/v1/config/public</code></p>
        <p><strong>说明：</strong>无感获取当前系统配置表中公共可见的键值集合。供前端页面动态渲染使用。</p>
        <p className="mt-2">返回数据结构样例：</p>
        <CodeBlock
          code={`{
  "error_msg": "",
  "data": {
    "site_name": "OpenFlare",
    "registration_enabled": "false",
    "password_login_enabled": "true",
    "password_register_enabled": "false",
    "cap_login_enabled": "true",
    "oidc_login_enabled": "true"
  }
}`}
          language="json"
        />

        <h3 id="4-2-admin-configs" className="text-base md:text-lg font-semibold text-foreground mt-6 mb-2">4.2 系统配置项 CRUD (管理员)</h3>
        <p><strong>说明：</strong>用于在后台对 `system_configs` 配置进行动态变更，要求管理员权限会话调用。</p>
        <ul className="list-disc pl-5 space-y-2">
          <li><strong>获取配置列表：</strong>GET <code className="bg-muted px-1 rounded text-xs font-mono">/api/v1/admin/system-configs?type=system</code></li>
          <li><strong>新建配置项：</strong>POST <code className="bg-muted px-1 rounded text-xs font-mono">/api/v1/admin/system-configs</code></li>
          <li><strong>修改指定配置值：</strong>PUT <code className="bg-muted px-1 rounded text-xs font-mono">/api/v1/admin/system-configs/:key</code></li>
          <li><strong>删除配置项：</strong>DELETE <code className="bg-muted px-1 rounded text-xs font-mono">/api/v1/admin/system-configs/:key</code></li>
        </ul>
      </div>
    ),
    children: [
      { value: "4-1-public-config", title: "4.1 公共系统配置" },
      { value: "4-2-admin-configs", title: "4.2 系统配置项 CRUD (管理员)" },
    ]
  }
]
