'use client';

import { Badge } from '@/components/ui/badge';

import type { TunnelStatusTone } from './tunnel-utils';

export function TunnelStatusBadge({
  label,
  tone = 'info',
}: {
  label: string;
  tone?: TunnelStatusTone;
}) {
  const variant =
    tone === 'danger'
      ? 'destructive'
      : tone === 'success'
        ? 'secondary'
        : 'outline';

  return (
    <Badge
      variant={variant}
      className={
        tone === 'success'
          ? 'border-emerald-500/30 bg-emerald-500/10 text-emerald-700 dark:text-emerald-300'
          : tone === 'warning'
            ? 'border-amber-500/30 bg-amber-500/10 text-amber-700 dark:text-amber-300'
            : undefined
      }
    >
      {label}
    </Badge>
  );
}
