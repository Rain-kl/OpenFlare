import type { NavigationItem } from '@/types/navigation';

export const dashboardNavigation: NavigationItem[] = [
  {
    href: '/',
    label: '总览',
    shortLabel: '总览',
    description: '查看阶段推进情况、系统入口与模块骨架状态。',
  },
  {
    href: '/proxy-routes',
    label: '反代规则',
    shortLabel: '规则',
    description: '已接入列表、表单、证书匹配与发布动作。',
  },
  {
    href: '/config-versions',
    label: '配置版本',
    shortLabel: '版本',
    description: '已接入版本预览、发布前 diff 与重新激活。',
  },
  {
    href: '/nodes',
    label: '节点管理',
    shortLabel: '节点',
    description: '已接入状态标签、部署命令、心跳与更新动作。',
  },
  {
    href: '/apply-logs',
    label: '应用记录',
    shortLabel: '记录',
    description: '阶段 3 接入筛选、分页与详情展示。',
  },
  {
    href: '/managed-domains',
    label: '域名管理',
    shortLabel: '域名',
    description: '阶段 3 接入证书绑定与启用状态切换。',
  },
  {
    href: '/tls-certificates',
    label: 'TLS 证书',
    shortLabel: '证书',
    description: '阶段 3 接入导入、上传与有效期展示。',
  },
  {
    href: '/files',
    label: '文件管理',
    shortLabel: '文件',
    description: '已接入上传、下载、复制链接、搜索与删除动作。',
  },
  {
    href: '/users',
    label: '用户管理',
    shortLabel: '用户',
    description: '已接入搜索、创建、编辑、封禁与权限调整。',
  },
  {
    href: '/settings',
    label: '设置',
    shortLabel: '设置',
    description: '已接入个人、运维、系统与关于内容维护。',
  },
];
