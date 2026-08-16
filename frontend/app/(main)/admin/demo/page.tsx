// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

'use client';

import * as React from 'react';
import { useTranslations } from 'next-intl';
import { Code } from 'lucide-react';

import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';

// 引入模块化拆分后的 Tab 子组件
import { DashboardTab } from './tabs/dashboard-tab';
import { TableTab } from './tabs/table-tab';
import { ControlsTab } from './tabs/controls-tab';

export default function HeaderDemoPage() {
  const t = useTranslations('admin.demo');
  const [activeTab, setActiveTab] = React.useState('dashboard');

  return (
    <div className='py-6 px-1 space-y-6'>
      {/* 1. 标准页面标题 */}
      <div className='flex items-center gap-2'>
        <Code className='size-5 text-primary' />
        <div>
          <h1 className='text-2xl font-semibold tracking-tight'>
            {t('pageTitle')}
          </h1>
        </div>
      </div>

      {/* 说明区域 */}
      <Card className='border-dashed shadow-none'>
        <CardHeader className='pb-3'>
          <div className='flex items-center gap-2'>
            <Code className='size-4 text-primary' />
            <CardTitle className='text-base font-semibold'>
              {t('specTitle')}
            </CardTitle>
          </div>
          <CardDescription>{t('specDescription')}</CardDescription>
        </CardHeader>
        <CardContent className='space-y-4'>
          <div className='space-y-2'>
            <h3 className='text-xs font-semibold uppercase tracking-wider text-muted-foreground'>
              {t('coreSpecTitle')}
            </h3>
            <ul className='list-disc pl-4 text-xs text-muted-foreground space-y-1.5'>
              <li>
                <strong>{t('rules.structureFocus.title')}</strong>
                {t('rules.structureFocus.description')}
              </li>
              <li>
                <strong>{t('rules.containerAlignment.title')}</strong>：使用{' '}
                <code className='bg-muted px-1 py-0.5 rounded font-mono'>
                  flex items-center gap-2
                </code>{' '}
                {t('rules.containerAlignment.description')}{' '}
                <code className='bg-muted px-1 py-0.5 rounded font-mono'>
                  py-6 px-1
                </code>{' '}
                (或{' '}
                <code className='bg-muted px-1 py-0.5 rounded font-mono'>
                  py-6
                </code>
                ) {t('rules.containerAlignment.description2')}
              </li>
              <li>
                <strong>{t('rules.iconPresentation.title')}</strong>：直接将
                Lucide 图标组件嵌套于标题容器中，应用{' '}
                <code className='bg-muted px-1 py-0.5 rounded font-mono'>
                  size-5 text-primary
                </code>{' '}
                {t('rules.iconPresentation.description')}
              </li>
              <li>
                <strong>{t('rules.textStyle.title')}</strong>：文本使用且仅使用{' '}
                <code className='bg-muted px-1 py-0.5 rounded font-mono'>
                  {'h1 className="text-2xl font-semibold tracking-tight"'}
                </code>
                {t('rules.textStyle.description')}{' '}
                <code className='bg-muted px-1 py-0.5 rounded font-mono'>
                  font-bold
                </code>
                {t('rules.textStyle.description2')}
              </li>
              <li>
                <strong>{t('rules.tabsModular.title')}</strong>：凡是带有多个
                Tab 页切换的页面，**禁止**将所有 Tab
                的渲染代码堆积在同一个物理页面主文件内。每个 Tab 页对应的
                Content 必须单独拆分为独立组件文件（如{' '}
                <code className='bg-muted px-1 py-0.5 rounded font-mono'>
                  tabs/events-tab.tsx
                </code>{' '}
                {t('rules.tabsModular.description')}{' '}
                <code className='bg-muted px-1 py-0.5 rounded font-mono'>
                  components/
                </code>{' '}
                {t('rules.tabsModular.description2')}
              </li>
              <li>
                <strong>{t('rules.complexityDriven.title')}</strong>：不仅是
                Tabs 切换，当一个页面代码行数超过 600
                {t('rules.complexityDriven.description')}{' '}
                <code className='bg-muted px-1 py-0.5 rounded font-mono'>
                  app/(main)/admin/database/components/
                </code>
                {t('rules.complexityDriven.description2')}{' '}
                <code className='bg-muted px-1 py-0.5 rounded font-mono'>
                  components/common/
                </code>
                {t('rules.complexityDriven.description3')}{' '}
                <code className='bg-muted px-1 py-0.5 rounded font-mono'>
                  /admin/database
                </code>
                ) 的{' '}
                <code className='bg-muted px-1 py-0.5 rounded font-mono'>
                  table-browser.tsx
                </code>
                、
                <code className='bg-muted px-1 py-0.5 rounded font-mono'>
                  cache-manager.tsx
                </code>{' '}
                与{' '}
                <code className='bg-muted px-1 py-0.5 rounded font-mono'>
                  sql-console.tsx
                </code>
                。
              </li>
            </ul>
          </div>
        </CardContent>
      </Card>

      {/* Tabs 切换 */}
      <Tabs value={activeTab} onValueChange={setActiveTab} className='w-full'>
        <TabsList variant='line' className='w-fit inline-flex gap-8 mb-6'>
          <TabsTrigger
            value='dashboard'
            className='px-0 pb-2 text-xs font-semibold'
          >
            {t('tabs.dashboard')}
          </TabsTrigger>
          <TabsTrigger
            value='table'
            className='px-0 pb-2 text-xs font-semibold'
          >
            {t('tabs.table')}
          </TabsTrigger>
          <TabsTrigger
            value='controls'
            className='px-0 pb-2 text-xs font-semibold'
          >
            {t('tabs.controls')}
          </TabsTrigger>
        </TabsList>

        <TabsContent value='dashboard' className='focus-visible:outline-none'>
          <DashboardTab />
        </TabsContent>
        <TabsContent value='table' className='focus-visible:outline-none'>
          <TableTab />
        </TabsContent>
        <TabsContent value='controls' className='focus-visible:outline-none'>
          <ControlsTab />
        </TabsContent>
      </Tabs>
    </div>
  );
}
