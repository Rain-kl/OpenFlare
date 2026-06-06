'use client';

import {useMutation} from '@tanstack/react-query';
import Link from 'next/link';
import {usePathname, useRouter, useSearchParams} from 'next/navigation';
import {useEffect, useRef, useState} from 'react';

import {InlineMessage} from '@/components/feedback/inline-message';
import {LoadingState} from '@/components/feedback/loading-state';
import {useAuth} from '@/components/providers/auth-provider';
import {AppCard} from '@/components/ui/app-card';
import {exchangeOAuthCode} from '@/features/auth/api/auth';

function parseOAuthSource(pathname: string | null, sourceParam: string) {
  const normalizedParam = sourceParam.trim();
  if (normalizedParam) {
    return normalizedParam;
  }

  const pathMatch = pathname?.match(/^\/oauth\/([^/?#]+)$/);
  if (!pathMatch) {
    return '';
  }

  return decodeURIComponent(pathMatch[1]);
}

export function OAuthCallback({ sourceId }: { sourceId?: number }) {
  const router = useRouter();
  const pathname = usePathname();
  const searchParams = useSearchParams();
  const { setUser } = useAuth();
  const handledRef = useRef('');
  const [prompt, setPrompt] = useState('正在处理授权结果...');
  const [message, setMessage] = useState<{
    tone: 'danger' | 'success';
    text: string;
  } | null>(null);
  const code = searchParams?.get('code')?.trim() ?? '';
  const state = searchParams?.get('state')?.trim() ?? '';
  const oauthError = searchParams?.get('error')?.trim() ?? '';
  const oauthErrorDescription =
    searchParams?.get('error_description')?.trim() ?? '';
  const resolvedSource =
    sourceId ?? parseOAuthSource(pathname, searchParams?.get('source') ?? '');

  const mutation = useMutation({
    mutationFn: () => exchangeOAuthCode(resolvedSource, code, state),
    onSuccess: (result) => {
      if (result.status === 'link_required') {
        setMessage({ tone: 'success', text: '请绑定已有账号以完成登录。' });
        router.replace('/oauth/link');
        return;
      }
      if (result.user) {
        setUser(result.user);
        setMessage({ tone: 'success', text: '登录成功，正在跳转...' });
        router.replace('/');
        return;
      }
      setPrompt('授权处理失败');
      setMessage({ tone: 'danger', text: '授权结果缺少用户信息。' });
    },
    onError: (error: Error) => {
      setPrompt('授权处理失败');
      setMessage({
        tone: 'danger',
        text: error.message || '授权失败，请稍后重试。',
      });
    },
  });

  useEffect(() => {
    if (oauthError || oauthErrorDescription) {
      setPrompt('授权处理失败');
      setMessage({
        tone: 'danger',
        text: oauthErrorDescription || oauthError,
      });
      return;
    }

    if (String(resolvedSource).trim() === '') {
      setPrompt('缺少认证源参数');
      setMessage({
        tone: 'danger',
        text: '未收到认证源参数，请返回登录页重试。',
      });
      return;
    }

    if (!code || !state) {
      setPrompt('缺少授权参数');
      setMessage({
        tone: 'danger',
        text: '未收到完整授权参数，请返回登录页重试。',
      });
      return;
    }

    const key = `${resolvedSource}:${code}:${state}`;
    if (handledRef.current === key) {
      return;
    }
    handledRef.current = key;
    mutation.mutate();
  }, [
    code,
    mutation,
    oauthError,
    oauthErrorDescription,
    resolvedSource,
    state,
  ]);

  return (
    <AppCard title="第三方登录回调" description={prompt}>
      <div className="space-y-4">
        {mutation.isPending ? <LoadingState /> : null}
        {message ? (
          <InlineMessage tone={message.tone} message={message.text} />
        ) : null}
        {message?.tone === 'danger' ? (
          <div className="flex justify-center">
            <Link
              href="/login"
              className="inline-flex items-center justify-center rounded-2xl border border-[var(--border-default)] bg-[var(--control-background)] px-4 py-3 text-sm font-medium text-[var(--foreground-primary)] transition hover:bg-[var(--control-background-hover)]"
            >
              返回登录
            </Link>
          </div>
        ) : null}
      </div>
    </AppCard>
  );
}
