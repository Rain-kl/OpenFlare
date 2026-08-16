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
import { useTranslations } from 'next-intl';

interface MenuItem {
  path: string;
  labelKey: string;
  descKey: string;
  icon: ComponentType<{ className?: string }>;
  readOnly?: boolean;
}

interface MenuGroup {
  nameKey: string;
  items: MenuItem[];
}

const MENU_GROUPS: MenuGroup[] = [
  {
    nameKey: 'groupBusiness',
    items: [
      {
        path: '/',
        labelKey: 'dashboard',
        descKey: 'descDashboard',
        icon: LayoutDashboard,
        readOnly: true,
      },
      {
        path: '/nodes',
        labelKey: 'nodes',
        descKey: 'descNodes',
        icon: Server,
      },
      {
        path: '/proxy-routes',
        labelKey: 'proxyRoutes',
        descKey: 'descProxyRoutes',
        icon: Route,
      },
      {
        path: '/websites',
        labelKey: 'websites',
        descKey: 'descWebsites',
        icon: Globe,
      },
      {
        path: '/certificates',
        labelKey: 'certificates',
        descKey: 'descCertificates',
        icon: ShieldCheck,
      },
      {
        path: '/dns-accounts',
        labelKey: 'dnsAccounts',
        descKey: 'descDnsAccounts',
        icon: Settings,
      },
      {
        path: '/origins',
        labelKey: 'origins',
        descKey: 'descOrigins',
        icon: Home,
      },
      {
        path: '/waf',
        labelKey: 'waf',
        descKey: 'descWaf',
        icon: ShieldCheck,
      },
      {
        path: '/ip-groups',
        labelKey: 'ipGroups',
        descKey: 'descIpGroups',
        icon: Layers,
      },
      {
        path: '/pages',
        labelKey: 'pages',
        descKey: 'descPages',
        icon: FileText,
      },
      {
        path: '/config-versions',
        labelKey: 'configVersions',
        descKey: 'descConfigVersions',
        icon: GitBranch,
      },
      {
        path: '/access-logs',
        labelKey: 'accessLogs',
        descKey: 'descAccessLogs',
        icon: ScrollText,
      },
      {
        path: '/performance',
        labelKey: 'performance',
        descKey: 'descPerformance',
        icon: Gauge,
      },
    ],
  },
  {
    nameKey: 'groupAdmin',
    items: [
      {
        path: '/admin/users',
        labelKey: 'users',
        descKey: 'descUsers',
        icon: UserRound,
      },
      {
        path: '/admin/tasks',
        labelKey: 'tasks',
        descKey: 'descTasks',
        icon: Layers,
      },
      {
        path: '/admin/files',
        labelKey: 'storage',
        descKey: 'descStorage',
        icon: FolderOpen,
      },
      {
        path: '/admin/database',
        labelKey: 'database',
        descKey: 'descDatabase',
        icon: Database,
      },
      {
        path: '/admin/push',
        labelKey: 'push',
        descKey: 'descPush',
        icon: Bell,
      },
      {
        path: '/admin/logs',
        labelKey: 'logs',
        descKey: 'descLogs',
        icon: Terminal,
      },
      {
        path: '/admin/system',
        labelKey: 'system',
        descKey: 'descSystem',
        icon: ShieldCheck,
      },
      {
        path: '/admin/settings',
        labelKey: 'adminSettings',
        descKey: 'descAdminSettings',
        icon: Settings,
        readOnly: true,
      },
    ],
  },
  {
    nameKey: 'groupDocs',
    items: [
      {
        path: 'https://openflare.fyrn.link/',
        labelKey: 'usageDocs',
        descKey: 'descUsageDocs',
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
  const t = useTranslations('settings.other');
  const tNav = useTranslations('layout.nav');

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
      toast.success(t('menuDisplayUpdated'));
    },
    onError: (error: Error) => {
      toast.error(error.message || t('updateConfigFailed'));
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
              {t('menuDisplayManagement')}
            </CardTitle>
            <CardDescription className='text-xs'>
              {t('menuDisplayManagementDesc')}
            </CardDescription>
          </div>
        </div>
      </CardHeader>
      <CardContent className='pt-6 space-y-6'>
        {MENU_GROUPS.map((group) => (
          <div key={group.nameKey} className='space-y-3'>
            <div className='flex items-center gap-2'>
              <span className='text-xs font-semibold text-muted-foreground tracking-wider uppercase'>
                {t(group.nameKey)}
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
                          {tNav(item.labelKey)}
                        </span>
                        {isReadOnly && (
                          <span className='text-[9px] px-1.5 py-0.5 rounded bg-muted text-muted-foreground border shrink-0'>
                            {t('notHideable')}
                          </span>
                        )}
                      </div>
                      <p className='text-xs text-muted-foreground leading-normal line-clamp-2'>
                        {t(item.descKey)}
                      </p>
                    </div>
                    <div className='flex items-center'>
                      <Switch
                        aria-label={tNav(item.labelKey)}
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
            <span className='font-semibold text-foreground'>
              {t('securityTipTitle')}
            </span>
            {t('securityTipDesc')}
          </div>
        </div>
      </CardContent>
    </Card>
  );
}
