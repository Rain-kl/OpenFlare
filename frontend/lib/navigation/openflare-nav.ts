import type { LucideIcon } from 'lucide-react';
import {
  FileText,
  Gauge,
  GitBranch,
  Globe,
  LayoutDashboard,
  Route,
  ScrollText,
  Server,
  ShieldCheck,
} from 'lucide-react';

export interface OpenFlareNavItem {
  titleKey: string;
  url: string;
  icon: LucideIcon;
  /** 子页面在侧栏中仍高亮父级菜单项 */
  childUrls?: string[];
}

export interface OpenFlareNavSubItem {
  titleKey: string;
  url: string;
  childUrls?: string[];
}

export interface OpenFlareNavGroup {
  titleKey: string;
  icon: LucideIcon;
  items: OpenFlareNavSubItem[];
}

export type OpenFlareSidebarNavEntry =
  | ({ kind: 'item' } & OpenFlareNavItem)
  | ({ kind: 'group' } & OpenFlareNavGroup);

/** 安全性折叠组 */
export const openflareSecurityNavGroup: OpenFlareNavGroup = {
  titleKey: 'security',
  icon: ShieldCheck,
  items: [
    { titleKey: 'waf', url: '/waf' },
    { titleKey: 'ipGroups', url: '/ip-groups' },
    { titleKey: 'rateLimits', url: '/rate-limits' },
  ],
};

/** 网站管理折叠组 */
export const openflareWebsiteNavGroup: OpenFlareNavGroup = {
  titleKey: 'websites',
  icon: Globe,
  items: [
    { titleKey: 'websites', url: '/websites', childUrls: ['/websites/detail'] },
    { titleKey: 'certificates', url: '/certificates' },
    { titleKey: 'dnsAccounts', url: '/dns-accounts' },
    {
      titleKey: 'cloudflare',
      url: '/cloudflare',
      childUrls: ['/cloudflare/settings'],
    },
    { titleKey: 'origins', url: '/origins', childUrls: ['/origins/detail'] },
    {
      titleKey: 'responses',
      url: '/responses',
      childUrls: [
        '/responses/error-page/edit',
        '/responses/error-page/preview',
        '/responses/offline/edit',
        '/responses/offline/preview',
      ],
    },
  ],
};

/**
 * OpenFlare 侧栏导航顺序（单一配置源）。
 * 调整菜单顺序或折叠组位置时，只需修改此数组。
 */
export const openflareSidebarNav: OpenFlareSidebarNavEntry[] = [
  { kind: 'item', titleKey: 'dashboard', url: '/', icon: LayoutDashboard },
  {
    kind: 'item',
    titleKey: 'nodes',
    url: '/nodes',
    icon: Server,
    childUrls: ['/nodes/detail'],
  },
  {
    kind: 'item',
    titleKey: 'proxyRoutes',
    url: '/proxy-routes',
    icon: Route,
    childUrls: ['/proxy-routes/detail'],
  },
  { kind: 'group', ...openflareWebsiteNavGroup },
  { kind: 'group', ...openflareSecurityNavGroup },
  {
    kind: 'item',
    titleKey: 'pages',
    url: '/pages',
    icon: FileText,
    childUrls: ['/pages/detail'],
  },
  {
    kind: 'item',
    titleKey: 'configVersions',
    url: '/config-versions',
    icon: GitBranch,
  },
  {
    kind: 'item',
    titleKey: 'accessLogs',
    url: '/access-logs',
    icon: ScrollText,
  },
  { kind: 'item', titleKey: 'performance', url: '/performance', icon: Gauge },
];

/** 扁平菜单项（供路由判断等逻辑复用） */
export const openflareNavItems: OpenFlareNavItem[] = openflareSidebarNav
  .filter(
    (entry): entry is { kind: 'item' } & OpenFlareNavItem =>
      entry.kind === 'item',
  )
  .map((entry) => {
    const { kind, ...item } = entry;
    void kind;
    return item;
  });

/** 网站模块页内二级导航 */
export const openflareWebsiteSubNav = [
  { titleKey: 'websites', url: '/websites' },
  { titleKey: 'certificates', url: '/certificates' },
  { titleKey: 'dnsAccounts', url: '/dns-accounts' },
  { titleKey: 'cloudflare', url: '/cloudflare' },
  { titleKey: 'responses', url: '/responses' },
] as const;

const nonConsoleRoutePrefixes = [
  '/admin',
  '/settings',
  '/files',
  '/home',
  '/login',
  '/register',
  '/docs',
];

export function matchesNavPath(
  pathname: string,
  url: string,
  childUrls?: string[],
): boolean {
  if (url === '/') {
    return pathname === '/';
  }

  if (pathname === url || pathname.startsWith(`${url}/`)) {
    return true;
  }

  return (childUrls ?? []).some(
    (childUrl) => pathname === childUrl || pathname.startsWith(`${childUrl}/`),
  );
}

export function isNavGroupActive(
  pathname: string,
  group: OpenFlareNavGroup,
): boolean {
  return group.items.some((item) =>
    matchesNavPath(pathname, item.url, item.childUrls),
  );
}

/** 判断当前路径是否属于 OpenFlare 业务控制台 */
export function isOpenFlareConsoleRoute(pathname: string): boolean {
  if (
    nonConsoleRoutePrefixes.some(
      (prefix) => pathname === prefix || pathname.startsWith(`${prefix}/`),
    )
  ) {
    return false;
  }

  return openflareSidebarNav.some((entry) => {
    if (entry.kind === 'group') {
      return isNavGroupActive(pathname, entry);
    }

    return matchesNavPath(pathname, entry.url, entry.childUrls);
  });
}
