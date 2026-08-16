// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

'use client';

import * as React from 'react';
import { useTranslations } from 'next-intl';
import { Code, Cpu, Database, HardDrive } from 'lucide-react';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Progress } from '@/components/ui/progress';

export function DashboardTab() {
  const t = useTranslations('admin.demo.dashboard');
  return (
    <div className='space-y-6'>
      <div className='grid grid-cols-1 md:grid-cols-3 gap-4'>
        {/* CPU */}
        <Card className='border-dashed shadow-none'>
          <CardHeader className='flex flex-row items-center justify-between pb-2'>
            <span className='text-xs font-medium text-muted-foreground'>
              {t('cpuUsage')}
            </span>
            <Cpu className='size-4 text-primary' />
          </CardHeader>
          <CardContent className='space-y-2'>
            <div className='text-2xl font-semibold tracking-tight'>42.8 %</div>
            <Progress value={42.8} className='h-1.5' />
            <p className='text-[10px] text-muted-foreground flex items-center gap-1'>
              <span className='size-1.5 rounded-full bg-emerald-500 inline-block animate-pulse' />
              {t('cpuStatus')}
            </p>
          </CardContent>
        </Card>

        {/* Storage */}
        <Card className='border-dashed shadow-none'>
          <CardHeader className='flex flex-row items-center justify-between pb-2'>
            <span className='text-xs font-medium text-muted-foreground'>
              {t('storageUsage')}
            </span>
            <HardDrive className='size-4 text-primary' />
          </CardHeader>
          <CardContent className='space-y-2'>
            <div className='text-2xl font-semibold tracking-tight'>
              72.4 GB{' '}
              <span className='text-xs text-muted-foreground'>/ 100 GB</span>
            </div>
            <Progress value={72.4} className='h-1.5' />
            <p className='text-[10px] text-muted-foreground'>
              {t('storageStatus')}
            </p>
          </CardContent>
        </Card>

        {/* DB Overview */}
        <Card className='border-dashed shadow-none'>
          <CardHeader className='flex flex-row items-center justify-between pb-2'>
            <span className='text-xs font-medium text-muted-foreground'>
              {t('database')}
            </span>
            <Database className='size-4 text-primary' />
          </CardHeader>
          <CardContent className='space-y-2'>
            <div className='text-2xl font-semibold tracking-tight'>
              PostgreSQL{' '}
              <span className='text-xs font-normal text-muted-foreground'>
                (v16.2)
              </span>
            </div>
            <div className='flex items-center gap-3 text-xs text-muted-foreground'>
              <div>
                {t('activeConnections')}{' '}
                <span className='font-semibold text-foreground'>12</span>
              </div>
              <div>
                {t('totalTables')}{' '}
                <span className='font-semibold text-foreground'>34</span>
              </div>
            </div>
            <p className='text-[10px] text-muted-foreground'>{t('dbStatus')}</p>
          </CardContent>
        </Card>
      </div>

      {/* 指标卡片规范与代码 */}
      <Card className='border-dashed shadow-none bg-muted/20'>
        <CardHeader className='pb-3'>
          <CardTitle className='text-sm font-semibold flex items-center gap-1.5'>
            <Code className='size-4 text-primary' />
            {t('dashboardDesignSpec')}
          </CardTitle>
        </CardHeader>
        <CardContent className='space-y-4'>
          <ul className='list-disc pl-4 text-xs text-muted-foreground space-y-1.5'>
            <li>
              {t('spec.dashedBorder')}
              <code className='bg-muted px-1 py-0.5 rounded font-mono'>
                border-dashed shadow-none
              </code>{' '}
              {t('spec.dashedBorder2')}
            </li>
            <li>
              {t('spec.headerLayout')}
              <code className='bg-muted px-1 py-0.5 rounded font-mono'>
                flex flex-row items-center justify-between pb-2
              </code>{' '}
              {t('spec.headerLayout2')}
            </li>
            <li>
              {t('spec.valueStyle')}
              <code className='bg-muted px-1 py-0.5 rounded font-mono'>
                text-2xl font-semibold tracking-tight
              </code>{' '}
              {t('spec.valueStyle2')}
            </li>
            <li>
              {t('spec.progressBar')}
              <code className='bg-muted px-1 py-0.5 rounded font-mono'>
                h-1.5
              </code>{' '}
              {t('spec.progressBar2')}
            </li>
            <li>
              {t('spec.secondaryText')}
              <code className='bg-muted px-1 py-0.5 rounded font-mono'>
                text-[10px] text-muted-foreground
              </code>{' '}
              {t('spec.secondaryText2')}
            </li>
          </ul>
          <pre className='text-[11px] font-mono text-muted-foreground overflow-x-auto p-3 bg-background rounded border border-border/40 leading-relaxed'>
            {`<Card className="border-dashed shadow-none">
  <CardHeader className="flex flex-row items-center justify-between pb-2">
    <span className="text-xs font-medium text-muted-foreground">指标标题</span>
    <Cpu className="size-4 text-primary" />
  </CardHeader>
  <CardContent className="space-y-2">
    <div className="text-2xl font-semibold tracking-tight">42.8 %</div>
    <Progress value={42.8} className="h-1.5" />
    <p className="text-[10px] text-muted-foreground">状态说明小字</p>
  </CardContent>
</Card>`}
          </pre>
        </CardContent>
      </Card>
    </div>
  );
}
