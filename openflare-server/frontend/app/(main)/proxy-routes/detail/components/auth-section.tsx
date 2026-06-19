'use client';

import {useEffect, useState} from 'react';
import {zodResolver} from '@hookform/resolvers/zod';
import {useForm} from 'react-hook-form';
import {z} from 'zod';

import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form';
import {Input} from '@/components/ui/input';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import type {ProxyRouteItem, ProxyRoutePoWConfig} from '@/lib/services/openflare';

import {defaultPowConfig} from '@/app/(main)/waf/components/helpers';
import {PowConfigPanel} from '@/app/(main)/waf/components/pow-config-panel';

import {proxyRouteFormIds} from '../helpers';
import {useRouteSectionSave} from '../hooks/use-route-section-save';
import {SectionShell} from './section-shell';

const authSchema = z
  .object({
    auth_mode: z.enum(['none', 'basic', 'pow']),
    basic_auth_username: z.string(),
    basic_auth_password: z.string(),
    pow_enabled: z.boolean(),
    pow_difficulty: z.string(),
    pow_algorithm: z.enum(['fast', 'slow']),
    pow_session_ttl: z.string(),
    pow_challenge_ttl: z.string(),
  })
  .superRefine((value, context) => {
    if (value.auth_mode === 'pow') {
      const difficulty = Number(value.pow_difficulty);
      if (!Number.isFinite(difficulty) || difficulty < 1 || difficulty > 16) {
        context.addIssue({
          code: z.ZodIssueCode.custom,
          path: ['pow_difficulty'],
          message: '难度需为 1-16 的整数',
        });
      }

      const sessionTtl = Number(value.pow_session_ttl);
      if (!Number.isFinite(sessionTtl) || sessionTtl < 60) {
        context.addIssue({
          code: z.ZodIssueCode.custom,
          path: ['pow_session_ttl'],
          message: '会话 TTL 需大于等于 60 秒',
        });
      }

      const challengeTtl = Number(value.pow_challenge_ttl);
      if (!Number.isFinite(challengeTtl) || challengeTtl < 30) {
        context.addIssue({
          code: z.ZodIssueCode.custom,
          path: ['pow_challenge_ttl'],
          message: '挑战 TTL 需大于等于 30 秒',
        });
      }
    }

    if (value.auth_mode === 'basic') {
      if (!value.basic_auth_username.trim()) {
        context.addIssue({
          code: z.ZodIssueCode.custom,
          path: ['basic_auth_username'],
          message: '请输入账号',
        });
      }
      if (!value.basic_auth_password.trim()) {
        context.addIssue({
          code: z.ZodIssueCode.custom,
          path: ['basic_auth_password'],
          message: '请输入密码',
        });
      }
    }
  });

type AuthValues = z.infer<typeof authSchema>;

function resolveAuthMode(route: ProxyRouteItem): AuthValues['auth_mode'] {
  if (route.basic_auth_enabled) {
    return 'basic';
  }
  if (route.pow_enabled) {
    return 'pow';
  }
  return 'none';
}

function syncPowFormValues(
  setValue: ReturnType<typeof useForm<AuthValues>>['setValue'],
  config: ProxyRoutePoWConfig,
) {
  setValue('pow_difficulty', String(config.difficulty));
  setValue('pow_algorithm', config.algorithm);
  setValue('pow_session_ttl', String(config.session_ttl));
  setValue('pow_challenge_ttl', String(config.challenge_ttl));
}

interface AuthSectionProps {
  route: ProxyRouteItem;
  onRouteUpdate: (route: ProxyRouteItem) => void;
  onSavingChange?: (saving: boolean) => void;
}

export function AuthSection({ route, onRouteUpdate, onSavingChange }: AuthSectionProps) {
  const { saving, save } = useRouteSectionSave(route, onRouteUpdate, onSavingChange);
  const [powConfig, setPowConfig] = useState<ProxyRoutePoWConfig>(
    route.pow_config ?? defaultPowConfig,
  );

  const form = useForm<AuthValues>({
    resolver: zodResolver(authSchema),
    defaultValues: {
      auth_mode: resolveAuthMode(route),
      basic_auth_username: route.basic_auth_username || '',
      basic_auth_password: route.basic_auth_password || '',
      pow_enabled: route.pow_enabled,
      pow_difficulty: String(route.pow_config?.difficulty ?? defaultPowConfig.difficulty),
      pow_algorithm: route.pow_config?.algorithm ?? defaultPowConfig.algorithm,
      pow_session_ttl: String(route.pow_config?.session_ttl ?? defaultPowConfig.session_ttl),
      pow_challenge_ttl: String(route.pow_config?.challenge_ttl ?? defaultPowConfig.challenge_ttl),
    },
  });

  useEffect(() => {
    const nextPowConfig = route.pow_config ?? defaultPowConfig;
    setPowConfig(nextPowConfig);
    form.reset({
      auth_mode: resolveAuthMode(route),
      basic_auth_username: route.basic_auth_username || '',
      basic_auth_password: route.basic_auth_password || '',
      pow_enabled: route.pow_enabled,
      pow_difficulty: String(nextPowConfig.difficulty),
      pow_algorithm: nextPowConfig.algorithm,
      pow_session_ttl: String(nextPowConfig.session_ttl),
      pow_challenge_ttl: String(nextPowConfig.challenge_ttl),
    });
  }, [form, route]);

  const authMode = form.watch('auth_mode');

  return (
    <SectionShell
      title="认证配置"
      description="配置基础鉴权或 PoW 防护，限制未授权访问。"
      formId={proxyRouteFormIds.auth}
      saving={saving}
    >
      <Form {...form}>
        <form
          id={proxyRouteFormIds.auth}
          className="space-y-5"
          onSubmit={form.handleSubmit(async (values) => {
            const nextPowConfig =
              values.auth_mode === 'pow' ? powConfig : route.pow_config ?? defaultPowConfig;
            const powEnabled = values.auth_mode === 'pow' || values.pow_enabled;

            await save(
              {
                basic_auth_enabled: values.auth_mode === 'basic',
                basic_auth_username:
                  values.auth_mode === 'basic' ? values.basic_auth_username.trim() : '',
                basic_auth_password:
                  values.auth_mode === 'basic' ? values.basic_auth_password.trim() : '',
                pow_enabled: powEnabled,
                pow_config: JSON.stringify(
                  values.auth_mode === 'pow' ? nextPowConfig : route.pow_config ?? defaultPowConfig,
                ),
              },
              '认证配置已保存',
            );
          })}
        >
          <FormField
            control={form.control}
            name="auth_mode"
            render={({ field }) => (
              <FormItem>
                <FormLabel>认证模式</FormLabel>
                <Select value={field.value} onValueChange={field.onChange}>
                  <FormControl>
                    <SelectTrigger>
                      <SelectValue />
                    </SelectTrigger>
                  </FormControl>
                  <SelectContent>
                    <SelectItem value="none">无认证</SelectItem>
                    <SelectItem value="basic">Basic Auth</SelectItem>
                    <SelectItem value="pow">PoW 防护</SelectItem>
                  </SelectContent>
                </Select>
                <FormDescription>同一站点仅启用一种认证模式。</FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          {authMode === 'basic' ? (
            <>
              <FormField
                control={form.control}
                name="basic_auth_username"
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>账号</FormLabel>
                    <FormControl>
                      <Input placeholder="admin" {...field} />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name="basic_auth_password"
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>密码</FormLabel>
                    <FormControl>
                      <Input type="text" placeholder="secret123" {...field} />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </>
          ) : null}

          {authMode === 'pow' ? (
            <PowConfigPanel
              enabled={form.watch('pow_enabled')}
              config={powConfig}
              onChange={(enabled, config) => {
                form.setValue('pow_enabled', enabled);
                setPowConfig(config);
                syncPowFormValues(form.setValue, config);
              }}
            />
          ) : null}
        </form>
      </Form>
    </SectionShell>
  );
}