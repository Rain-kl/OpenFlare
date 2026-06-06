'use client';

import {zodResolver} from '@hookform/resolvers/zod';
import {useMutation} from '@tanstack/react-query';
import Link from 'next/link';
import {useRouter} from 'next/navigation';
import {useState} from 'react';
import {useForm} from 'react-hook-form';
import {z} from 'zod';

import {InlineMessage} from '@/components/feedback/inline-message';
import {useAuth} from '@/components/providers/auth-provider';
import {AppCard} from '@/components/ui/app-card';
import {linkExistingOAuthAccount} from '@/features/auth/api/auth';
import {AuthButton, AuthFormField, AuthInput,} from '@/features/auth/components/auth-form-primitives';

const schema = z.object({
  username: z.string().min(1, '请输入用户名'),
  password: z.string().min(1, '请输入密码'),
});

type FormValues = z.infer<typeof schema>;

export function OAuthLinkExistingForm() {
  const router = useRouter();
  const { setUser } = useAuth();
  const [errorMessage, setErrorMessage] = useState('');
  const form = useForm<FormValues>({
    resolver: zodResolver(schema),
    defaultValues: {
      username: '',
      password: '',
    },
  });

  const mutation = useMutation({
    mutationFn: linkExistingOAuthAccount,
    onSuccess: (result) => {
      if (result.user) {
        setUser(result.user);
        router.replace('/');
        return;
      }
      setErrorMessage('绑定成功但未返回用户信息，请重新登录。');
    },
    onError: (error: Error) => {
      setErrorMessage(error.message || '绑定失败，请稍后重试。');
    },
  });

  const handleSubmit = form.handleSubmit((values) => {
    setErrorMessage('');
    mutation.mutate(values);
  });

  return (
    <AppCard
      title="绑定已有账号"
      description="当前第三方账号尚未绑定本地用户，请使用已有账号完成关联。"
    >
      <form className="space-y-4" onSubmit={handleSubmit}>
        <AuthFormField label="用户名">
          <AuthInput
            placeholder="请输入用户名"
            {...form.register('username')}
          />
          {form.formState.errors.username ? (
            <span className="text-xs text-[var(--status-danger-foreground)]">
              {form.formState.errors.username.message}
            </span>
          ) : null}
        </AuthFormField>

        <AuthFormField label="密码">
          <AuthInput
            type="password"
            placeholder="请输入密码"
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

        <AuthButton type="submit" disabled={mutation.isPending}>
          {mutation.isPending ? '绑定中...' : '绑定并登录'}
        </AuthButton>
      </form>

      <div className="mt-6 text-sm text-[var(--foreground-secondary)]">
        <Link
          href="/login"
          className="text-[var(--brand-primary)] transition hover:opacity-80"
        >
          返回登录
        </Link>
      </div>
    </AppCard>
  );
}
