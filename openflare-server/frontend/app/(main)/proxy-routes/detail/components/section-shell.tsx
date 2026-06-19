'use client';

import type {ReactNode} from 'react';

import {Button} from '@/components/ui/button';
import {Card, CardContent, CardDescription, CardHeader, CardTitle} from '@/components/ui/card';

interface SectionShellProps {
  title: string;
  description: string;
  formId: string;
  saving?: boolean;
  children: ReactNode;
}

export function SectionShell({
  title,
  description,
  formId,
  saving = false,
  children,
}: SectionShellProps) {
  return (
    <Card>
      <CardHeader className="flex flex-row items-start justify-between gap-4 space-y-0">
        <div className="space-y-1">
          <CardTitle className="text-sm font-semibold">{title}</CardTitle>
          <CardDescription>{description}</CardDescription>
        </div>
        <Button type="submit" form={formId} size="sm" className="h-8 shrink-0 text-xs" disabled={saving}>
          {saving ? '保存中...' : '保存'}
        </Button>
      </CardHeader>
      <CardContent>{children}</CardContent>
    </Card>
  );
}