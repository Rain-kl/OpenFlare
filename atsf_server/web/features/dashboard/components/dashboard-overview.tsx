'use client';

import Link from 'next/link';
import { useQueries } from '@tanstack/react-query';

import { ErrorState } from '@/components/feedback/error-state';
import { LoadingState } from '@/components/feedback/loading-state';
import { AppCard } from '@/components/ui/app-card';
import { StatusBadge } from '@/components/ui/status-badge';
import { getConfigVersions } from '@/features/config-versions/api/config-versions';
import type { ConfigVersionItem } from '@/features/config-versions/types';
import { getManagedDomains } from '@/features/managed-domains/api/managed-domains';
import type { ManagedDomainItem } from '@/features/managed-domains/types';
import { getNodes } from '@/features/nodes/api/nodes';
import type { NodeItem } from '@/features/nodes/types';
import { getProxyRoutes } from '@/features/proxy-routes/api/proxy-routes';
import type { ProxyRouteItem } from '@/features/proxy-routes/types';
import { getTlsCertificates } from '@/features/tls-certificates/api/tls-certificates';
import type { TlsCertificateItem } from '@/features/tls-certificates/types';
import { getUsers, searchUsers } from '@/features/users/api/users';
import type { UserItem } from '@/features/users/types';
import { formatDateTime } from '@/lib/utils/date';

function parseCustomHeaders(rawValue: string) {
  if (!rawValue) {
    return [];
  }

  try {
    const parsed = JSON.parse(rawValue);
    return Array.isArray(parsed) ? parsed : [];
  } catch {
    return [];
  }
}

function getErrorMessage(error: unknown) {
  return error instanceof Error ? error.message : '请求失败，请稍后重试。';
}

function SummaryCard({
  title,
  description,
  href,
  items,
  status,
  error,
}: {
  title: string;
  description: string;
  href: string;
  items: ReadonlyArray<{ label: string; value: string | number }>;
  status: 'pending' | 'success' | 'error';
  error?: unknown;
}) {
  return (
    <AppCard
      title={title}
      description={description}
      action={
        <Link
          href={href}
          className='inline-flex items-center rounded-full border border-[var(--border-default)] px-3 py-1.5 text-xs text-[var(--foreground-primary)] transition hover:bg-[var(--control-background-hover)]'
        >
          进入模块
        </Link>
      }
    >
      {status === 'pending' ? (
        <LoadingState />
      ) : status === 'error' ? (
        <ErrorState title={`${title}加载失败`} description={getErrorMessage(error)} />
      ) : (
        <div className='grid gap-3 sm:grid-cols-2'>
          {items.map((item) => (
            <div
              key={item.label}
              className='rounded-2xl border border-[var(--border-default)] bg-[var(--surface-muted)] px-4 py-4'
            >
              <p className='text-xs uppercase tracking-[0.2em] text-[var(--foreground-muted)]'>
                {item.label}
              </p>
              <p className='mt-2 text-lg font-semibold text-[var(--foreground-primary)]'>{item.value}</p>
            </div>
          ))}
        </div>
      )}
    </AppCard>
  );
}

export function DashboardOverview() {
  const [
    nodesQuery,
    versionsQuery,
    managedDomainsQuery,
    certificatesQuery,
    proxyRoutesQuery,
    usersQuery,
  ] = useQueries({
    queries: [
      { queryKey: ['dashboard', 'nodes'], queryFn: getNodes },
      { queryKey: ['dashboard', 'config-versions'], queryFn: getConfigVersions },
      { queryKey: ['dashboard', 'managed-domains'], queryFn: getManagedDomains },
      { queryKey: ['dashboard', 'tls-certificates'], queryFn: getTlsCertificates },
      { queryKey: ['dashboard', 'proxy-routes'], queryFn: getProxyRoutes },
      {
        queryKey: ['dashboard', 'users'],
        queryFn: async () => {
          try {
            return await searchUsers('');
          } catch {
            return getUsers(0);
          }
        },
      },
    ],
  });

  const nodes = (nodesQuery.data ?? []) as NodeItem[];
  const versions = (versionsQuery.data ?? []) as ConfigVersionItem[];
  const domains = (managedDomainsQuery.data ?? []) as ManagedDomainItem[];
  const certificates = (certificatesQuery.data ?? []) as TlsCertificateItem[];
  const routes = (proxyRoutesQuery.data ?? []) as ProxyRouteItem[];
  const users = (usersQuery.data ?? []) as UserItem[];

  const expiringCertificates = certificates.filter((item) => {
    const expiresAt = new Date(item.not_after).getTime();
    return !Number.isNaN(expiresAt) && expiresAt >= Date.now() && expiresAt - Date.now() <= 30 * 24 * 60 * 60 * 1000;
  }).length;

  const expiredCertificates = certificates.filter((item) => {
    const expiresAt = new Date(item.not_after).getTime();
    return !Number.isNaN(expiresAt) && expiresAt < Date.now();
  }).length;

  const dashboardCards = [
    {
      title: '节点摘要',
      description: '集中查看在线状态、待接入节点和自动更新覆盖情况。',
      href: '/node',
      status: nodesQuery.status,
      error: nodesQuery.error,
      items: [
        { label: '节点总数', value: nodes.length },
        { label: '在线节点', value: nodes.filter((item) => item.status === 'online').length },
        { label: '待接入节点', value: nodes.filter((item) => item.status === 'pending').length },
        { label: '自动更新', value: nodes.filter((item) => item.auto_update_enabled).length },
      ],
    },
    {
      title: '版本摘要',
      description: '直接感知当前发布沉淀、激活版本和最近一次变更时间。',
      href: '/config-version',
      status: versionsQuery.status,
      error: versionsQuery.error,
      items: [
        { label: '版本总数', value: versions.length },
        { label: '激活版本', value: versions.filter((item) => item.is_active).length },
        { label: '最近创建', value: versions[0]?.created_at ? formatDateTime(versions[0].created_at) : '—' },
      ],
    },
    {
      title: '规则摘要',
      description: '查看域名规则启用状态、通配符覆盖和证书绑定规模。',
      href: '/managed-domain',
      status: managedDomainsQuery.status,
      error: managedDomainsQuery.error,
      items: [
        { label: '域名规则', value: domains.length },
        { label: '已启用', value: domains.filter((item) => item.enabled).length },
        { label: '通配符规则', value: domains.filter((item) => item.domain.startsWith('*.')).length },
        { label: '已绑证书', value: domains.filter((item) => item.cert_id).length },
      ],
    },
    {
      title: '证书概览',
      description: '快速识别证书存量、近 30 天到期风险和过期情况。',
      href: '/tls-certificate',
      status: certificatesQuery.status,
      error: certificatesQuery.error,
      items: [
        { label: '证书总数', value: certificates.length },
        { label: '30 天内到期', value: expiringCertificates },
        { label: '已过期', value: expiredCertificates },
        { label: '最近更新', value: certificates[0]?.updated_at ? formatDateTime(certificates[0].updated_at) : '—' },
      ],
    },
    {
      title: '发布与摘要',
      description: '汇总反向代理规则规模、HTTPS 覆盖和请求头配置密度。',
      href: '/proxy-route',
      status: proxyRoutesQuery.status,
      error: proxyRoutesQuery.error,
      items: [
        { label: '规则总数', value: routes.length },
        { label: '已启用', value: routes.filter((item) => item.enabled).length },
        { label: 'HTTPS 规则', value: routes.filter((item) => item.enable_https).length },
        {
          label: '自定义请求头',
          value: routes.reduce((count, route) => count + parseCustomHeaders(route.custom_headers).length, 0),
        },
      ],
    },
    {
      title: '用户概览',
      description: '展示当前可管理用户池的角色和状态分布。',
      href: '/user',
      status: usersQuery.status,
      error: usersQuery.error,
      items: [
        { label: '用户总数', value: users.length },
        { label: '管理员', value: users.filter((item) => item.role >= 10).length },
        { label: '已激活', value: users.filter((item) => item.status === 1).length },
        { label: '已封禁', value: users.filter((item) => item.status !== 1).length },
      ],
    },
  ] as const;

  return (
    <div className='space-y-6'>
      <AppCard
        title='控制台仪表盘'
        description='首页统一承接节点、发布、域名、证书、反向代理和用户六个模块的核心总览，避免在各页面重复扫一遍摘要。'
        action={<StatusBadge label='实时聚合概览' variant='success' />}
      >
        <div className='grid gap-4 md:grid-cols-3'>
          <div className='rounded-2xl border border-[var(--border-default)] bg-[var(--surface-muted)] p-4'>
            <p className='text-xs uppercase tracking-[0.2em] text-[var(--foreground-muted)]'>信息架构</p>
            <p className='mt-2 text-base font-semibold text-[var(--foreground-primary)]'>首页负责总览</p>
            <p className='mt-2 text-sm leading-6 text-[var(--foreground-secondary)]'>
              模块页保留列表、表单和具体操作，摘要统一回收到仪表盘。
            </p>
          </div>
          <div className='rounded-2xl border border-[var(--border-default)] bg-[var(--surface-muted)] p-4'>
            <p className='text-xs uppercase tracking-[0.2em] text-[var(--foreground-muted)]'>导航效率</p>
            <p className='mt-2 text-base font-semibold text-[var(--foreground-primary)]'>顶栏与侧栏已精简</p>
            <p className='mt-2 text-sm leading-6 text-[var(--foreground-secondary)]'>
              顶栏只保留版本、主题切换和用户入口，侧栏宽度与按钮高度同步压缩。
            </p>
          </div>
          <div className='rounded-2xl border border-[var(--border-default)] bg-[var(--surface-muted)] p-4'>
            <p className='text-xs uppercase tracking-[0.2em] text-[var(--foreground-muted)]'>响应式</p>
            <p className='mt-2 text-base font-semibold text-[var(--foreground-primary)]'>小屏可正常展开侧栏</p>
            <p className='mt-2 text-sm leading-6 text-[var(--foreground-secondary)]'>
              小于 1000px 时切换为抽屉侧栏，不再出现按钮可点但导航不可见的问题。
            </p>
          </div>
        </div>
      </AppCard>

      <div className='grid gap-6 xl:grid-cols-2'>
        {dashboardCards.map((card) => (
          <SummaryCard key={card.title} {...card} />
        ))}
      </div>
    </div>
  );
}
