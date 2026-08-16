'use client';

import Link from 'next/link';
import { useTranslations } from 'next-intl';
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
  const t = useTranslations('openflareOps');
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
      toast.success(t('saved'));
      await queryClient.invalidateQueries({ queryKey: optionsQueryKey });
      setSavingSection(null);
    },
    onError: (error) => {
      setSavingSection(null);
      toast.error(error instanceof Error ? error.message : t('saveFailed'));
    },
  });

  const rotateTokenMutation = useMutation({
    mutationFn: () => NodeService.rotateBootstrapToken(),
    onSuccess: async (data) => {
      toast.success(t('tokenRotated'));
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
      toast.error(
        error instanceof Error ? error.message : t('tokenRotateFailed'),
      );
    },
  });

  const syncUptimeKumaMutation = useMutation({
    mutationFn: () => UptimeKumaService.sync(),
    onSuccess: () => toast.success(t('kumaSynced')),
    onError: (error) =>
      toast.error(error instanceof Error ? error.message : t('syncFailed')),
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
        entries: agentOptionEntries(fields, t),
      });
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t('invalidParams'));
    }
  };

  const saveUptimeKumaSettings = () => {
    try {
      saveMutation.mutate({
        section: 'uptimekuma',
        entries: uptimeKumaOptionEntries(fields, t),
      });
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t('invalidParams'));
    }
  };

  const savePagesSettings = () => {
    try {
      saveMutation.mutate({
        section: 'pages',
        entries: pagesOptionEntries(fields, t),
      });
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t('invalidParams'));
    }
  };

  if (optionsQuery.isLoading) {
    return (
      <LoadingStateWithBorder icon={Server} description={t('loading')} />
    );
  }

  if (optionsQuery.isError) {
    return (
      <ErrorInline
        message={
          optionsQuery.error instanceof Error
            ? optionsQuery.error.message
            : t('loadFailed')
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
              <CardTitle className='text-base'>{t('agent.title')}</CardTitle>
              <CardDescription>{t('agent.description')}</CardDescription>
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
              {t('save')}
            </Button>
          </CardHeader>
          <CardContent className='space-y-4'>
            <div className='grid gap-4 md:grid-cols-2'>
              <FieldInput
                label={t('agent.heartbeat', {
                  duration: formatDurationLabel(
                    fields.agent_heartbeat_interval,
                    t,
                  ),
                })}
                value={fields.agent_heartbeat_interval}
                type='number'
                onChange={(value) =>
                  updateField('agent_heartbeat_interval', value)
                }
              />
              <FieldInput
                label={t('agent.offline', {
                  duration: formatDurationLabel(
                    fields.node_offline_threshold,
                    t,
                  ),
                })}
                value={fields.node_offline_threshold}
                type='number'
                onChange={(value) =>
                  updateField('node_offline_threshold', value)
                }
              />
            </div>
            <ToggleRow
              label={t('agent.wsUpgrade')}
              description={t('agent.wsUpgradeDesc')}
              checked={fields.agent_websocket_upgrade_enabled}
              onChange={(value) =>
                updateField('agent_websocket_upgrade_enabled', value)
              }
            />
            <FieldInput
              label={t('agent.updateRepo')}
              value={fields.agent_update_repo}
              placeholder='Rain-kl/OpenFlare'
              onChange={(value) => updateField('agent_update_repo', value)}
            />
          </CardContent>
        </Card>

        <Card className='border-dashed shadow-none'>
          <CardHeader className='flex flex-row items-center justify-between gap-4'>
            <div>
              <CardTitle className='text-base'>{t('geo.title')}</CardTitle>
              <CardDescription>{t('geo.description')}</CardDescription>
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
              {t('save')}
            </Button>
          </CardHeader>
          <CardContent className='space-y-4'>
            <div className='space-y-1.5'>
              <Label>{t('geo.mode')}</Label>
              <Select
                value={fields.geoip_provider}
                onValueChange={(value) => updateField('geoip_provider', value)}
              >
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value='disabled'>{t('geo.disabled')}</SelectItem>
                  <SelectItem value='mmdb'>MaxMind mmdb</SelectItem>
                  <SelectItem value='ip-api'>ip-api.com</SelectItem>
                  <SelectItem value='geojs'>geojs.io</SelectItem>
                  <SelectItem value='ipinfo'>ipinfo.io</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div className='flex flex-col gap-3 rounded-lg border border-dashed p-3 sm:flex-row sm:items-end'>
              <FieldInput
                label={t('geo.testIp')}
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
                {geoIPMutation.isPending ? t('geo.querying') : t('geo.query')}
              </Button>
            </div>
            {geoIPMutation.data ? (
              <div className='grid gap-2 text-sm sm:grid-cols-2'>
                <InfoCell
                  label={t('geo.country')}
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
            <CardTitle className='text-base'>{t('pages.title')}</CardTitle>
            <CardDescription>{t('pages.description')}</CardDescription>
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
            {t('save')}
          </Button>
        </CardHeader>
        <CardContent className='space-y-4'>
          <div className='grid gap-4 md:grid-cols-2'>
            <FieldInput
              label={t('pages.maxSize')}
              value={fields.pages_max_package_size_mb}
              type='number'
              onChange={(value) =>
                updateField('pages_max_package_size_mb', value)
              }
              placeholder='100'
            />
            <FieldInput
              label={t('pages.history')}
              value={fields.pages_max_history_count}
              type='number'
              onChange={(value) =>
                updateField('pages_max_history_count', value)
              }
              placeholder='20'
            />
          </div>
          <p className='text-xs text-muted-foreground'>
            {t('pages.historyHint')}
          </p>
        </CardContent>
      </Card>

      <Card className='border-dashed shadow-none'>
        <CardHeader className='flex flex-row items-center justify-between gap-4'>
          <div>
            <CardTitle className='text-base'>
              {t('discovery.title')}
            </CardTitle>
            <CardDescription>{t('discovery.description')}</CardDescription>
          </div>
          <div className='flex flex-wrap gap-2'>
            <Button variant='outline' size='sm' asChild>
              <Link href='/nodes'>
                <ExternalLink className='size-3.5 mr-1' />
                {t('discovery.manageNodes')}
              </Link>
            </Button>
            <Button
              variant='outline'
              size='sm'
              disabled={rotateTokenMutation.isPending}
              onClick={() => rotateTokenMutation.mutate()}
            >
              <RotateCw className='size-3.5 mr-1' />
              {t('discovery.rotate')}
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
              {t('discovery.tokenReadonly')}
            </p>
            <p className='mt-2 break-all text-sm font-mono'>
              {bootstrapQuery.isLoading
                ? t('discovery.loading')
                : discoveryToken || t('discovery.notGenerated')}
            </p>
          </div>
          <div className='space-y-1.5'>
            <Label>{t('discovery.command')}</Label>
            <Textarea
              readOnly
              value={discoveryCommand || t('discovery.commandHint')}
              className='min-h-24 font-mono text-xs'
            />
            {discoveryCommand ? (
              <Button
                type='button'
                variant='outline'
                size='sm'
                onClick={() =>
                  void copyText(discoveryCommand).then(() =>
                    toast.success(t('discovery.copied')),
                  )
                }
              >
                <Copy className='size-3.5 mr-1' />
                {t('discovery.copy')}
              </Button>
            ) : null}
          </div>
        </CardContent>
      </Card>

      <Card className='border-dashed shadow-none'>
        <CardHeader className='flex flex-row items-center justify-between gap-4'>
          <div>
            <CardTitle className='text-base'>{t('kuma.title')}</CardTitle>
            <CardDescription>{t('kuma.description')}</CardDescription>
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
              {t('kuma.syncNow')}
            </Button>
            <Button
              size='sm'
              disabled={savingSection === 'uptimekuma'}
              onClick={saveUptimeKumaSettings}
            >
              {t('save')}
            </Button>
          </div>
        </CardHeader>
        <CardContent className='space-y-4'>
          <ToggleRow
            label={t('kuma.enable')}
            checked={fields.uptime_kuma_enabled}
            onChange={(value) => updateField('uptime_kuma_enabled', value)}
          />
          {fields.uptime_kuma_enabled ? (
            <>
              <div className='grid gap-4 md:grid-cols-2'>
                <FieldInput
                  label={t('kuma.url')}
                  value={fields.uptime_kuma_url}
                  onChange={(value) => updateField('uptime_kuma_url', value)}
                  placeholder='http://localhost:3001'
                />
                <FieldInput
                  label={t('kuma.username')}
                  value={fields.uptime_kuma_username}
                  onChange={(value) =>
                    updateField('uptime_kuma_username', value)
                  }
                />
                <FieldInput
                  label={t('kuma.password')}
                  value={fields.uptime_kuma_password}
                  type='password'
                  onChange={(value) =>
                    updateField('uptime_kuma_password', value)
                  }
                  placeholder={t('kuma.passwordKeep')}
                />
                <FieldInput
                  label={t('kuma.syncInterval')}
                  value={fields.uptime_kuma_sync_interval}
                  type='number'
                  onChange={(value) =>
                    updateField('uptime_kuma_sync_interval', value)
                  }
                />
                <div className='space-y-1.5'>
                  <Label>{t('kuma.scope')}</Label>
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
                      <SelectItem value='all'>{t('kuma.allSites')}</SelectItem>
                      <SelectItem value='selected'>
                        {t('kuma.selectedSites')}
                      </SelectItem>
                    </SelectContent>
                  </Select>
                </div>
              </div>
              {fields.uptime_kuma_monitor_scope === 'selected' ? (
                <div className='rounded-lg border border-dashed p-3'>
                  <div className='flex items-center justify-between gap-2'>
                    <p className='text-sm font-medium'>
                      {t('kuma.selectedLabel')}
                    </p>
                    <Button
                      type='button'
                      variant='outline'
                      size='sm'
                      onClick={() => setUptimeKumaModalOpen(true)}
                    >
                      {t('kuma.pickSites')}
                    </Button>
                  </div>
                  <p className='mt-2 break-all text-xs text-muted-foreground'>
                    {fields.uptime_kuma_selected_sites
                      ? fields.uptime_kuma_selected_sites.split(',').join(', ')
                      : t('kuma.noneSelected')}
                  </p>
                </div>
              ) : null}
              <div className='grid gap-4 md:grid-cols-2'>
                <FieldInput
                  label={t('kuma.interval')}
                  value={fields.uptime_kuma_interval}
                  type='number'
                  onChange={(value) =>
                    updateField('uptime_kuma_interval', value)
                  }
                />
                <FieldInput
                  label={t('kuma.retry')}
                  value={fields.uptime_kuma_retry}
                  type='number'
                  onChange={(value) => updateField('uptime_kuma_retry', value)}
                />
                <FieldInput
                  label={t('kuma.retryInterval')}
                  value={fields.uptime_kuma_retry_interval}
                  type='number'
                  onChange={(value) =>
                    updateField('uptime_kuma_retry_interval', value)
                  }
                />
                <FieldInput
                  label={t('kuma.timeout')}
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
