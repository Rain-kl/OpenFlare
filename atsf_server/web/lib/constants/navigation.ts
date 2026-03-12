import type { NavigationItem } from '@/types/navigation';

export const dashboardNavigation: NavigationItem[] = [
  {
    href: '/',
    label: '总览',
    icon: 'home',
  },
  {
    href: '/node',
    label: '节点',
    icon: 'node',
  },
  {
    href: '/proxy-route',
    label: '规则',
    icon: 'proxy',
  },
  {
    href: '/config-version',
    label: '发布',
    icon: 'release',
  },
  {
    href: '/website',
    label: '网站',
    icon: 'website',
  },
  {
    href: '/user',
    label: '用户',
    icon: 'user',
  },
  {
    href: '/performance',
    label: '性能',
    icon: 'performance',
  },
  {
    href: '/setting',
    label: '设置',
    icon: 'setting',
  },
];
