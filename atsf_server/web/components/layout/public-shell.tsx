import Link from 'next/link';
import type { ReactNode } from 'react';

import { ThemeToggle } from '@/components/ui/theme-toggle';
import { publicEnv } from '@/lib/env/public-env';

interface PublicShellProps {
  children: ReactNode;
}

export function PublicShell({ children }: PublicShellProps) {
  return (
    <div className='flex min-h-screen items-center justify-center px-4 py-12'>
      <div className='w-full max-w-3xl rounded-[var(--radius-xl)] border border-[var(--border-default)] bg-[var(--surface-panel)]/85 p-8 shadow-[var(--shadow-soft)] backdrop-blur'>
        <div className='mb-8 flex flex-col gap-4 border-b border-[var(--border-default)] pb-6 md:flex-row md:items-end md:justify-between'>
          <div>
            <p className='text-sm font-medium uppercase tracking-[0.24em] text-[var(--brand-primary)]'>ATSFlare</p>
            <h1 className='mt-2 text-3xl font-semibold text-[var(--foreground-primary)]'>欢迎使用 ATSFlare 管理端</h1>
            <p className='mt-2 text-sm leading-6 text-[var(--foreground-secondary)]'>
              当前已接入认证入口、核心业务模块以及阶段 4 的设置、用户、文件与关于页迁移。
            </p>
          </div>
          <div className='flex flex-col items-start gap-3 text-sm text-[var(--foreground-secondary)] md:items-end'>
            <ThemeToggle />
            <span className='rounded-full border border-[var(--border-default)] px-3 py-1.5'>
              {publicEnv.appVersion}
            </span>
          </div>
        </div>

        <div className='space-y-6'>{children}</div>

        <div className='mt-8 border-t border-[var(--border-default)] pt-6 text-sm text-[var(--foreground-secondary)]'>
          <div className='flex flex-wrap gap-4'>
            <Link href='/' className='text-[var(--brand-primary)] transition hover:opacity-80'>
              返回新版总览
            </Link>
            <Link href='/about' className='text-[var(--brand-primary)] transition hover:opacity-80'>
              查看关于页
            </Link>
          </div>
        </div>
      </div>
    </div>
  );
}
