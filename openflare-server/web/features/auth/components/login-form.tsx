'use client';

import {zodResolver} from '@hookform/resolvers/zod';
import {useMutation, useQuery} from '@tanstack/react-query';
import Link from 'next/link';
import {useRouter, useSearchParams} from 'next/navigation';
import {useState} from 'react';
import {useForm} from 'react-hook-form';
import {z} from 'zod';

import {InlineMessage} from '@/components/feedback/inline-message';
import {useAuth} from '@/components/providers/auth-provider';
import {AppCard} from '@/components/ui/app-card';
import {getOAuthAuthorizeUrl, login} from '@/features/auth/api/auth';
import {getPublicStatus} from '@/features/auth/api/public';
import {AuthButton, AuthFormField, AuthInput, SecondaryButton,} from '@/features/auth/components/auth-form-primitives';
import {PublicAuthGuard} from '@/features/auth/components/public-auth-guard';

const TEXT = {
  usernameRequired: '\u8bf7\u8f93\u5165\u7528\u6237\u540d',
  passwordRequired: '\u8bf7\u8f93\u5165\u5bc6\u7801',
  loginFailed:
    '\u767b\u5f55\u5931\u8d25\uff0c\u8bf7\u7a0d\u540e\u91cd\u8bd5\u3002',
  oauthUnavailable: '第三方登录当前不可用。',
  title: '\u7528\u6237\u767b\u5f55',
  username: '\u7528\u6237\u540d',
  password: '\u5bc6\u7801',
  loginPending: '\u767b\u5f55\u4e2d...',
  login: '\u767b\u5f55',
  forgotPassword: '\u5fd8\u8bb0\u5bc6\u7801\uff1f',
  register: '\u6ce8\u518c',
};

const loginSchema = z.object({
  username: z.string().min(1, TEXT.usernameRequired),
  password: z.string().min(1, TEXT.passwordRequired),
});

type LoginFormValues = z.infer<typeof loginSchema>;

export function LoginForm() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const { setUser } = useAuth();
  const [errorMessage, setErrorMessage] = useState('');
  const redirect = searchParams?.get('redirect') || '/';

  const form = useForm<LoginFormValues>({
    resolver: zodResolver(loginSchema),
    defaultValues: {
      username: '',
      password: '',
    },
  });

  const statusQuery = useQuery({
    queryKey: ['public-status'],
    queryFn: getPublicStatus,
  });

  const loginMutation = useMutation({
    mutationFn: login,
    onSuccess: (user) => {
      setUser(user);
      router.replace(redirect);
    },
    onError: (error: Error) => {
      setErrorMessage(error.message || TEXT.loginFailed);
    },
  });

  const oauthMutation = useMutation({
    mutationFn: getOAuthAuthorizeUrl,
    onSuccess: (result) => {
      window.location.href = result.authorize_url;
    },
    onError: (error: Error) => {
      setErrorMessage(error.message || TEXT.oauthUnavailable);
    },
  });

  const handleSubmit = form.handleSubmit((values) => {
    setErrorMessage('');
    loginMutation.mutate(values);
  });

  const handleOAuthLogin = (sourceName: string) => {
    setErrorMessage('');
    oauthMutation.mutate(sourceName);
  };

  return (
    <PublicAuthGuard>
      <AppCard title={TEXT.title}>
        <form className="space-y-4" onSubmit={handleSubmit}>
          <AuthFormField label={TEXT.username}>
            <AuthInput
              placeholder={TEXT.username}
              {...form.register('username')}
            />
            {form.formState.errors.username ? (
              <span className="text-xs text-[var(--status-danger-foreground)]">
                {form.formState.errors.username.message}
              </span>
            ) : null}
          </AuthFormField>

          <AuthFormField label={TEXT.password}>
            <AuthInput
              type="password"
              placeholder={TEXT.password}
              {...form.register('password')}
            />
            {form.formState.errors.password ? (
              <span className="text-xs text-[var(--status-danger-foreground)]">
                {form.formState.errors.password.message}
              </span>
            ) : null}
          </AuthFormField>

          {errorMessage ? (
            <InlineMessage tone="danger" message={errorMessage} />
          ) : null}

          <div>
            <AuthButton type="submit" disabled={loginMutation.isPending}>
              {loginMutation.isPending ? TEXT.loginPending : TEXT.login}
            </AuthButton>
          </div>

          {(statusQuery.data?.auth_sources ?? []).length > 0 ? (
            <div className="flex flex-col items-center gap-3 pt-1">
              <div className="text-xs text-[var(--foreground-secondary)]">
                第三方账号登录
              </div>
              <div className="flex flex-wrap justify-center gap-3">
                {(statusQuery.data?.auth_sources ?? []).map((source) => (
                  <SecondaryButton
                    key={source.id}
                    type="button"
                    onClick={() => handleOAuthLogin(source.name)}
                    className="min-w-36"
                    disabled={oauthMutation.isPending}
                  >
                    {source.display_name || source.name} 登录
                  </SecondaryButton>
                ))}
              </div>
            </div>
          ) : null}
        </form>

        <div className="mt-6 flex flex-wrap gap-3 text-sm text-[var(--foreground-secondary)]">
          <Link
            href="/reset"
            className="text-[var(--brand-primary)] transition hover:opacity-80"
          >
            {TEXT.forgotPassword}
          </Link>
        </div>
      </AppCard>
    </PublicAuthGuard>
  );
}
