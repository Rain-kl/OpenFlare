'use client';

import { useEffect, useState } from 'react';
import {
  useMutation,
  useQuery,
  useQueryClient,
  type UseQueryResult,
} from '@tanstack/react-query';
import {
  Clock,
  Fingerprint,
  Globe,
  Loader2,
  Lock,
  Mail,
  Pencil,
  Plus,
  Settings,
  Shield,
  Trash2,
  UserPlus,
} from 'lucide-react';

import { Button } from '@/components/ui/button';
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card';
import { Switch } from '@/components/ui/switch';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog';
import { AuthSourceModal } from '@/components/common/settings/auth-source-modal';
import services from '@/lib/services';
import type { AuthSource, SystemConfig } from '@/lib/services/admin';
import { toast } from 'sonner';
import { useTranslations } from 'next-intl';

const SECURITY_KEYS = [
  {
    key: 'password_login_enabled',
    titleKey: 'passwordLoginEnabled',
    descKey: 'passwordLoginEnabledDesc',
    icon: Lock,
  },
  {
    key: 'registration_enabled',
    titleKey: 'registrationEnabled',
    descKey: 'registrationEnabledDesc',
    icon: UserPlus,
  },
  {
    key: 'password_register_enabled',
    titleKey: 'passwordRegisterEnabled',
    descKey: 'passwordRegisterEnabledDesc',
    icon: Fingerprint,
  },
  {
    key: 'oidc_login_enabled',
    titleKey: 'oidcLoginEnabled',
    descKey: 'oidcLoginEnabledDesc',
    icon: Globe,
  },
  {
    key: 'email_login_verification_enabled',
    titleKey: 'emailLoginVerification',
    descKey: 'emailLoginVerificationDesc',
    icon: Mail,
  },
  {
    key: 'email_register_verification_enabled',
    titleKey: 'emailRegisterVerification',
    descKey: 'emailRegisterVerificationDesc',
    icon: Mail,
  },
] as const;

interface SecurityTabProps {
  configs: Record<string, SystemConfig>;
  systemConfigsQuery: UseQueryResult<SystemConfig[], Error>;
}

export function SecurityTab({ configs, systemConfigsQuery }: SecurityTabProps) {
  const queryClient = useQueryClient();
  const t = useTranslations('settings.security');
  const tCommon = useTranslations('common');
  const [authSourceModalOpen, setAuthSourceModalOpen] = useState(false);
  const [selectedSource, setSelectedSource] = useState<AuthSource | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<AuthSource | null>(null);

  const [capCount, setCapCount] = useState('');
  const [capDifficulty, setCapDifficulty] = useState('');
  const [capSize, setCapSize] = useState('');
  const [capTTL, setCapTTL] = useState('');
  const [capTokenTTL, setCapTokenTTL] = useState('');
  const [capAutoSolve, setCapAutoSolve] = useState(true);

  const [sessionTTL, setSessionTTL] = useState('168');
  const [customHours, setCustomHours] = useState('');

  const authSourcesQuery = useQuery({
    queryKey: ['auth', 'sources'],
    queryFn: () => services.adminAuthSource.listAuthSources(),
  });

  useEffect(() => {
    if (systemConfigsQuery.data) {
      const cfgMap = configs;
      setCapCount(cfgMap['cap_challenge_count']?.value || '1');
      setCapDifficulty(cfgMap['cap_challenge_difficulty']?.value || '4');
      setCapSize(cfgMap['cap_challenge_size']?.value || '32');
      setCapTTL(cfgMap['cap_challenge_ttl_seconds']?.value || '600');
      setCapTokenTTL(cfgMap['cap_token_ttl_seconds']?.value || '1200');
      setCapAutoSolve(cfgMap['cap_auto_solve']?.value !== 'false');

      // 初始化登录保持设置
      const ttlVal = cfgMap['login_session_ttl_hours']?.value || '0';
      if (
        ttlVal === '0' ||
        ttlVal === '168' ||
        ttlVal === '720' ||
        ttlVal === '-1'
      ) {
        setSessionTTL(ttlVal);
        setCustomHours('');
      } else {
        setSessionTTL('custom');
        setCustomHours(ttlVal);
      }
    }
  }, [systemConfigsQuery.data, configs]);

  const updateTTLMutation = useMutation({
    mutationFn: async (value: string) => {
      const config = configs['login_session_ttl_hours'];
      if (!config) {
        throw new Error('缺少配置项: login_session_ttl_hours');
      }
      await services.adminSystemConfig.updateSystemConfig(
        'login_session_ttl_hours',
        {
          value: value,
          description: config.description,
        },
      );
    },
    onSuccess: async () => {
      await queryClient.invalidateQueries({
        queryKey: ['admin', 'system-configs'],
      });
      toast.success(t('loginSessionTTLUpdated'));
    },
    onError: (error: Error) => {
      toast.error(error.message || t('updateConfigFailed'));
    },
  });

  const handleTTLChange = (val: string) => {
    setSessionTTL(val);
    if (val !== 'custom') {
      updateTTLMutation.mutate(val);
    }
  };

  const handleCustomBlur = () => {
    const parsed = parseInt(customHours, 10);
    if (isNaN(parsed) || parsed <= 0) {
      toast.error(t('invalidHours'));
      // 重置为原本的值
      const originalVal = configs['login_session_ttl_hours']?.value || '0';
      setCustomHours(
        originalVal === 'custom' ||
          ['0', '168', '720', '-1'].includes(originalVal)
          ? ''
          : originalVal,
      );
      return;
    }
    updateTTLMutation.mutate(parsed.toString());
  };

  const updateConfigMutation = useMutation({
    mutationFn: async ({ key, value }: { key: string; value: boolean }) => {
      const config = configs[key];
      if (!config) {
        throw new Error(`缺少配置项: ${key}`);
      }
      await services.adminSystemConfig.updateSystemConfig(key, {
        value: value ? 'true' : 'false',
        description: config.description,
      });
    },
    onSuccess: async () => {
      await queryClient.invalidateQueries({
        queryKey: ['admin', 'system-configs'],
      });
      await queryClient.invalidateQueries({ queryKey: ['public-config'] });
      toast.success(t('securityConfigUpdated'));
    },
    onError: (error: Error) => {
      toast.error(error.message || t('updateConfigFailed'));
    },
  });

  const toggleSourceMutation = useMutation({
    mutationFn: async (source: AuthSource) => {
      await services.adminAuthSource.toggleAuthSource(source.id, {
        is_active: !source.is_active,
      });
    },
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ['auth', 'sources'] });
      await queryClient.invalidateQueries({
        queryKey: ['auth', 'public-sources'],
      });
      toast.success(t('authSourceStatusUpdated'));
    },
    onError: (error: Error) => {
      toast.error(error.message || t('toggleStatusFailed'));
    },
  });

  const deleteSourceMutation = useMutation({
    mutationFn: async (sourceId: string) => {
      await services.adminAuthSource.deleteAuthSource(sourceId);
    },
    onSuccess: async () => {
      setDeleteTarget(null);
      await queryClient.invalidateQueries({ queryKey: ['auth', 'sources'] });
      await queryClient.invalidateQueries({
        queryKey: ['auth', 'public-sources'],
      });
      toast.success(t('authSourceDeleted'));
    },
    onError: (error: Error) => {
      toast.error(error.message || t('deleteAuthSourceFailed'));
    },
  });

  const handleToggle = (key: string, checked: boolean) => {
    updateConfigMutation.mutate({ key, value: checked });
  };

  const saveCapMutation = useMutation({
    mutationFn: async () => {
      const updates = [
        { key: 'cap_challenge_count', value: capCount },
        { key: 'cap_challenge_difficulty', value: capDifficulty },
        { key: 'cap_challenge_size', value: capSize },
        { key: 'cap_challenge_ttl_seconds', value: capTTL },
        { key: 'cap_token_ttl_seconds', value: capTokenTTL },
        { key: 'cap_auto_solve', value: capAutoSolve ? 'true' : 'false' },
      ];

      for (const update of updates) {
        const currentCfg = configs[update.key];
        await services.adminSystemConfig.updateSystemConfig(update.key, {
          value: update.value,
          description: currentCfg?.description || '',
        });
      }
    },
    onSuccess: async () => {
      await queryClient.invalidateQueries({
        queryKey: ['admin', 'system-configs'],
      });
      toast.success(t('captchaConfigSaved'));
    },
    onError: (error: Error) => {
      toast.error(error.message || t('saveConfigFailed'));
    },
  });

  const handleCapSave = (e: React.FormEvent) => {
    e.preventDefault();
    saveCapMutation.mutate();
  };

  return (
    <div className='space-y-6'>
      {/* 系统登录与注册控制 */}
      <Card className='border border-dashed shadow-sm'>
        <CardHeader className='border-b border-dashed pb-4'>
          <div className='flex items-center gap-2'>
            <div className='p-1.5 rounded-lg bg-primary/10 text-primary'>
              <Settings className='size-4' />
            </div>
            <div>
              <CardTitle className='text-base font-semibold'>
                {t('loginRegistrationSettings')}
              </CardTitle>
              <CardDescription className='text-xs'>
                {t('loginRegistrationSettingsDesc')}
              </CardDescription>
            </div>
          </div>
        </CardHeader>
        <CardContent className='pt-6'>
          <div className='grid grid-cols-1 md:grid-cols-2 gap-4'>
            {SECURITY_KEYS.map((item) => {
              const config = configs[item.key];
              const checked = config ? config.value === 'true' : false;
              const Icon = item.icon;
              return (
                <div
                  key={item.key}
                  className='flex items-center justify-between gap-4 rounded-xl border border-dashed p-4 bg-card hover:bg-muted/10 hover:border-primary/30 transition-all duration-300 shadow-sm'
                >
                  <div className='space-y-1'>
                    <div className='flex items-center gap-2'>
                      {Icon && <Icon className='size-4 text-primary' />}
                      <Label
                        htmlFor={item.key}
                        className='font-medium text-sm text-foreground'
                      >
                        {t(item.titleKey)}
                      </Label>
                    </div>
                    <p className='text-xs text-muted-foreground leading-relaxed pr-2'>
                      {t(item.descKey)}
                    </p>
                  </div>
                  <Switch
                    id={item.key}
                    checked={checked}
                    disabled={updateConfigMutation.isPending}
                    onCheckedChange={(value) => handleToggle(item.key, value)}
                  />
                </div>
              );
            })}

            {/* 登录状态保持时间 (选择后立即更改) */}
            <div className='flex items-center justify-between gap-4 rounded-xl border border-dashed p-4 bg-card hover:bg-muted/10 hover:border-primary/30 transition-all duration-300 shadow-sm md:col-span-2'>
              <div className='space-y-1 pr-4'>
                <div className='flex items-center gap-2'>
                  <Clock className='size-4 text-primary' />
                  <span className='font-medium text-sm text-foreground'>
                    {t('loginSessionTTL')}
                  </span>
                </div>
                <p className='text-xs text-muted-foreground leading-relaxed pr-2'>
                  {t('loginSessionTTLDesc')}
                </p>
              </div>

              <div className='flex items-center gap-2 flex-shrink-0'>
                <Select
                  value={sessionTTL}
                  disabled={updateTTLMutation.isPending}
                  onValueChange={handleTTLChange}
                >
                  <SelectTrigger
                    className='w-[180px] bg-card border-dashed text-xs h-8'
                    aria-label={t('loginSessionTTL')}
                  >
                    <SelectValue placeholder={t('selectRetentionTime')} />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value='0'>{t('sessionOff')}</SelectItem>
                    <SelectItem value='168'>{t('session7Days')}</SelectItem>
                    <SelectItem value='720'>{t('session30Days')}</SelectItem>
                    <SelectItem value='-1'>
                      {t('sessionNeverExpire')}
                    </SelectItem>
                    <SelectItem value='custom'>{t('sessionCustom')}</SelectItem>
                  </SelectContent>
                </Select>

                {sessionTTL === 'custom' && (
                  <Input
                    type='number'
                    min={1}
                    value={customHours}
                    onChange={(e) => setCustomHours(e.target.value)}
                    onBlur={handleCustomBlur}
                    onKeyDown={(e) => {
                      if (e.key === 'Enter') {
                        handleCustomBlur();
                      }
                    }}
                    placeholder={t('hoursPlaceholder')}
                    disabled={updateTTLMutation.isPending}
                    className='w-20 bg-card border-dashed text-xs h-8 px-2'
                  />
                )}
              </div>
            </div>
          </div>
        </CardContent>
      </Card>

      {/* 认证源配置管理 */}
      <Card className='border border-dashed shadow-sm'>
        <CardHeader className='border-b border-dashed pb-4 flex flex-row items-center justify-between gap-4'>
          <div className='flex items-center gap-2'>
            <div className='p-1.5 rounded-lg bg-primary/10 text-primary'>
              <Globe className='size-4' />
            </div>
            <div>
              <CardTitle className='text-base font-semibold'>
                {t('authSourceManagement')}
              </CardTitle>
              <CardDescription className='text-xs'>
                {t('authSourceManagementDesc')}
              </CardDescription>
            </div>
          </div>
          <Button
            type='button'
            size='sm'
            onClick={() => {
              setSelectedSource(null);
              setAuthSourceModalOpen(true);
            }}
            variant='secondary'
          >
            <Plus className='mr-1.5 size-3.5' />
            {t('addAuthSource')}
          </Button>
        </CardHeader>
        <CardContent className='pt-6 space-y-3'>
          {authSourcesQuery.isPending ? (
            <div className='flex items-center justify-center p-8'>
              <Loader2 className='size-6 animate-spin text-muted-foreground/50' />
            </div>
          ) : (authSourcesQuery.data ?? []).length > 0 ? (
            (authSourcesQuery.data ?? []).map((source) => (
              <div
                key={source.id}
                className='flex items-center justify-between rounded-xl border border-dashed p-4 bg-card hover:bg-muted/10 transition-all duration-300 shadow-sm'
              >
                <div className='space-y-1.5'>
                  <div className='flex items-center gap-2'>
                    <span className='font-semibold text-sm text-foreground'>
                      {source.display_name || source.name}
                    </span>
                    <span
                      className={`text-[10px] px-2 py-0.5 rounded-full border font-medium ${
                        source.is_active
                          ? 'bg-emerald-500/10 text-emerald-500 border-emerald-500/20'
                          : 'bg-amber-500/10 text-amber-500 border-amber-500/20'
                      }`}
                    >
                      {source.is_active ? t('enabled') : t('disabled')}
                    </span>
                  </div>
                  <div className='text-xs text-muted-foreground font-mono'>
                    {t('identifier')}: {source.name} · {t('type')}:{' '}
                    {source.type.toUpperCase()}
                  </div>
                </div>
                <div className='flex items-center gap-4'>
                  <span
                    className={`text-xs px-2.5 py-1 rounded-lg border font-medium hidden sm:inline-block ${
                      source.client_secret_configured
                        ? 'bg-primary/5 text-primary border-primary/10'
                        : 'bg-rose-500/5 text-rose-500 border-rose-500/10'
                    }`}
                  >
                    {source.client_secret_configured
                      ? t('secretConfigured')
                      : t('secretNotConfigured')}
                  </span>

                  <div className='flex items-center gap-2'>
                    <Switch
                      checked={source.is_active}
                      disabled={toggleSourceMutation.isPending}
                      className='scale-90 mr-2'
                      onCheckedChange={() =>
                        toggleSourceMutation.mutate(source)
                      }
                    />
                    <Button
                      type='button'
                      variant='ghost'
                      size='icon'
                      className='size-8 text-muted-foreground hover:text-primary hover:bg-primary/10 rounded-lg transition-colors'
                      onClick={() => {
                        setSelectedSource(source);
                        setAuthSourceModalOpen(true);
                      }}
                    >
                      <Pencil className='size-4' />
                    </Button>
                    <Button
                      type='button'
                      variant='ghost'
                      size='icon'
                      className='size-8 text-muted-foreground hover:text-rose-500 hover:bg-rose-500/10 rounded-lg transition-colors'
                      disabled={deleteSourceMutation.isPending}
                      onClick={() => setDeleteTarget(source)}
                    >
                      <Trash2 className='size-4' />
                    </Button>
                  </div>
                </div>
              </div>
            ))
          ) : (
            <div className='rounded-xl border border-dashed border-border/50 px-4 py-8 text-center text-xs text-muted-foreground bg-muted/5 flex flex-col items-center justify-center gap-3'>
              <span>{t('noAuthSources')}</span>
              <Button
                type='button'
                size='sm'
                variant='outline'
                onClick={() => {
                  setSelectedSource(null);
                  setAuthSourceModalOpen(true);
                }}
                className='border-dashed'
              >
                <Plus className='mr-1.5 size-3.5' />
                {t('addAuthSource')}
              </Button>
            </div>
          )}
        </CardContent>
      </Card>

      {/* 人机验证配置 (Cap CAPTCHA) */}
      <Card className='border border-dashed shadow-sm'>
        <CardHeader className='border-b border-dashed pb-4 flex flex-row items-center justify-between gap-4'>
          <div className='flex items-center gap-2'>
            <div className='p-1.5 rounded-lg bg-primary/10 text-primary'>
              <Shield className='size-4' />
            </div>
            <div>
              <CardTitle className='text-base font-semibold'>
                {t('captchaConfig')}
              </CardTitle>
              <CardDescription className='text-xs'>
                {t('captchaConfigDesc')}
              </CardDescription>
            </div>
          </div>
          <Switch
            checked={configs['cap_login_enabled']?.value === 'true'}
            disabled={updateConfigMutation.isPending}
            onCheckedChange={(checked) =>
              handleToggle('cap_login_enabled', checked)
            }
            aria-label={t('loginCaptchaEnabled')}
          />
        </CardHeader>
        <CardContent className='pt-6'>
          {/* 自动开始计算 Switch */}
          <div className='flex items-center justify-between rounded-xl border border-dashed p-4 bg-card mb-4'>
            <div className='space-y-0.5'>
              <Label htmlFor='cap_auto_solve' className='text-sm font-semibold'>
                {t('autoStartSolving')}
              </Label>
            </div>
            <Switch
              id='cap_auto_solve'
              checked={capAutoSolve}
              onCheckedChange={setCapAutoSolve}
            />
          </div>
          <form onSubmit={handleCapSave} className='space-y-6'>
            <div className='grid grid-cols-1 sm:grid-cols-2 gap-4'>
              <div className='space-y-1.5'>
                <Label
                  htmlFor='cap_challenge_count'
                  className='text-xs font-semibold'
                >
                  {t('challengeCount')}
                </Label>
                <Input
                  id='cap_challenge_count'
                  type='number'
                  min={1}
                  max={100}
                  value={capCount}
                  onChange={(e) => setCapCount(e.target.value)}
                  placeholder='50'
                  className='bg-card border-dashed text-xs'
                />
                <p className='text-[10px] text-muted-foreground leading-normal'>
                  {t('challengeCountDesc')}
                </p>
              </div>

              <div className='space-y-1.5'>
                <Label
                  htmlFor='cap_challenge_difficulty'
                  className='text-xs font-semibold'
                >
                  {t('challengeDifficulty')}
                </Label>
                <Input
                  id='cap_challenge_difficulty'
                  type='number'
                  min={1}
                  max={10}
                  value={capDifficulty}
                  onChange={(e) => setCapDifficulty(e.target.value)}
                  placeholder='4'
                  className='bg-card border-dashed text-xs'
                />
                <p className='text-[10px] text-muted-foreground leading-normal'>
                  {t('challengeDifficultyDesc')}
                </p>
              </div>

              <div className='space-y-1.5'>
                <Label
                  htmlFor='cap_challenge_size'
                  className='text-xs font-semibold'
                >
                  {t('challengeSize')}
                </Label>
                <Input
                  id='cap_challenge_size'
                  type='number'
                  min={8}
                  max={64}
                  value={capSize}
                  onChange={(e) => setCapSize(e.target.value)}
                  placeholder='32'
                  className='bg-card border-dashed text-xs'
                />
                <p className='text-[10px] text-muted-foreground leading-normal'>
                  {t('challengeSizeDesc')}
                </p>
              </div>

              <div className='space-y-1.5'>
                <Label
                  htmlFor='cap_challenge_ttl'
                  className='text-xs font-semibold'
                >
                  {t('challengeTTL')}
                </Label>
                <Input
                  id='cap_challenge_ttl'
                  type='number'
                  min={10}
                  value={capTTL}
                  onChange={(e) => setCapTTL(e.target.value)}
                  placeholder='600'
                  className='bg-card border-dashed text-xs'
                />
                <p className='text-[10px] text-muted-foreground leading-normal'>
                  {t('challengeTTLDesc')}
                </p>
              </div>

              <div className='space-y-1.5 sm:col-span-2'>
                <Label
                  htmlFor='cap_token_ttl'
                  className='text-xs font-semibold'
                >
                  {t('tokenTTL')}
                </Label>
                <Input
                  id='cap_token_ttl'
                  type='number'
                  min={10}
                  value={capTokenTTL}
                  onChange={(e) => setCapTokenTTL(e.target.value)}
                  placeholder='1200'
                  className='bg-card border-dashed text-xs'
                />
                <p className='text-[10px] text-muted-foreground leading-normal'>
                  {t('tokenTTLDesc')}
                </p>
              </div>
            </div>

            <div className='flex justify-end pt-4 border-t border-dashed'>
              <Button
                type='submit'
                size='sm'
                disabled={saveCapMutation.isPending}
              >
                {saveCapMutation.isPending ? (
                  <>
                    <Loader2 className='mr-1.5 size-3.5 animate-spin' />
                    {t('saving')}
                  </>
                ) : (
                  t('saveConfig')
                )}
              </Button>
            </div>
          </form>
        </CardContent>
      </Card>

      <AuthSourceModal
        isOpen={authSourceModalOpen}
        source={selectedSource}
        onClose={() => setAuthSourceModalOpen(false)}
        onChanged={async () => {
          await queryClient.invalidateQueries({
            queryKey: ['auth', 'sources'],
          });
          await queryClient.invalidateQueries({
            queryKey: ['auth', 'public-sources'],
          });
          await authSourcesQuery.refetch();
        }}
      />

      <AlertDialog
        open={Boolean(deleteTarget)}
        onOpenChange={(open) => !open && setDeleteTarget(null)}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('deleteAuthSourceTitle')}</AlertDialogTitle>
            <AlertDialogDescription>
              {t('deleteAuthSourceConfirm', {
                name: deleteTarget?.display_name || deleteTarget?.name || '',
              })}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={deleteSourceMutation.isPending}>
              {tCommon('cancel')}
            </AlertDialogCancel>
            <AlertDialogAction
              disabled={deleteSourceMutation.isPending}
              onClick={() =>
                deleteTarget && deleteSourceMutation.mutate(deleteTarget.id)
              }
            >
              {deleteSourceMutation.isPending
                ? t('deleting')
                : t('confirmDelete')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}
