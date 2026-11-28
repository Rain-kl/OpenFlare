import Link from 'next/link';
import {
  Activity,
  ArrowDownRight,
  ArrowRight,
  ArrowUpRight,
  BookOpen,
  Boxes,
  Globe2,
  Network,
  ShieldCheck,
  Terminal,
  Waypoints,
} from 'lucide-react';

type LandingPageProps = {
  locale: 'zh' | 'en';
};

const copy = {
  zh: {
    navDocs: '阅读文档',
    github: 'GitHub',
    telegram: 'Telegram',
    eyebrow: 'OPEN SOURCE · EDGE SECURITY · SMARTDNS',
    title: '让边缘网络',
    accent: '清晰可控',
    description:
      '用一个控制面管理 OpenFlare Edge 集群与 OpenFlare DNS 权威节点，按实时网络质量智能调度流量，并在网络边缘执行 WAF、PoW 防护与安全隧道。',
    start: '从快速开始入门',
    explore: '了解系统架构',
    status: '边缘网络拓扑示意',
    statusDetail: '控制面 · SmartDNS 调度 · 边缘节点',
    visitor: '访问者',
    edge: 'OpenFlare Edge',
    origin: '源站与私有服务',
    statNodes: '边缘代理',
    statDns: '智能权威解析',
    statSync: '边缘安全防护',
    featureEyebrow: 'PLATFORM CAPABILITIES',
    featuresTitle: '从智能调度到边缘执行',
    featuresIntro: '控制面负责协调，SmartDNS 负责择优引流，边缘节点负责服务流量。',
    features: [
      {
        title: '智能权威 DNS（SmartDNS）',
        text: '独立 OpenFlare DNS 结合网络质量矩阵、ECS 识别与可视化规则引擎，将访客调度至低延迟健康节点。',
        href: '/guide/smartdns',
        icon: Globe2,
      },
      {
        title: '多节点配置发布',
        text: '集中管理站点与运行策略，配置经版本化校验后同步到在线节点。',
        href: '/guide/quick-start',
        icon: Network,
      },
      {
        title: '边缘安全防护',
        text: '在边缘执行 WAF、IP 组和 PoW 人机验证，降低恶意流量到达源站的机会。',
        href: '/guide/waf-usage',
        icon: ShieldCheck,
      },
      {
        title: '加密隧道连接',
        text: '通过 Relay 与 OpenFlared 将内网 Web 服务安全接入边缘网络。',
        href: '/guide/tunnel-usage',
        icon: Waypoints,
      },
      {
        title: '静态站点托管',
        text: '管理预构建静态资源，支持不可变部署、切换和回滚。',
        href: '/guide/pages-usage',
        icon: Boxes,
      },
      {
        title: '网络质量与运行观测',
        text: '内置多点探测工具、网络质量矩阵与全量访问日志，支持独立运行或协同多云 DNS。',
        href: '/guide/network-tools',
        icon: Activity,
      },
    ],
    architectureEyebrow: 'ONE CONTROL PLANE, DISTRIBUTED EDGE',
    architectureTitle: '让拓扑保持可读',
    architectureText:
      '控制面发布经过校验的站点与调度快照。SmartDNS 按实时网络质量引导访问，边缘节点执行站点、WAF 与性能策略并将流量送达公网源站或私有服务。',
    stepControl: '控制面与 SmartDNS',
    stepEdge: '边缘节点',
    stepServices: '源站服务',
    stepControlText: '编排配置并智能解析调度',
    stepEdgeText: '同步状态并代理请求',
    stepServicesText: '公网应用或内网服务',
    ctaEyebrow: 'START BUILDING YOUR EDGE',
    ctaTitle: '从第一台节点开始',
    ctaText: '快速开始指南涵盖 Server 部署、首次登录与节点接入。',
    ctaButton: '打开快速开始',
    ctaCommunity: '加入 Telegram 群组',
    footerCommunity: 'Telegram 群组',
    footer: '基于 Apache License 2.0 发布',
  },
  en: {
    navDocs: 'Read the docs',
    github: 'GitHub',
    telegram: 'Telegram',
    eyebrow: 'OPEN SOURCE · EDGE SECURITY · SMARTDNS',
    title: 'Make your edge',
    accent: 'clear and controlled',
    description:
      'Manage OpenFlare Edge and OpenFlare DNS from one control plane. Steer traffic with quality-aware SmartDNS, enforce WAF and proof-of-work challenges, and connect private services through secure tunnels.',
    start: 'Start with the quick guide',
    explore: 'Explore the architecture',
    status: 'Edge network topology',
    statusDetail: 'Control plane · SmartDNS steering · edge nodes',
    visitor: 'Visitors',
    edge: 'OpenFlare Edge',
    origin: 'Origins & private services',
    statNodes: 'edge proxy',
    statDns: 'smart authoritative DNS',
    statSync: 'edge security',
    featureEyebrow: 'PLATFORM CAPABILITIES',
    featuresTitle: 'From smart steering to edge execution',
    featuresIntro: 'The control plane coordinates, SmartDNS steers, and edge nodes serve traffic.',
    features: [
      {
        title: 'Intelligent DNS (SmartDNS)',
        text: 'Standalone OpenFlare DNS combines real-time quality matrices, ECS awareness, and visual flow rules to route visitors to optimal healthy nodes.',
        href: '/en/guide/smartdns',
        icon: Globe2,
      },
      {
        title: 'Multi-node releases',
        text: 'Manage sites and runtime policies centrally, then distribute validated configuration versions to online nodes.',
        href: '/en/guide/quick-start',
        icon: Network,
      },
      {
        title: 'Edge security',
        text: 'Apply WAF rules, IP groups, and proof-of-work challenges at the edge before requests reach your origins.',
        href: '/en/guide/waf-usage',
        icon: ShieldCheck,
      },
      {
        title: 'Encrypted tunnels',
        text: 'Connect private web services through Relay and OpenFlared without exposing inbound ports.',
        href: '/en/guide/tunnel-usage',
        icon: Waypoints,
      },
      {
        title: 'Static site hosting',
        text: 'Manage prebuilt assets with immutable deployments, activation, and rollback.',
        href: '/en/guide/pages-usage',
        icon: Boxes,
      },
      {
        title: 'Health & observability',
        text: 'Inspect node health, network latency metrics, and access logs in one place, standalone or paired with multi-provider DNS.',
        href: '/en/design/observability-design',
        icon: Activity,
      },
    ],
    architectureEyebrow: 'ONE CONTROL PLANE, DISTRIBUTED EDGE',
    architectureTitle: 'Keep the topology readable',
    architectureText:
      'The control plane publishes validated site and scheduling snapshots. SmartDNS guides visitors by real-time network quality while edge nodes enforce site, WAF, and performance policies toward public origins or private services.',
    stepControl: 'Control plane & SmartDNS',
    stepEdge: 'Edge nodes',
    stepServices: 'Origin services',
    stepControlText: 'Organize config and steer DNS',
    stepEdgeText: 'Sync state and proxy requests',
    stepServicesText: 'Public apps or private services',
    ctaEyebrow: 'START BUILDING YOUR EDGE',
    ctaTitle: 'Begin with your first node',
    ctaText: 'The quick start covers Server deployment, first login, and connecting an edge node.',
    ctaButton: 'Open the quick start',
    ctaCommunity: 'Join Telegram Group',
    footerCommunity: 'Telegram Group',
    footer: 'Released under the Apache License 2.0',
  },
} as const;

export function LandingPage({ locale }: LandingPageProps) {
  const text = copy[locale];
  const homeUrl = locale === 'zh' ? '/' : '/en';
  const githubUrl = 'https://github.com/Rain-kl/OpenFlare';
  const telegramUrl = 'https://t.me/OpenFlareCDN';

  return (
    <main className="landing-page relative">
      <div
        aria-hidden="true"
        className="landing-grid pointer-events-none absolute inset-x-0 top-0 h-[760px] opacity-70"
      />
      <header className="relative z-10 mx-auto flex max-w-7xl items-center justify-between px-6 py-5 lg:px-10">
        <Link aria-label="OpenFlare home" className="flex items-center gap-2.5" href={homeUrl}>
          <img alt="" className="size-8" src="/brand/openflare-icon.svg" />
          <span className="text-base font-semibold tracking-tight">OpenFlare</span>
        </Link>
        <nav className="flex items-center gap-2 sm:gap-3">
          <Link
            className="rounded-full px-4 py-2 text-sm font-medium text-fd-muted-foreground transition hover:bg-fd-accent hover:text-fd-foreground"
            href={locale === 'zh' ? '/en' : '/'}
          >
            {locale === 'zh' ? 'EN' : '中文'}
          </Link>
          <Link
            className="hidden items-center rounded-full px-4 py-2 text-sm font-medium text-fd-muted-foreground transition hover:bg-fd-accent hover:text-fd-foreground sm:inline-flex"
            href={githubUrl}
            rel="noreferrer"
            target="_blank"
          >
            <ArrowUpRight aria-hidden="true" className="mr-2 size-4" />
            {text.github}
          </Link>
          <Link
            className="hidden items-center rounded-full px-4 py-2 text-sm font-medium text-fd-muted-foreground transition hover:bg-fd-accent hover:text-fd-foreground sm:inline-flex"
            href={telegramUrl}
            rel="noreferrer"
            target="_blank"
          >
            <ArrowUpRight aria-hidden="true" className="mr-2 size-4" />
            {text.telegram}
          </Link>
          <Link
            className="inline-flex items-center gap-2 rounded-full bg-fd-primary px-4 py-2 text-sm font-medium text-fd-primary-foreground transition hover:opacity-90"
            href={locale === 'zh' ? '/guide/quick-start' : '/en/guide/quick-start'}
          >
            <BookOpen aria-hidden="true" className="size-4" />
            {text.navDocs}
          </Link>
        </nav>
      </header>

      <section className="relative mx-auto grid max-w-7xl items-center gap-12 px-6 pb-24 pt-16 lg:grid-cols-[0.96fr_1.04fr] lg:px-10 lg:pb-32 lg:pt-20">
        <div className="relative z-10">
          <div className="mb-6 inline-flex items-center gap-2 rounded-full border border-dashed border-fd-border bg-fd-card/70 px-3 py-1.5 font-mono text-[10px] tracking-[0.16em] text-fd-muted-foreground sm:text-xs">
            <span className="size-1.5 rounded-full bg-fd-primary" />
            {text.eyebrow}
          </div>
          <h1 className="max-w-3xl text-5xl font-semibold leading-[1.08] tracking-[-0.055em] sm:text-6xl lg:text-7xl">
            {text.title}
            <br />
            <span className="bg-gradient-to-r from-fd-primary via-emerald-500 to-cyan-600 bg-clip-text text-transparent">
              {text.accent}
            </span>
          </h1>
          <p className="mt-7 max-w-xl text-base leading-7 text-fd-muted-foreground sm:text-lg sm:leading-8">
            {text.description}
          </p>
          <div className="mt-9 flex flex-wrap items-center gap-3">
            <Link
              className="group inline-flex items-center gap-2 rounded-full bg-fd-primary px-5 py-3 text-sm font-semibold text-fd-primary-foreground transition hover:gap-3"
              href={locale === 'zh' ? '/guide/quick-start' : '/en/guide/quick-start'}
            >
              {text.start}
              <ArrowRight aria-hidden="true" className="size-4 transition-transform group-hover:translate-x-0.5" />
            </Link>
            <Link
              className="inline-flex items-center gap-2 rounded-full border border-dashed border-fd-border px-5 py-3 text-sm font-medium transition hover:border-fd-primary/50 hover:bg-fd-accent"
              href={locale === 'zh' ? '/design/architecture' : '/en/design/architecture'}
            >
              {text.explore}
              <ArrowDownRight aria-hidden="true" className="size-4" />
            </Link>
          </div>
          <div className="mt-12 flex flex-wrap gap-x-8 gap-y-4 border-t border-dashed border-fd-border pt-6">
            {[
              ['EDGE NODE', text.statNodes],
              ['SMARTDNS', text.statDns],
              ['WAF', text.statSync],
            ].map(([value, label]) => (
              <div className="flex items-center gap-2.5" key={label}>
                <span className="font-mono text-sm font-semibold text-fd-primary">{value}</span>
                <span className="text-xs text-fd-muted-foreground">{label}</span>
              </div>
            ))}
          </div>
        </div>

        <div className="relative mx-auto w-full max-w-[650px] lg:ml-auto">
          <div className="absolute -inset-6 rounded-[2.5rem] bg-emerald-500/10 blur-3xl" />
          <div className="landing-network relative overflow-hidden rounded-[1.75rem] border border-dashed border-fd-border p-5 sm:p-7">
            <div className="flex items-start justify-between gap-4">
              <div>
                <div className="flex items-center gap-2 text-sm font-medium">
                  <span className="size-2 rounded-full bg-emerald-500 shadow-[0_0_16px_rgba(16,185,129,.75)]" />
                  {text.status}
                </div>
                <p className="mt-1.5 text-xs text-fd-muted-foreground">{text.statusDetail}</p>
              </div>
              <div className="rounded-md border border-dashed border-fd-border bg-fd-card/80 px-2 py-1 font-mono text-[10px] tracking-wider text-fd-muted-foreground">
                ARCHITECTURE
              </div>
            </div>

            <div className="relative mt-6 h-[285px] overflow-hidden rounded-xl border border-dashed border-fd-border/70 bg-fd-background/45 sm:h-[325px]">
              <svg aria-hidden="true" className="absolute inset-0 h-full w-full" preserveAspectRatio="none" viewBox="0 0 640 340">
                <path className="network-line" d="M95 82 C205 80 214 154 315 154" fill="none" strokeWidth="1.5" />
                <path className="network-line" d="M95 82 C220 102 236 236 315 236" fill="none" strokeWidth="1.5" />
                <path className="network-line" d="M315 154 C415 154 424 102 542 102" fill="none" strokeWidth="1.5" />
                <path className="network-line" d="M315 236 C415 236 430 102 542 102" fill="none" strokeWidth="1.5" />
              </svg>
              <div className="absolute left-[5%] top-[17%] flex items-center gap-2 rounded-lg border border-dashed border-fd-border bg-fd-card/90 px-2.5 py-2 text-[11px] font-medium sm:px-3 sm:text-xs">
                <Globe2 className="size-4 text-fd-muted-foreground" />
                {text.visitor}
              </div>
              <div className="absolute left-[35%] top-[39%] flex min-w-[168px] flex-col gap-2 rounded-xl border border-fd-primary/40 bg-fd-card px-3 py-3 shadow-[0_0_35px_rgba(16,185,129,.08)] sm:min-w-[190px] sm:px-4">
                <div className="flex items-center gap-2 text-[11px] font-semibold sm:text-xs">
                  <img alt="" className="size-5" src="/brand/openflare-icon.svg" />
                  {text.edge}
                </div>
                <div className="flex gap-1.5">
                  <span className="h-1 flex-1 rounded-full bg-fd-primary" />
                  <span className="h-1 flex-1 rounded-full bg-fd-primary/50" />
                  <span className="h-1 flex-1 rounded-full bg-fd-primary/20" />
                </div>
              </div>
              <div className="absolute right-[5%] top-[23%] flex items-center gap-2 rounded-lg border border-dashed border-fd-border bg-fd-card/90 px-2.5 py-2 text-[10px] font-medium sm:px-3 sm:text-xs">
                <Terminal className="size-4 text-fd-muted-foreground" />
                {text.origin}
              </div>
              <div className="absolute bottom-3 left-3 flex items-center gap-1.5 font-mono text-[9px] uppercase tracking-wider text-fd-muted-foreground sm:bottom-4 sm:left-4 sm:text-[10px]">
                <span className="size-1.5 rounded-full bg-fd-primary" />
                CONTROL PLANE
              </div>
              <div className="absolute bottom-3 right-3 font-mono text-[9px] uppercase tracking-wider text-fd-muted-foreground sm:bottom-4 sm:right-4 sm:text-[10px]">
                SMARTDNS · TLS · WAF
              </div>
            </div>

            <div className="mt-4 grid grid-cols-3 divide-x divide-dashed divide-fd-border text-center">
              {[
                ['EDGE NODE', text.statNodes],
                ['SMARTDNS', text.statDns],
                ['WAF', text.statSync],
              ].map(([value, label]) => (
                <div className="px-2" key={label}>
                  <div className="font-mono text-sm font-semibold">{value}</div>
                  <div className="mt-1 text-[10px] text-fd-muted-foreground">{label}</div>
                </div>
              ))}
            </div>
          </div>
        </div>
      </section>

      <section className="relative border-y border-dashed border-fd-border bg-fd-card/25">
        <div className="mx-auto max-w-7xl px-6 py-20 lg:px-10 lg:py-24">
          <div className="mb-10 flex flex-col justify-between gap-4 sm:flex-row sm:items-end">
            <div>
              <p className="font-mono text-[10px] tracking-[0.18em] text-fd-primary sm:text-xs">
                {text.featureEyebrow}
              </p>
              <h2 className="mt-3 text-3xl font-semibold tracking-tight sm:text-4xl">
                {text.featuresTitle}
              </h2>
            </div>
            <p className="max-w-md text-sm leading-6 text-fd-muted-foreground sm:text-right">
              {text.featuresIntro}
            </p>
          </div>
          <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
            {text.features.map((feature, index) => {
              const Icon = feature.icon;
              return (
                <Link className="landing-card group rounded-xl p-5 sm:p-6" href={feature.href} key={feature.title}>
                  <div className="mb-8 flex items-center justify-between">
                    <Icon aria-hidden="true" className="size-5 text-fd-primary" />
                    <span className="font-mono text-[10px] text-fd-muted-foreground/70">
                      0{index + 1}
                    </span>
                  </div>
                  <h3 className="text-base font-semibold tracking-tight">{feature.title}</h3>
                  <p className="mt-2 text-sm leading-6 text-fd-muted-foreground">
                    {feature.text}
                  </p>
                  <div className="mt-5 flex items-center gap-1.5 text-xs font-medium text-fd-primary opacity-0 transition group-hover:opacity-100">
                    {text.navDocs}
                    <ArrowRight aria-hidden="true" className="size-3.5" />
                  </div>
                </Link>
              );
            })}
          </div>
        </div>
      </section>

      <section className="mx-auto grid max-w-7xl gap-10 px-6 py-20 lg:grid-cols-[0.82fr_1.18fr] lg:items-center lg:px-10 lg:py-28">
        <div>
          <p className="font-mono text-[10px] tracking-[0.18em] text-fd-primary sm:text-xs">
            {text.architectureEyebrow}
          </p>
          <h2 className="mt-3 text-3xl font-semibold tracking-tight sm:text-4xl">
            {text.architectureTitle}
          </h2>
          <p className="mt-5 max-w-lg text-sm leading-7 text-fd-muted-foreground sm:text-base">
            {text.architectureText}
          </p>
          <Link
            className="mt-7 inline-flex items-center gap-2 text-sm font-semibold text-fd-primary hover:underline"
            href={locale === 'zh' ? '/design/architecture' : '/en/design/architecture'}
          >
            {text.explore}
            <ArrowRight aria-hidden="true" className="size-4" />
          </Link>
        </div>
        <div className="rounded-2xl border border-dashed border-fd-border p-4 sm:p-7">
          <div className="grid gap-3 sm:grid-cols-[1fr_38px_1fr_38px_1fr] sm:items-center">
            {[
              [Network, text.stepControl, text.stepControlText],
              [Waypoints, text.stepEdge, text.stepEdgeText],
              [Terminal, text.stepServices, text.stepServicesText],
            ].map(([Icon, title, description], index) => {
              const FeatureIcon = Icon as typeof Network;
              return (
                <div className="contents" key={title as string}>
                  {index > 0 && (
                    <div className="hidden justify-center text-fd-muted-foreground sm:flex">
                      <ArrowRight aria-hidden="true" className="size-4" />
                    </div>
                  )}
                  <div className="rounded-xl border border-dashed border-fd-border bg-fd-card/80 p-4">
                    <FeatureIcon aria-hidden="true" className="mb-4 size-5 text-fd-primary" />
                    <h3 className="text-sm font-semibold">{title as string}</h3>
                    <p className="mt-1.5 text-xs leading-5 text-fd-muted-foreground">
                      {description as string}
                    </p>
                  </div>
                </div>
              );
            })}
          </div>
        </div>
      </section>

      <section className="mx-auto max-w-7xl px-6 pb-20 lg:px-10 lg:pb-28">
        <div className="relative overflow-hidden rounded-2xl border border-dashed border-fd-primary/40 bg-fd-primary/[0.045] px-6 py-10 sm:px-10 sm:py-12">
          <div aria-hidden="true" className="absolute -right-10 -top-20 size-64 rounded-full border border-dashed border-fd-primary/20" />
          <div aria-hidden="true" className="absolute -right-1 -top-11 size-44 rounded-full border border-dashed border-fd-primary/20" />
          <div className="relative z-10 max-w-2xl">
            <p className="font-mono text-[10px] tracking-[0.18em] text-fd-primary sm:text-xs">
              {text.ctaEyebrow}
            </p>
            <h2 className="mt-3 text-3xl font-semibold tracking-tight sm:text-4xl">
              {text.ctaTitle}
            </h2>
            <p className="mt-3 text-sm leading-6 text-fd-muted-foreground sm:text-base">
              {text.ctaText}
            </p>
            <div className="mt-6 flex flex-wrap items-center gap-3">
              <Link
                className="inline-flex items-center gap-2 rounded-full bg-fd-primary px-5 py-3 text-sm font-semibold text-fd-primary-foreground transition hover:gap-3"
                href={locale === 'zh' ? '/guide/quick-start' : '/en/guide/quick-start'}
              >
                {text.ctaButton}
                <ArrowRight aria-hidden="true" className="size-4" />
              </Link>
              <Link
                className="inline-flex items-center gap-2 rounded-full border border-dashed border-fd-border px-5 py-3 text-sm font-medium transition hover:border-fd-primary/50 hover:bg-fd-accent"
                href={telegramUrl}
                rel="noreferrer"
                target="_blank"
              >
                {text.ctaCommunity}
                <ArrowUpRight aria-hidden="true" className="size-4" />
              </Link>
            </div>
          </div>
        </div>
      </section>

      <footer className="border-t border-dashed border-fd-border px-6 py-6">
        <div className="mx-auto flex max-w-7xl flex-col items-center justify-between gap-3 text-xs text-fd-muted-foreground sm:flex-row lg:px-10">
          <span>© OpenFlare contributors</span>
          <div className="flex flex-wrap items-center justify-center gap-4">
            <Link
              className="transition hover:text-fd-foreground"
              href={telegramUrl}
              rel="noreferrer"
              target="_blank"
            >
              {text.footerCommunity}
            </Link>
            <Link
              className="transition hover:text-fd-foreground"
              href={githubUrl}
              rel="noreferrer"
              target="_blank"
            >
              {text.github}
            </Link>
            <span>{text.footer}</span>
          </div>
        </div>
      </footer>
    </main>
  );
}
