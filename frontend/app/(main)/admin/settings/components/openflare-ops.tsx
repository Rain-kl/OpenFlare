'use client';

import Link from 'next/link';
import { useEffect, useMemo, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import {
  Copy,
  ExternalLink,
  Loader2,
  RefreshCw,
  RotateCw,
  Save,
  Server,
} from 'lucide-react';
import { toast } from 'sonner';
import { Button } from '@/components/ui/button';
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { Switch } from '@/components/ui/switch';
import { Textarea } from '@/components/ui/textarea';
import { ErrorInline } from '@/components/layout/error';
import { LoadingStateWithBorder } from '@/components/layout/loading';
import {
  NodeService,
  OptionService,
  StatusService,
  UptimeKumaService,
} from '@/lib/services/openflare';

import {
  agentOptionEntries,
  buildDiscoveryCommand,
  defaultOpenFlareOpsFields,
  formatDurationLabel,
  getBrowserOrigin,
  mapOptionsToOpsFields,
  type OpenFlareOpsFields,
  optionsToMap,
  pagesOptionEntries,
  uptimeKumaOptionEntries,
} from './openflare-ops-utils';
import { UptimeKumaSiteSelectModal } from './uptimekuma-site-modal';

const optionsQueryKey = ['openflare', 'options'] as const;
const openflarePublicStatusQueryKey = ['openflare', 'public-status'] as const;

async function copyText(value: string) {
  await navigator.clipboard.writeText(value);
}

export function OpenFlareOpsSettings() {
  const queryClient = useQueryClient();
  const [fields, setFields] = useState<OpenFlareOpsFields>(
    defaultOpenFlareOpsFields,
  );
  const [savingSection, setSavingSection] = useState<string | null>(null);
  const [geoIPTestIP, setGeoIPTestIP] = useState('8.8.8.8');
  const [uptimeKumaModalOpen, setUptimeKumaModalOpen] = useState(false);

  const optionsQuery = useQuery({
    queryKey: optionsQueryKey,
    queryFn: () => OptionService.list(),
  });

  const statusQuery = useQuery({
    queryKey: openflarePublicStatusQueryKey,
    queryFn: () => StatusService.getPublicStatus(),
  });

  const bootstrapQuery = useQuery({
    queryKey: ['openflare', 'bootstrap-token'],
    queryFn: () => NodeService.getBootstrapToken(),
  });

  useEffect(() => {
    if (!optionsQuery.data) return;
    const optionMap = optionsToMap(optionsQuery.data);
    const serverAddress =
      optionMap.server_address ||
      statusQuery.data?.server_address ||
      getBrowserOrigin();
    setFields(mapOptionsToOpsFields(optionMap, serverAddress));
  }, [optionsQuery.data, statusQuery.data?.server_address]);

  const geoIPMutation = useMutation({
    mutationFn: () =>
      OptionService.lookupGeoIP(fields.geoip_provider, geoIPTestIP.trim()),
  });

  const saveMutation = useMutation({
    mutationFn: async ({
      section,
      entries,
    }: {
      section: string;
      entries: Array<{ key: string; value: string }>;
    }) => {
      setSavingSection(section);
      await OptionService.updateBatch(entries);
    },
    onSuccess: async () => {
      toast.success('OpenFlare 运维设置已保存');
      await queryClient.invalidateQueries({ queryKey: optionsQueryKey });
      setSavingSection(null);
    },
    onError: (error) => {
      setSavingSection(null);
      toast.error(error instanceof Error ? error.message : '保存失败');
    },
  });

  const rotateTokenMutation = useMutation({
    mutationFn: () => NodeService.rotateBootstrapToken(),
    onSuccess: async (data) => {
      toast.success('Discovery Token 已重新生成');
      await queryClient.invalidateQueries({
        queryKey: ['openflare', 'bootstrap-token'],
      });
      if (data.discovery_token) {
        try {
          await copyText(data.discovery_token);
        } catch {
          // ignore clipboard errors
        }
      }
    },
    onError: (error) => {
      toast.error(error instanceof Error ? error.message : 'Token 轮换失败');
    },
  });

  const syncUptimeKumaMutation = useMutation({
    mutationFn: () => UptimeKumaService.sync(),
    onSuccess: () => toast.success('Uptime Kuma 同步任务已执行'),
    onError: (error) =>
      toast.error(error instanceof Error ? error.message : '同步失败'),
  });

  const discoveryToken = bootstrapQuery.data?.discovery_token ?? '';
  const discoveryCommand = useMemo(() => {
    if (!fields.server_address || !discoveryToken) return '';
    return buildDiscoveryCommand(fields.server_address, discoveryToken);
  }, [discoveryToken, fields.server_address]);

  const updateField = <K extends keyof OpenFlareOpsFields>(
    key: K,
    value: OpenFlareOpsFields[K],
  ) => {
    setFields((previous) => ({ ...previous, [key]: value }));
  };

  const saveAgentSettings = () => {
    try {
      saveMutation.mutate({
        section: 'agent',
        entries: agentOptionEntries(fields),
      });
    } catch (error) {
      toast.error(error instanceof Error ? error.message : '参数校验失败');
    }
  };

  const saveUptimeKumaSettings = () => {
    try {
      saveMutation.mutate({
        section: 'uptimekuma',
        entries: uptimeKumaOptionEntries(fields),
      });
    } catch (error) {
      toast.error(error instanceof Error ? error.message : '参数校验失败');
    }
  };

  const savePagesSettings = () => {
    try {
      saveMutation.mutate({
        section: 'pages',
        entries: pagesOptionEntries(fields),
      });
    } catch (error) {
      toast.error(error instanceof Error ? error.message : '参数校验失败');
    }
  };

  if (optionsQuery.isLoading) {
    return (
      <LoadingStateWithBorder
        icon={Server}
        description='加载 OpenFlare 运维设置...'
      />
    );
  }

  if (optionsQuery.isError) {
    return (
      <ErrorInline
        message={
          optionsQuery.error instanceof Error
            ? optionsQuery.error.message
            : '加载失败'
        }
        onRetry={() => void optionsQuery.refetch()}
      />
    );
  }

  return (
    <div className='space-y-6'>
      <div className='grid gap-6 xl:grid-cols-2'>
        <Card className='border-dashed shadow-none'>
          <CardHeader className='flex flex-row items-center justify-between gap-4'>
            <div>
              <CardTitle className='text-base'>Agent 运行参数</CardTitle>
              <CardDescription>
                心跳间隔与离线阈值会在下个心跳周期同步到节点。
              </CardDescription>
            </div>
            <Button
              size='sm'
              disabled={savingSection === 'agent'}
              onClick={saveAgentSettings}
            >
              {savingSection === 'agent' ? (
                <Loader2 className='size-4 animate-spin mr-1' />
              ) : (
                <Save className='size-3.5 mr-1' />
              )}
              保存
            </Button>
          </CardHeader>
          <CardContent className='space-y-4'>
            <div className='grid gap-4 md:grid-cols-2'>
              <FieldInput
                label={`心跳间隔 (${formatDurationLabel(fields.agent_heartbeat_interval)})`}
                value={fields.agent_heartbeat_interval}
                type='number'
                onChange={(value) =>
                  updateField('agent_heartbeat_interval', value)
                }
              />
              <FieldInput
                label={`离线阈值 (${formatDurationLabel(fields.node_offline_threshold)})`}
                value={fields.node_offline_threshold}
                type='number'
                onChange={(value) =>
                  updateField('node_offline_threshold', value)
                }
              />
            </div>
            <ToggleRow
              label='开启 WS 连接升级'
              description='HTTP 心跳成功后尝试升级为 WebSocket，配置发布可即时通知。'
              checked={fields.agent_websocket_upgrade_enabled}
              onChange={(value) =>
                updateField('agent_websocket_upgrade_enabled', value)
              }
            />
            <FieldInput
              label='Agent 更新仓库'
              value={fields.agent_update_repo}
              placeholder='Rain-kl/OpenFlare'
              onChange={(value) => updateField('agent_update_repo', value)}
            />
          </CardContent>
        </Card>

        <Card className='border-dashed shadow-none'>
          <CardHeader className='flex flex-row items-center justify-between gap-4'>
            <div>
              <CardTitle className='text-base'>IP 归属解析</CardTitle>
              <CardDescription>
                控制节点地图等场景的 GeoIP 来源；访客访问记录归属地固定使用
                MaxMind mmdb。
              </CardDescription>
            </div>
            <Button
              size='sm'
              disabled={savingSection === 'agent'}
              onClick={saveAgentSettings}
            >
              {savingSection === 'agent' ? (
                <Loader2 className='size-4 animate-spin mr-1' />
              ) : (
                <Save className='size-3.5 mr-1' />
              )}
              保存
            </Button>
          </CardHeader>
          <CardContent className='space-y-4'>
            <div className='space-y-1.5'>
              <Label>归属方式</Label>
              <Select
                value={fields.geoip_provider}
                onValueChange={(value) => updateField('geoip_provider', value)}
              >
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value='disabled'>关闭</SelectItem>
                  <SelectItem value='mmdb'>MaxMind mmdb</SelectItem>
                  <SelectItem value='ip-api'>ip-api.com</SelectItem>
                  <SelectItem value='geojs'>geojs.io</SelectItem>
                  <SelectItem value='ipinfo'>ipinfo.io</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div className='flex flex-col gap-3 rounded-lg border border-dashed p-3 sm:flex-row sm:items-end'>
              <FieldInput
                label='测试 IP'
                value={geoIPTestIP}
                onChange={setGeoIPTestIP}
                placeholder='8.8.8.8'
              />
              <Button
                type='button'
                variant='outline'
                disabled={geoIPMutation.isPending}
                onClick={() => geoIPMutation.mutate()}
              >
                {geoIPMutation.isPending ? '查询中...' : '查询归属'}
              </Button>
            </div>
            {geoIPMutation.data ? (
              <div className='grid gap-2 text-sm sm:grid-cols-2'>
                <InfoCell
                  label='国家/地区'
                  value={geoIPMutation.data.name || '—'}
                />
                <InfoCell
                  label='ISO Code'
                  value={geoIPMutation.data.iso_code || '—'}
                />
              </div>
            ) : null}
          </CardContent>
        </Card>
      </div>

      <Card className='border-dashed shadow-none'>
        <CardHeader className='flex flex-row items-center justify-between gap-4'>
          <div>
            <CardTitle className='text-base'>Pages 静态托管</CardTitle>
            <CardDescription>
              配置部署包上传体积上限与每个项目的历史部署保留数量。
            </CardDescription>
          </div>
          <Button
            size='sm'
            disabled={savingSection === 'pages'}
            onClick={savePagesSettings}
          >
            {savingSection === 'pages' ? (
              <Loader2 className='size-4 animate-spin mr-1' />
            ) : (
              <Save className='size-3.5 mr-1' />
            )}
            保存
          </Button>
        </CardHeader>
        <CardContent className='space-y-4'>
          <div className='grid gap-4 md:grid-cols-2'>
            <FieldInput
              label='部署包大小上限 (MiB)'
              value={fields.pages_max_package_size_mb}
              type='number'
              onChange={(value) =>
                updateField('pages_max_package_size_mb', value)
              }
              placeholder='100'
            />
            <FieldInput
              label='历史部署保留数（0 不限制）'
              value={fields.pages_max_history_count}
              type='number'
              onChange={(value) =>
                updateField('pages_max_history_count', value)
              }
              placeholder='20'
            />
          </div>
          <p className='text-xs text-muted-foreground'>
            每个项目最多保留 N
            条部署：激活部署始终保留，其余按从新到旧填充；超出的非激活部署会在上传成功后自动清理。支持
            zip、tar.gz、tar.xz、tar.bz2、tar、7z 格式。
          </p>
        </CardContent>
      </Card>

      <Card className='border-dashed shadow-none'>
        <CardHeader className='flex flex-row items-center justify-between gap-4'>
          <div>
            <CardTitle className='text-base'>Discovery Token 与部署</CardTitle>
            <CardDescription>
              新节点首次接入使用 Discovery
              Token；轮换请前往节点管理或使用下方操作。
            </CardDescription>
          </div>
          <div className='flex flex-wrap gap-2'>
            <Button variant='outline' size='sm' asChild>
              <Link href='/nodes'>
                <ExternalLink className='size-3.5 mr-1' />
                节点管理
              </Link>
            </Button>
            <Button
              variant='outline'
              size='sm'
              disabled={rotateTokenMutation.isPending}
              onClick={() => rotateTokenMutation.mutate()}
            >
              <RotateCw className='size-3.5 mr-1' />
              轮换 Token
            </Button>
          </div>
        </CardHeader>
        <CardContent className='space-y-4'>
          <FieldInput
            label='Server URL'
            value={fields.server_address}
            onChange={(value) => updateField('server_address', value)}
            placeholder='https://yourdomain.com'
          />
          <div className='rounded-lg border border-dashed p-3'>
            <p className='text-[10px] uppercase tracking-wider text-muted-foreground'>
              Discovery Token（只读）
            </p>
            <p className='mt-2 break-all text-sm font-mono'>
              {bootstrapQuery.isLoading
                ? '加载中...'
                : discoveryToken || '未生成'}
            </p>
          </div>
          <div className='space-y-1.5'>
            <Label>一键部署命令</Label>
            <Textarea
              readOnly
              value={
                discoveryCommand || '请先填写可访问的 Server URL 并获取 Token。'
              }
              className='min-h-24 font-mono text-xs'
            />
            {discoveryCommand ? (
              <Button
                type='button'
                variant='outline'
                size='sm'
                onClick={() =>
                  void copyText(discoveryCommand).then(() =>
                    toast.success('命令已复制'),
                  )
                }
              >
                <Copy className='size-3.5 mr-1' />
                复制命令
              </Button>
            ) : null}
          </div>
        </CardContent>
      </Card>

      <Card className='border-dashed shadow-none'>
        <CardHeader className='flex flex-row items-center justify-between gap-4'>
          <div>
            <CardTitle className='text-base'>Uptime Kuma 集成</CardTitle>
            <CardDescription>
              将反代站点差分同步至 Uptime Kuma 监控实例。
            </CardDescription>
          </div>
          <div className='flex flex-wrap gap-2'>
            <Button
              variant='outline'
              size='sm'
              disabled={
                !fields.uptime_kuma_enabled || syncUptimeKumaMutation.isPending
              }
              onClick={() => syncUptimeKumaMutation.mutate()}
            >
              <RefreshCw className='size-3.5 mr-1' />
              立即同步
            </Button>
            <Button
              size='sm'
              disabled={savingSection === 'uptimekuma'}
              onClick={saveUptimeKumaSettings}
            >
              保存
            </Button>
          </div>
        </CardHeader>
        <CardContent className='space-y-4'>
          <ToggleRow
            label='开启 Uptime Kuma'
            checked={fields.uptime_kuma_enabled}
            onChange={(value) => updateField('uptime_kuma_enabled', value)}
          />
          {fields.uptime_kuma_enabled ? (
            <>
              <div className='grid gap-4 md:grid-cols-2'>
                <FieldInput
                  label='Uptime Kuma 地址'
                  value={fields.uptime_kuma_url}
                  onChange={(value) => updateField('uptime_kuma_url', value)}
                  placeholder='http://localhost:3001'
                />
                <FieldInput
                  label='用户名'
                  value={fields.uptime_kuma_username}
                  onChange={(value) =>
                    updateField('uptime_kuma_username', value)
                  }
                />
                <FieldInput
                  label='密码'
                  value={fields.uptime_kuma_password}
                  type='password'
                  onChange={(value) =>
                    updateField('uptime_kuma_password', value)
                  }
                  placeholder='留空表示不更新'
                />
                <FieldInput
                  label='同步间隔 (分钟)'
                  value={fields.uptime_kuma_sync_interval}
                  type='number'
                  onChange={(value) =>
                    updateField('uptime_kuma_sync_interval', value)
                  }
                />
                <div className='space-y-1.5'>
                  <Label>监控范围</Label>
                  <Select
                    value={fields.uptime_kuma_monitor_scope}
                    onValueChange={(value) =>
                      updateField('uptime_kuma_monitor_scope', value)
                    }
                  >
                    <SelectTrigger>
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value='all'>全部站点</SelectItem>
                      <SelectItem value='selected'>选择站点</SelectItem>
                    </SelectContent>
                  </Select>
                </div>
              </div>
              {fields.uptime_kuma_monitor_scope === 'selected' ? (
                <div className='rounded-lg border border-dashed p-3'>
                  <div className='flex items-center justify-between gap-2'>
                    <p className='text-sm font-medium'>已选站点</p>
                    <Button
                      type='button'
                      variant='outline'
                      size='sm'
                      onClick={() => setUptimeKumaModalOpen(true)}
                    >
                      选择监控站点
                    </Button>
                  </div>
                  <p className='mt-2 break-all text-xs text-muted-foreground'>
                    {fields.uptime_kuma_selected_sites
                      ? fields.uptime_kuma_selected_sites.split(',').join(', ')
                      : '未选择任何站点'}
                  </p>
                </div>
              ) : null}
              <div className='grid gap-4 md:grid-cols-2'>
                <FieldInput
                  label='检测频率 (秒)'
                  value={fields.uptime_kuma_interval}
                  type='number'
                  onChange={(value) =>
                    updateField('uptime_kuma_interval', value)
                  }
                />
                <FieldInput
                  label='重试次数'
                  value={fields.uptime_kuma_retry}
                  type='number'
                  onChange={(value) => updateField('uptime_kuma_retry', value)}
                />
                <FieldInput
                  label='重试间隔 (秒)'
                  value={fields.uptime_kuma_retry_interval}
                  type='number'
                  onChange={(value) =>
                    updateField('uptime_kuma_retry_interval', value)
                  }
                />
                <FieldInput
                  label='请求超时 (秒)'
                  value={fields.uptime_kuma_timeout}
                  type='number'
                  onChange={(value) =>
                    updateField('uptime_kuma_timeout', value)
                  }
                />
              </div>
            </>
          ) : null}
        </CardContent>
      </Card>

      <UptimeKumaSiteSelectModal
        open={uptimeKumaModalOpen}
        selectedSites={
          fields.uptime_kuma_selected_sites
            ? fields.uptime_kuma_selected_sites.split(',')
            : []
        }
        onOpenChange={setUptimeKumaModalOpen}
        onSave={(sites) =>
          updateField('uptime_kuma_selected_sites', sites.join(','))
        }
      />
    </div>
  );
}

function FieldInput({
  label,
  value,
  onChange,
  type = 'text',
  placeholder,
}: {
  label: string;
  value: string;
  onChange: (value: string) => void;
  type?: string;
  placeholder?: string;
}) {
  return (
    <div className='space-y-1.5'>
      <Label>{label}</Label>
      <Input
        type={type}
        value={value}
        placeholder={placeholder}
        onChange={(event) => onChange(event.target.value)}
        className='h-9 text-xs'
      />
    </div>
  );
}

function ToggleRow({
  label,
  description,
  checked,
  onChange,
}: {
  label: string;
  description?: string;
  checked: boolean;
  onChange: (value: boolean) => void;
}) {
  return (
    <div className='flex items-center justify-between gap-3 rounded-lg border border-dashed px-3 py-2'>
      <div>
        <Label className='text-xs'>{label}</Label>
        {description ? (
          <p className='mt-0.5 text-[11px] text-muted-foreground'>
            {description}
          </p>
        ) : null}
      </div>
      <Switch checked={checked} onCheckedChange={onChange} />
    </div>
  );
}

function InfoCell({ label, value }: { label: string; value: string }) {
  return (
    <div className='rounded-lg border border-dashed px-3 py-2'>
      <p className='text-[10px] uppercase tracking-wider text-muted-foreground'>
        {label}
      </p>
      <p className='mt-2 text-sm font-medium break-all'>{value}</p>
    </div>
  );
}
