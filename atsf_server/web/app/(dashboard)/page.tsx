import { DashboardOverview } from '@/features/dashboard/components/dashboard-overview';
import { PageHeader } from '@/components/layout/page-header';

export default function DashboardPage() {
  return (
    <div className='space-y-6'>
      <PageHeader
        title='ATSFlare 管理端迁移进度'
        description='新版前端已完成认证、核心链路与阶段 4 边缘模块迁移，当前可继续推进联调与回归。'
      />
      <DashboardOverview />
    </div>
  );
}
