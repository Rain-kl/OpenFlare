'use client';

import type {ReactNode} from 'react';
import type {LucideIcon} from 'lucide-react';

import {Card, CardContent, CardDescription, CardHeader, CardTitle} from '@/components/ui/card';
import {cn} from '@/lib/utils';

export function NodeInfoRow({
  label,
  children,
  className,
}: {
  label: string;
  children: ReactNode;
  className?: string;
}) {
  return (
    <div className={cn('flex items-start justify-between gap-4 py-2.5 text-sm', className)}>
      <span className="shrink-0 text-muted-foreground">{label}</span>
      <div className="min-w-0 text-right font-medium">{children}</div>
    </div>
  );
}

export function NodeKpiCard({
  label,
  value,
  icon: Icon,
}: {
  label: string;
  value: ReactNode;
  icon?: LucideIcon;
}) {
  return (
    <div className="rounded-xl border bg-card/60 px-4 py-3.5 backdrop-blur-sm">
      <div className="flex items-center gap-2 text-xs text-muted-foreground">
        {Icon ? <Icon className="size-3.5 opacity-70" /> : null}
        <span>{label}</span>
      </div>
      <p className="mt-2 text-sm font-semibold tracking-tight break-all">{value}</p>
    </div>
  );
}

export function NodeSectionCard({
  title,
  description,
  action,
  children,
  className,
}: {
  title: string;
  description?: string;
  action?: React.ReactNode;
  children: ReactNode;
  className?: string;
}) {
  return (
    <Card className={cn('border shadow-none', className)}>
      <CardHeader className={cn('pb-3', action ? 'flex-row items-start justify-between space-y-0' : '')}>
        <div className="space-y-1">
          <CardTitle className="text-base font-semibold">{title}</CardTitle>
          {description ? <CardDescription>{description}</CardDescription> : null}
        </div>
        {action}
      </CardHeader>
      <CardContent>{children}</CardContent>
    </Card>
  );
}

export function NodeErrorBanner({ message }: { message: string }) {
  return (
    <div className="rounded-xl border border-destructive/25 bg-destructive/5 px-4 py-3 text-sm text-destructive">
      {message}
    </div>
  );
}