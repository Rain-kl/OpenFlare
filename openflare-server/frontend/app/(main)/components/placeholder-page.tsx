import type {LucideIcon} from 'lucide-react';

import {Card, CardDescription, CardHeader, CardTitle} from '@/components/ui/card';

interface PlaceholderPageProps {
  title: string;
  icon: LucideIcon;
}

export function PlaceholderPage({title, icon: Icon}: PlaceholderPageProps) {
  return (
    <div className="py-6 px-1 space-y-6">
      <div className="flex items-center gap-2">
        <Icon className="size-5 text-primary" />
        <h1 className="text-2xl font-semibold tracking-tight">{title}</h1>
      </div>

      <Card>
        <CardHeader>
          <CardTitle className="text-base">开发中</CardTitle>
          <CardDescription>该功能正在迁移开发中，敬请期待。</CardDescription>
        </CardHeader>
      </Card>
    </div>
  );
}