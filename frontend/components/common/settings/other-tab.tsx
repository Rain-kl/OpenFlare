'use client';

import { ComponentType, useMemo } from 'react';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import {
  Bell,
  Database,
  FileText,
  FolderOpen,
  Home,
  Info,
  Layers,
  LayoutList,
  Settings,
  ShieldCheck,
  Terminal,
  UserRound,
  LayoutDashboard,
  Route,
  Server,
  Globe,
  GitBranch,
  ScrollText,
  Gauge,
} from 'lucide-react';

import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card';
import { Switch } from '@/components/ui/switch';
import services from '@/lib/services';
import type { SystemConfig } from '@/lib/services/admin';
import { toast } from 'sonner';

interface MenuItem {
  path: string;
  label: string;
  description: string;
  icon: ComponentType<{ className?: string }>;
  readOnly?: boolean;
}

interface MenuGroup {
  name: string;
  items: MenuItem[];
}

const MENU_GROUPS: MenuGroup[] = [
  {
    name: '业务菜单',
    items: [
      {
        path: '/',
        label: '数据看板',
        description: '查看边缘节点请求、访客与流量等多维度图表看板',
        icon: LayoutDashboard,
        readOnly: true,
      },
      {
        path: '/nodes',
        label: '节点管理',
        description: '注册与管理反向代理边缘节点服务状态',
        icon: Server,
      },
      {
        path: '/proxy-routes',
        label: '规则管理',
        description: '配置和发布反代路由规则、负载均衡与源站绑定',
        icon: Route,
      },
      {
        path: '/websites',
        label: '域名列表',
        description: '配置站点根域名并查看关联网站详情',
        icon: Globe,
      },
      {
        path: '/certificates',
        label: 'TLS证书',
        description: '管理和下发网站 SSL/TLS 证书',
        icon: ShieldCheck,
      },
      {
        path: '/dns-accounts',
        label: 'DNS账号',
        description: '配置 Cloudflare/Alidns 等 DNS 服务商 API 凭证',
        icon: Settings,
      },
      {
        path: '/origins',
        label: '源站地址',
        description: '集中维护和管理后端业务源站服务器组',
        icon: Home,
      },
      {
        path: '/waf',
        label: 'WAF 管理',
        description: '配置 Web 应用防火墙自定义规则与频率限制',
        icon: ShieldCheck,
      },
      {
        path: '/ip-groups',
        label: 'IP 组',
        description: '管理黑白名单 IP 集合，用于 WAF 规则过滤',
        icon: Layers,
      },
      {
        path: '/pages',
        label: 'Pages 静态站',
        description: '托管和发布静态网页项目，支持自动关联域名',
        icon: FileText,
      },
      {
        path: '/config-versions',
        label: '版本发布',
        description: '查看规则快照版本并进行配置热更新分发与回滚',
        icon: GitBranch,
      },
      {
        path: '/access-logs',
        label: '访问日志',
        description: '多维度实时浏览边缘节点请求访问明细',
        icon: ScrollText,
      },
      {
        path: '/performance',
        label: '性能调优',
        description: '配置缓存、图片压缩等页面性能加速策略',
        icon: Gauge,
      },
    ],
  },
  {
    name: '管理菜单',
    items: [
      {
        path: '/admin/users',
        label: '用户管理',
        description: '查看和管理系统用户列表及其状态',
        icon: UserRound,
      },
      {
        path: '/admin/tasks',
        label: '任务管理',
        description: '查看和调度系统异步及定时任务',
        icon: Layers,
      },
      {
        path: '/admin/files',
        label: '存储管理',
        description: '管理系统文件存储和清理无用文件',
        icon: FolderOpen,
      },
      {
        path: '/admin/database',
        label: '数据管理',
        description: '监控物理数据库状态、分页浏览表数据及交互式 SQL 查询',
        icon: Database,
      },
      {
        path: '/admin/push',
        label: '通知推送',
        description: '配置和发送系统通知及推送消息',
        icon: Bell,
      },
      {
        path: '/admin/logs',
        label: '系统日志',
        description: '查看异步任务执行日志和系统运行情况',
        icon: Terminal,
      },
      {
        path: '/admin/system',
        label: '系统配置',
        description: '管理和维护系统基础键值对配置',
        icon: ShieldCheck,
      },
      {
        path: '/admin/settings',
        label: '系统设置',
        description: '配置安全验证、邮箱服务及目录显示',
        icon: Settings,
        readOnly: true,
      },
    ],
  },
  {
    name: '文档菜单',
    items: [
      {
        path: 'https://open-flare.pages.dev/',
        label: '使用文档',
        description: '面向开发与运营的部署使用指南',
        icon: FileText,
      },
    ],
  },
];

interface OtherTabProps {
  configs: Record<string, SystemConfig>;
}

export function OtherTab({ configs }: OtherTabProps) {
  const queryClient = useQueryClient();

  const menuDisplayConfig = useMemo(() => {
    const raw = configs['menu_display_config']?.value;
    if (!raw) return {} as Record<string, boolean>;
    try {
      return JSON.parse(raw) as Record<string, boolean>;
    } catch {
      return {} as Record<string, boolean>;
    }
  }, [configs]);

  const updateMenuConfigMutation = useMutation({
    mutationFn: async ({
      path,
      enabled,
    }: {
      path: string;
      enabled: boolean;
    }) => {
      const newConfig = { ...menuDisplayConfig, [path]: enabled };
      const currentCfg = configs['menu_display_config'];
      await services.adminSystemConfig.updateSystemConfig(
        'menu_display_config',
        {
          value: JSON.stringify(newConfig),
          description:
            currentCfg?.description ||
            '目录显示配置（JSON 字符串，格式为 {url: enabled}）',
        },
      );
    },
    onSuccess: async () => {
      await queryClient.invalidateQueries({
        queryKey: ['admin', 'system-configs'],
      });
      await queryClient.invalidateQueries({ queryKey: ['public-config'] });
      toast.success('目录显示配置已更新');
    },
    onError: (error: Error) => {
      toast.error(error.message || '更新配置失败');
    },
  });

  const handleMenuToggle = (path: string, checked: boolean) => {
    updateMenuConfigMutation.mutate({ path, enabled: checked });
  };

  return (
    <Card className='border border-dashed shadow-sm'>
      <CardHeader className='border-b border-dashed pb-4'>
        <div className='flex items-center gap-2'>
          <div className='p-1.5 rounded-lg bg-primary/10 text-primary'>
            <LayoutList className='size-4' />
          </div>
          <div>
            <CardTitle className='text-base font-semibold'>
              目录显示管理
            </CardTitle>
            <CardDescription className='text-xs'>
              配置系统左侧菜单的显示与隐藏状态，适用于所有登录用户。
            </CardDescription>
          </div>
        </div>
      </CardHeader>
      <CardContent className='pt-6 space-y-6'>
        {MENU_GROUPS.map((group) => (
          <div key={group.name} className='space-y-3'>
            <div className='flex items-center gap-2'>
              <span className='text-xs font-semibold text-muted-foreground tracking-wider uppercase'>
                {group.name}
              </span>
              <div className='h-px bg-border/40 flex-1' />
            </div>
            <div className='grid grid-cols-1 md:grid-cols-2 gap-4'>
              {group.items.map((item) => {
                const Icon = item.icon;
                const isReadOnly = !!item.readOnly;
                const checked = menuDisplayConfig[item.path] !== false;

                return (
                  <div
                    key={item.path}
                    className='flex items-center justify-between gap-4 rounded-xl border border-dashed p-4 bg-card hover:bg-muted/10 hover:border-primary/30 transition-all duration-300 shadow-sm'
                  >
                    <div className='space-y-1.5 flex-1 min-w-0 pr-2'>
                      <div className='flex items-center gap-2'>
                        {Icon && (
                          <Icon className='size-4 text-primary shrink-0' />
                        )}
                        <span className='font-medium text-sm text-foreground truncate'>
                          {item.label}
                        </span>
                        {isReadOnly && (
                          <span className='text-[9px] px-1.5 py-0.5 rounded bg-muted text-muted-foreground border shrink-0'>
                            不可隐藏
                          </span>
                        )}
                      </div>
                      <p className='text-xs text-muted-foreground leading-normal line-clamp-2'>
                        {item.description}
                      </p>
                    </div>
                    <div className='flex items-center'>
                      <Switch
                        checked={checked}
                        disabled={
                          isReadOnly || updateMenuConfigMutation.isPending
                        }
                        onCheckedChange={(val) =>
                          handleMenuToggle(item.path, val)
                        }
                      />
                    </div>
                  </div>
                );
              })}
            </div>
          </div>
        ))}

        <div className='p-3.5 rounded-lg border border-dashed border-primary/20 bg-primary/5 flex items-start gap-2.5'>
          <Info className='size-4 text-primary shrink-0 mt-0.5' />
          <div className='text-xs text-muted-foreground leading-relaxed'>
            <span className='font-semibold text-foreground'>安全提示：</span>
            为了防止管理员在关闭“系统设置”后导致无法重新访问此配置页，系统限制了“系统设置”的关闭权限。其它所有菜单均可自由开关，隐藏后对应的分组标题在为空时也会自动隐藏。
          </div>
        </div>
      </CardContent>
    </Card>
  );
}
