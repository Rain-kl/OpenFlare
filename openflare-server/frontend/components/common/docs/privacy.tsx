import {type PolicySection} from "./types"

/**
 * ------------------------------------------------------------------
 * 隐私政策 (PRIVACY POLICY)
 * ------------------------------------------------------------------
 */
export const privacySections: PolicySection[] = [
  {
    value: "collection-details",
    title: "1. 详细信息收集说明",
    content: (
      <div className="space-y-4 text-sm leading-relaxed">
        <p>我们仅遵循合法、正当、必要的原则，收集为您提供服务所必需的信息。我们的数据收集范围严格限制如下：</p>
        <div className="space-y-3">
          <div>
            <span className="font-semibold text-foreground">1.1 身份鉴权信息：</span>
            <p className="mt-1 text-muted-foreground">当您通过本地账号注册或绑定第三方 OIDC 认证源登录时，我们会收集您的用户名、关联的邮箱、加密后的密码哈希指纹及关联的第三方 OpenID 标识符。<strong>我们不强制要求绑定手机号、身份证件或任何真实社会信用实体信息。</strong></p>
          </div>
          <div>
            <span className="font-semibold text-foreground">1.2 服务与接口日志信息：</span>
            <p className="mt-1 text-muted-foreground">为保障系统运行安全及满足安全审计要求，我们会自动收集您的操作日志，包括 IP 地址、访问日期和时间、个人访问令牌 API 调用历史记录、User-Agent（浏览器/设备/请求工具类型）。</p>
          </div>
        </div>
      </div>
    ),
  },
  {
    value: "storage-security",
    title: "2. 数据存储与安全保护",
    content: (
      <div className="space-y-4 text-sm leading-relaxed">
        <p>我们深知数据安全的重要性，并采取业界领先的技术措施保护您的数据：</p>
        <ul className="list-disc pl-4 md:pl-5 space-y-2">
          <li><strong>存储安全：</strong>用户数据独立存储于专用的云数据库或本地容器化持久层中，仅供授权系统应用挂载读取。</li>
          <li><strong>加密技术：</strong>敏感的密码指纹和个人访问令牌哈希在数据库中均采用高强度算法加密存储。API 数据传输链路强制使用 SSL/TLS 进行安全加密。</li>
          <li><strong>访问控制：</strong>我们实行严格的最小权限原则（Least Privilege），平台数据不会对外共享，所有系统级内部运维操作均有审计日志可查。</li>
        </ul>
      </div>
    ),
  },
  {
    value: "usage-rules",
    title: "3. 信息使用规范",
    content: (
      <div className="space-y-4 text-sm leading-relaxed">
        <p>我们收集的信息将仅用于以下目的：</p>
        <ul className="list-disc pl-4 md:pl-5 space-y-1">
          <li><strong>身份识别：</strong>用于确认您的注册身份，展示您的个人中心及设置页面数据。</li>
          <li><strong>业务功能：</strong>用于识别并处理您的个人访问令牌 API 鉴权指令。</li>
          <li><strong>安全风控：</strong>利用 IP 及行为日志进行接口反作弊、反暴力破解分析，保障后台服务稳定性。</li>
        </ul>
        <p><strong>禁止用途：</strong>我们承诺<strong>绝不</strong>将您的数据出售给第三方，亦不会向任何机构提供任何用户画像分析或广告推送服务。</p>
      </div>
    ),
  },
  {
    value: "sharing-disclosure",
    title: "4. 信息共享与对外披露",
    content: (
      <div className="space-y-4 text-sm leading-relaxed">
        <p><strong>4.1 共享原则：</strong>除以下极端情况外，我们不会向任何第三方（包括且不限于关联公司、商业合作伙伴）共享您的个人信息：</p>
        <ul className="list-disc pl-4 md:pl-5 space-y-1 text-muted-foreground">
          <li>事先获得您的明确授权或同意；</li>
          <li>根据适用的法律法规、法律程序的要求、强制性的行政或司法要求所必须的情况下进行提供。</li>
        </ul>
        <p><strong>4.2 转让与公开披露：</strong>我们不会将您的个人信息转让给任何公司、组织和个人，亦不进行任何公开商业披露。</p>
      </div>
    ),
  },
  {
    value: "user-rights",
    title: "5. 您的权利与数据管理",
    content: (
      <div className="space-y-4 text-sm leading-relaxed">
        <p>依照《中华人民共和国个人信息保护法》，您对您的个人信息享有完整的控制权：</p>
        <div className="space-y-3">
          <div>
            <span className="font-semibold text-foreground">5.1 查阅与管理权：</span>
            <p className="mt-1 text-muted-foreground">您可以随时登录本平台，查阅您的基础个人信息，生成、轮换或撤销您的 AccessToken (API 密钥)。</p>
          </div>
          <div>
            <span className="font-semibold text-foreground">5.2 删除与注销权：</span>
            <p className="mt-1 text-muted-foreground">若您决定停止使用本服务，您可以申请注销账户。注销后，我们将立即从活跃存储媒介中删除您的所有敏感信息，法律法规要求留存的安全日志除外。</p>
          </div>
        </div>
      </div>
    ),
  },
  {
    value: "policy-update",
    title: "6. 政策更新与通知",
    content: (
      <div className="space-y-4 text-sm leading-relaxed">
        <p>随着业务的发展或法律法规的变动，本《隐私政策》条款可能发生变更。</p>
        <p>当条款发生重大变更时，我们会以显著方式（如平台公告、站内弹窗等）予以通知。如果您继续使用本服务，即表示您同意接受修订后的政策约束。</p>
      </div>
    ),
  },
]
