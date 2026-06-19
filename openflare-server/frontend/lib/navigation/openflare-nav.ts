import type {LucideIcon} from 'lucide-react';
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
  title: string;
  url: string;
  icon: LucideIcon;
  /** 子页面在侧栏中仍高亮父级菜单项 */
  childUrls?: string[];
}

export interface OpenFlareNavSubItem {
  title: string;
  url: string;
  childUrls?: string[];
}

export interface OpenFlareNavGroup {
  title: string;
  icon: LucideIcon;
  items: OpenFlareNavSubItem[];
}

export type OpenFlareSidebarNavEntry =
  | ({kind: 'item'} & OpenFlareNavItem)
  | ({kind: 'group'} & OpenFlareNavGroup);

/** 安全性折叠组 */
export const openflareSecurityNavGroup: OpenFlareNavGroup = {
  title: '安全性',
  icon: ShieldCheck,
  items: [
    {title: 'WAF', url: '/waf'},
    {title: 'IP 组', url: '/waf/ip-groups'},
  ],
};

/** 网站管理折叠组 */
export const openflareWebsiteNavGroup: OpenFlareNavGroup = {
  title: '网站管理',
  icon: Globe,
  items: [
    {title: '域名列表', url: '/websites', childUrls: ['/websites/detail']},
    {title: 'TLS证书', url: '/websites/certificates'},
    {title: 'DNS账号', url: '/websites/dns-accounts'},
    {title: '源站地址', url: '/origins', childUrls: ['/origins/detail']},
  ],
};

/**
 * OpenFlare 侧栏导航顺序（单一配置源）。
 * 调整菜单顺序或折叠组位置时，只需修改此数组。
 */
export const openflareSidebarNav: OpenFlareSidebarNavEntry[] = [
  {kind: 'item', title: '数据看板', url: '/', icon: LayoutDashboard},
  {kind: 'item', title: '节点管理', url: '/nodes', icon: Server, childUrls: ['/nodes/detail']},
  {kind: 'item', title: '规则管理', url: '/proxy-routes', icon: Route, childUrls: ['/proxy-routes/detail']},
  {kind: 'group', ...openflareWebsiteNavGroup},
  {kind: 'group', ...openflareSecurityNavGroup},
  {kind: 'item', title: 'Pages', url: '/pages', icon: FileText, childUrls: ['/pages/detail']},
  {kind: 'item', title: '版本发布', url: '/config-versions', icon: GitBranch},
  {kind: 'item', title: '访问日志', url: '/access-logs', icon: ScrollText},
  {kind: 'item', title: '性能调优', url: '/performance', icon: Gauge},
];

/** 扁平菜单项（供路由判断等逻辑复用） */
export const openflareNavItems: OpenFlareNavItem[] = openflareSidebarNav
  .filter((entry): entry is {kind: 'item'} & OpenFlareNavItem => entry.kind === 'item')
  .map(({kind: _kind, ...item}) => item);

/** 网站模块页内二级导航 */
export const openflareWebsiteSubNav = [
  {title: '网站列表', url: '/websites'},
  {title: '证书', url: '/websites/certificates'},
  {title: 'DNS 账号', url: '/websites/dns-accounts'},
] as const;

const nonConsoleRoutePrefixes = ['/admin', '/settings', '/files', '/home', '/login', '/register', '/docs'];

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

export function isNavGroupActive(pathname: string, group: OpenFlareNavGroup): boolean {
  return group.items.some((item) => matchesNavPath(pathname, item.url, item.childUrls));
}

/** 判断当前路径是否属于 OpenFlare 业务控制台 */
export function isOpenFlareConsoleRoute(pathname: string): boolean {
  if (nonConsoleRoutePrefixes.some((prefix) => pathname === prefix || pathname.startsWith(`${prefix}/`))) {
    return false;
  }

  return openflareSidebarNav.some((entry) => {
    if (entry.kind === 'group') {
      return isNavGroupActive(pathname, entry);
    }

    return matchesNavPath(pathname, entry.url, entry.childUrls);
  });
}
