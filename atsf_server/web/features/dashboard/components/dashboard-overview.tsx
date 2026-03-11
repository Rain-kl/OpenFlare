import Link from 'next/link';

import { AppCard } from '@/components/ui/app-card';
import { StatusBadge } from '@/components/ui/status-badge';
import { dashboardNavigation } from '@/lib/constants/navigation';

const readinessItems = [
  {
    title: '工程底座',
    description: 'Next.js App Router、TypeScript strict、Tailwind CSS 与静态导出链路已建立。',
  },
  {
    title: '质量工具',
    description: 'ESLint、Prettier、Vitest、Playwright 配置已就位，可继续补充模块测试。',
  },
  {
    title: '目录分层',
    description: '已拆分 app、components、features、lib、store、tests 等基础结构。',
  },
];

export function DashboardOverview() {
  return (
    <div className='space-y-6'>
      <AppCard
        title='阶段 1 已启动'
        description='当前已完成新版管理端基础工程初始化，可继续推进认证迁移与框架层骨架。'
        action={<StatusBadge label='可继续阶段 2' variant='success' />}
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
        <AppCard title='模块入口' description='各业务页面已建立路由占位，后续按阶段逐步接入真实数据与交互。'>
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

        <AppCard title='下一步建议' description='按前端改造计划，后续优先进入认证与框架层迁移。'>
          <ol className='space-y-3 text-sm leading-6 text-[var(--foreground-secondary)]'>
            <li>1. 继续完善认证页细节与第三方登录体验。</li>
            <li>2. 在业务模块中逐步接入 Query 与 API 资源层。</li>
            <li>3. 为主链路页面补齐真实列表、表单与动作反馈。</li>
          </ol>
        </AppCard>
      </div>
    </div>
  );
}
