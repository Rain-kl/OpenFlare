import Link from 'next/link';

import { AppCard } from '@/components/ui/app-card';
import { StatusBadge } from '@/components/ui/status-badge';
import { dashboardNavigation } from '@/lib/constants/navigation';

const readinessItems = [
  {
    title: '工程底座',
    description: 'Next.js App Router、TypeScript strict、Tailwind CSS 与静态导出链路已稳定运行。',
  },
  {
    title: '认证骨架',
    description: '登录、注册、重置密码、鉴权守卫与后台布局已迁移到新前端。',
  },
  {
    title: '业务模块',
    description: '核心链路与阶段 4 的设置、用户、文件模块都已接入真实数据与交互。',
  },
];

export function DashboardOverview() {
  return (
    <div className='space-y-6'>
      <AppCard
        title='阶段 4 进行中'
        description='当前已完成新版管理端基础工程、核心模块和大部分边缘模块迁移，可继续推进联调与回归。'
        action={<StatusBadge label='准备联调回归' variant='success' />}
      >
        <div className='grid gap-4 lg:grid-cols-3'>
          {readinessItems.map((item) => (
            <div
              key={item.title}
              className='rounded-2xl border border-[var(--border-default)] bg-[var(--surface-muted)] p-4'
            >
              <p className='text-base font-semibold text-[var(--foreground-primary)]'>{item.title}</p>
              <p className='mt-2 text-sm leading-6 text-[var(--foreground-secondary)]'>
                {item.description}
              </p>
            </div>
          ))}
        </div>
      </AppCard>

      <div className='grid gap-6 xl:grid-cols-[1.3fr_0.9fr]'>
        <AppCard title='模块入口' description='新版导航中的业务页面均已接入真实数据，设置与边缘模块也已可直接使用。'>
          <div className='grid gap-3 md:grid-cols-2'>
            {dashboardNavigation.slice(1).map((item) => (
              <Link
                key={item.href}
                href={item.href}
                className='rounded-2xl border border-[var(--border-default)] bg-[var(--surface-muted)] p-4 transition hover:border-[var(--border-strong)] hover:bg-[var(--accent-soft)]'
              >
                <p className='text-sm font-semibold text-[var(--foreground-primary)]'>{item.label}</p>
                <p className='mt-2 text-sm leading-6 text-[var(--foreground-secondary)]'>
                  {item.description}
                </p>
              </Link>
            ))}
          </div>
        </AppCard>

        <AppCard title='下一步建议' description='按前端改造计划，当前可以开始阶段 5 的联调、回归与默认入口切换准备。'>
          <ol className='space-y-3 text-sm leading-6 text-[var(--foreground-secondary)]'>
            <li>1. 对照旧前端完成设置、用户、文件与关于页的联调回归。</li>
            <li>2. 补齐关键页面的测试覆盖与构建验收。</li>
            <li>3. 评估默认入口切换与旧前端回滚预案。</li>
          </ol>
        </AppCard>
      </div>
    </div>
  );
}
