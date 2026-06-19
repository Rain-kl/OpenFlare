import {type PolicySection} from "./types"

export const TERMS_LAST_UPDATED = "2026-06-07"

/**
 * ------------------------------------------------------------------
 * 服务条款 (TERMS OF SERVICE)
 * ------------------------------------------------------------------
 */
export const termsSections: PolicySection[] = [
  {
    value: "contract-establishment",
    title: "1. 缔约申明与服务综述",
    content: (
      <div className="space-y-4 text-sm leading-relaxed">
        <p><strong>1.1 缔约主体：</strong>本《服务协议》（以下简称“本协议”）是您（以下称“用户”或“开发者”）与本通用开发脚手架平台（以下简称“本系统”或“平台”）运营维护方之间关于使用本系统所订立的契约。</p>
        <p><strong>1.2 审慎阅读：</strong>本系统作为一个通用的、面向二次开发的全栈软件底座，旨在为用户提供基础的注册、会话、OIDC 接入、API Key 令牌鉴权及后台管理服务。若您使用本系统，请务必仔细阅读本协议各条款。</p>
        <p><strong>1.3 协议构成：</strong>本协议内容包括协议正文及所有我们已经发布或将来可能发布的各类规则、声明、说明。所有规则为本协议不可分割的组成部分，与协议正文具有同等法律效力。</p>
      </div>
    ),
  },
  {
    value: "service-definition",
    title: "2. 服务定义与性质界定",
    content: (
      <div className="space-y-4 text-sm leading-relaxed">
        <ul className="list-disc pl-4 md:pl-5 space-y-2">
          <li><strong>责任自担：</strong>关于用户对本系统进行二次开发并应用于其他生产环境产生的任何业务行为，由二开部署运营主体承担全部合规责任。</li>
        </ul>
      </div>
    ),
  },
  {
    value: "account-specifications",
    title: "3. 账号注册与使用规范",
    content: (
      <div className="space-y-4 text-sm leading-relaxed">
        <p><strong>3.1 账号体系：</strong>用户可通过本系统的前端注册表单自助创建账户，或通过系统管理员配置并启用的自定义第三方 OIDC 认证源进行登录关联。</p>
        <p><strong>3.2 密码及令牌安全责任：</strong></p>
        <ul className="list-disc pl-4 md:pl-5 space-y-2">
          <li><strong>密码安全：</strong>您应妥善保管您账户的登录密码。</li>
          <li><strong>API Token 安全：</strong>个人生成的 AccessToken (API Key) 代表您账户的完整调用权限。<strong>因您保管不善导致 Token 泄漏而造成的一切数据丢失或系统损失，均由您自行承担。</strong></li>
        </ul>
      </div>
    ),
  },
  {
    value: "user-conduct",
    title: "4. 用户行为准则（负面清单）",
    content: (
      <div className="space-y-4 text-sm leading-relaxed">
        <p>您在使用本系统时，必须严格遵守《中华人民共和国网络安全法》、《计算机信息网络国际联网安全保护管理办法》等法律法规。<strong>严禁利用本平台从事以下活动（“红线条款”）：</strong></p>
        <div className="bg-red-500/5 border border-red-500/20 rounded-lg p-3 md:p-4 space-y-3">
          <ul className="list-disc pl-4 md:pl-5 space-y-2 text-red-600 dark:text-red-400 font-medium">
            <li><strong>危害国家安全：</strong>反对宪法所确定的基本原则、危害国家安全、泄露国家秘密、颠覆国家政权、破坏国家统一的；</li>
            <li><strong>非法信息服务：</strong>黑客攻击工具、DDoS 攻击服务、服务器爆破等其他非法信息服务平台；</li>
            <li><strong>黄赌毒关联：</strong>制作、复制、发布、传播淫秽、色情、赌博、暴力、凶杀、恐怖或者教唆犯罪的；</li>
            <li><strong>侵犯知识产权：</strong>销售盗版软件、盗版影视资源、非法游戏外挂、私服、黑号、社工库数据等；</li>
            <li><strong>其他违法信息：</strong>涉及散布谣言、宣扬邪教/封建迷信、侮辱/诽谤他人、侵害他人合法权益的。</li>
          </ul>
        </div>
        <p><strong>处理规则：</strong>一旦发现您违反上述规定，平台运维主体有权不经通知<strong>立即永久封禁您的账号、拦截所有 AccessToken 请求，并依法向公安机关、网安部门移交相关线索。</strong></p>
      </div>
    ),
  },
  {
    value: "liability-limitation",
    title: "5. 免责声明与不可抗力",
    content: (
      <div className="space-y-4 text-sm leading-relaxed">
        <p><strong>5.1 基础免责：</strong>本开发底座按“现状”（As-Is）及“现有”（As-Available）状态提供。我们不保证服务一定能满足您的特定开发需求，对服务的及时性、安全性、准确性都不作额外担保。</p>
        <p><strong>5.2 技术服务中断：</strong>对于因以下不可抗力原因导致的服务中断、数据丢失或账号损坏，平台不承担赔偿责任：</p>
        <ul className="list-disc pl-4 md:pl-5 space-y-1 text-muted-foreground">
          <li>自然灾害（台风、地震、海啸、洪水等）；</li>
          <li>政府行为、网络安全法律法规调整或行政命令；</li>
          <li>电信部门线路技术故障、机房海底光缆损毁；</li>
          <li>黑客入侵、勒索病毒感染导致的数据损毁或宕机。</li>
        </ul>
      </div>
    ),
  },
  {
    value: "governing-law",
    title: "6. 法律适用与争议解决",
    content: (
      <div className="space-y-4 text-sm leading-relaxed">
        <p><strong>6.1 法律适用：</strong>本协议的订立、执行、解释及争议的解决均适用<strong>中华人民共和国法律</strong>（不包括港澳台地区冲突法）。</p>
        <p><strong>6.2 争议解决：</strong>若发生任何争议或纠纷，首先应友好协商解决；协商不成的，应提交至本系统部署或运营方所在地有管辖权的人民法院管辖。</p>
      </div>
    ),
  },
]
