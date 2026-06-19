import {type PolicySection} from "./types"
import {CodeBlock} from "@/components/ui/code-block"

/**
 * ------------------------------------------------------------------
 * 使用指南 (How To Use)
 * ------------------------------------------------------------------
 */
export const howToUseSections: PolicySection[] = [
  {
    value: "quick-start",
    title: "1. 快速开始",
    content: (
      <div className="space-y-4 text-sm leading-relaxed">
        <div className="bg-muted/50 border border-border/50 rounded-lg px-3 py-2 mb-6">
          <p className="text-muted-foreground m-0">为开发者提供通用全栈开发脚手架 (Boilerplate) 平台使用说明</p>
        </div>
        <ul className="list-disc pl-5 space-y-2">
          <li><strong>架构底座：</strong>Go (Gin + GORM + Redis + Asynq) 后端 + React (Next.js 16 + Tailwind CSS 4 + Shadcn UI) 前端</li>
          <li><strong>认证体系：</strong>支持本地常规账号密码注册登录 + 第三方自定义 OIDC (OAuth2) 认证源绑定</li>
          <li><strong>访问令牌：</strong>提供开发者个人 AccessToken (API Key)，用于通过 Http Header 鉴权直接调用系统 API</li>
          <li><strong>可观测性：</strong>集成 Zap 结构化日志与 OpenTelemetry 全链路 Tracing 追踪</li>
        </ul>
      </div>
    )
  },
  {
    value: "auth-security",
    title: "2. 身份认证与安全设置",
    content: (
      <div className="space-y-4 text-sm leading-relaxed">
        <p>平台采用混合式身份认证，满足不同的部署和业务场景需求：</p>
        <h3 id="2-1-login" className="text-base md:text-lg font-semibold text-foreground mt-4 mb-2">2.1 常规账号密码认证</h3>
        <ul className="list-disc pl-4 md:pl-5 space-y-2">
          <li><strong>自主注册与密码登录：</strong>支持普通用户通过用户名及密码直接进行注册与会话建立，密码在后端采用 <code className="bg-muted px-1.5 py-0.5 rounded text-xs font-mono">bcrypt</code> 高强度加盐哈希存储。</li>
          <li><strong>系统开关控制：</strong>管理员可在后台配置动态开关，随时禁用自主密码注册或密码登录，以转为纯第三方认证模式。</li>
        </ul>

        <h3 id="2-2-oidc" className="text-base md:text-lg font-semibold text-foreground mt-4 mb-2">2.2 第三方 OIDC 认证源</h3>
        <p>用户可以在个人资料页面关联绑定外部授权账户：</p>
        <ol className="list-decimal pl-4 md:pl-5 space-y-1">
          <li>进入 <strong>设置 / 个人资料</strong> 页面。</li>
          <li>在“第三方账号绑定”栏目下查看当前绑定的账号，或点击未绑定的可用 OIDC 认证源直接触发 OAuth2 绑定流。</li>
          <li>绑定成功后，用户在登录界面可直接点击 OIDC 登录按钮实现快捷跳转。</li>
        </ol>
      </div>
    ),
    children: [
      { value: "2-1-login", title: "2.1 常规账号密码认证" },
      { value: "2-2-oidc", title: "2.2 第三方 OIDC 认证源" },
    ]
  },
  {
    value: "access-token",
    title: "3. 个人访问令牌 (AccessToken) 接口对接",
    content: (
      <div className="space-y-4 text-sm leading-relaxed">
        <p>为便于开发者或第三方工具直接调用系统 API，平台提供个人访问令牌管理功能。</p>
        <h3 id="3-1-generation" className="text-base md:text-lg font-semibold text-foreground mt-4 mb-2">3.1 令牌生成与存储规范</h3>
        <ul className="list-disc pl-4 md:pl-5 space-y-2">
          <li><strong>一次性明文展示：</strong>创建令牌时生成的随机明文 Token 值 (形如 <code className="bg-muted px-1.5 py-0.5 rounded text-xs font-mono">at_xxx</code>) 仅会在弹窗中展示一次。请立即复制保存，关闭弹窗后系统将无法重新获取。</li>
          <li><strong>安全哈希存储：</strong>数据库仅存储 Token 的 SHA-256 哈希指纹，即使数据库泄漏，攻击者也无法通过摘要逆向恢复令牌原文。</li>
        </ul>

        <h3 id="3-2-usage" className="text-base md:text-lg font-semibold text-foreground mt-4 mb-2">3.2 携带 Header 进行 API 调用</h3>
        <p>您可以凭借保存的明文 Token 随时调用系统开放接口，系统认证支持以下两种 Http 请求头携带方式之一：</p>
        <div className="space-y-2">
          <p className="font-semibold text-xs text-muted-foreground">方式一：Authorization Bearer 头</p>
          <CodeBlock
            code={`GET /api/v1/user/self HTTP/1.1
Host: localhost:3000
Authorization: Bearer at_628d022b7a95e26b...`}
            language="http"
          />
        </div>
        <div className="space-y-2">
          <p className="font-semibold text-xs text-muted-foreground">方式二：X-Access-Token 自定义头</p>
          <CodeBlock
            code={`GET /api/v1/user/self HTTP/1.1
Host: localhost:3000
X-Access-Token: at_628d022b7a95e26b...`}
            language="http"
          />
        </div>
      </div>
    ),
    children: [
      { value: "3-1-generation", title: "3.1 令牌生成与存储规范" },
      { value: "3-2-usage", title: "3.2 携带 Header 进行 API 调用" },
    ]
  },
  {
    value: "config-system",
    title: "4. 动态系统配置项",
    content: (
      <div className="space-y-4 text-sm leading-relaxed">
        <p>平台内置了完备的 KV 配置管理模块，允许管理员动态调整系统运行状态：</p>
        <ul className="list-disc pl-4 md:pl-5 space-y-2">
          <li><strong>缓存读取加速：</strong>配置加载基于 GORM 读取数据库，并辅以 Redis Hash 结构进行多层缓存加速，大幅降低配置查询耗时。</li>
          <li><strong>核心系统配置项说明：</strong>
            <ul className="list-[circle] pl-5 mt-1 space-y-1 text-muted-foreground">
              <li><code className="bg-muted px-1 rounded text-xs font-mono">site_name</code>：平台展示名称</li>
              <li><code className="bg-muted px-1 rounded text-xs font-mono">password_login_enabled</code>：密码登录开关</li>
              <li><code className="bg-muted px-1 rounded text-xs font-mono">registration_enabled</code>：用户自主注册开关</li>
              <li><code className="bg-muted px-1 rounded text-xs font-mono">max_api_keys_per_user</code>：普通用户创建令牌的最大数限制 (默认5)</li>
            </ul>
          </li>
        </ul>
      </div>
    )
  },
  {
    value: "worker-scheduler",
    title: "5. 异步任务与定时调度",
    content: (
      <div className="space-y-4 text-sm leading-relaxed">
        <p>项目借助 Cobra 实现多命令入口分发，通过独立部署 Worker 服务解耦复杂计算或高延迟IO：</p>
        <ul className="list-disc pl-4 md:pl-5 space-y-2">
          <li><strong>定时任务派发 (CMD scheduler)：</strong>负责按 Cron 表达式配置，定时将待执行任务推送到 Redis 队列中。</li>
          <li><strong>多优先级 Worker (CMD worker)：</strong>基于 Asynq 驱动，按优先级拉取任务并并发调度执行（如定时清理临时上传目录、同步外部系统日志等）。</li>
        </ul>
      </div>
    )
  },
  {
    value: "tracing-metrics",
    title: "6. 链路追踪与结构化日志",
    content: (
      <div className="space-y-4 text-sm leading-relaxed">
        <p>为了保障分布式微服务架构下的可观测性，平台接入了高级监控组件：</p>
        <ul className="list-disc pl-4 md:pl-5 space-y-2">
          <li><strong>OpenTelemetry Tracing：</strong>自动传递 Tracing 上下文，所有经由 Gin 中间件、外部请求或 GORM 数据库的事务操作都将带有全局唯一的 Span，用于排查链路耗时或调用异常。</li>
          <li><strong>Zap 结构化日志：</strong>将后端控制台或日志输出格式统一规范化为 JSON，方便与 ELK、Loki 等日志收集分析工具无缝对接。</li>
        </ul>
      </div>
    )
  }
]
