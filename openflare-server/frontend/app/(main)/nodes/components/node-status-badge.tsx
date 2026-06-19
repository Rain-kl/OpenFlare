import {Badge} from '@/components/ui/badge';
import {cn} from '@/lib/utils';

import type {StatusTone} from './node-utils';

const toneClassName: Record<StatusTone, string> = {
  success: 'bg-emerald-500/10 border-emerald-500/20 text-emerald-600',
  warning: 'bg-amber-500/10 border-amber-500/20 text-amber-600',
  danger: 'bg-destructive/10 border-destructive/20 text-destructive',
  info: 'bg-blue-500/10 border-blue-500/20 text-blue-600',
};

export function NodeStatusBadge({
  label,
  tone,
  className,
}: {
  label: string;
  tone: StatusTone;
  className?: string;
}) {
  return (
    <Badge
      variant="outline"
      className={cn('text-[10px] rounded-full py-0 px-2 font-medium', toneClassName[tone], className)}
    >
      {label}
    </Badge>
  );
}
