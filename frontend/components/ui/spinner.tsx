'use client';

import { useTranslations } from 'next-intl';
import { Loader2Icon } from 'lucide-react';

import { cn } from '@/lib/utils';

function Spinner({ className, ...props }: React.ComponentProps<'svg'>) {
  const uiText = useTranslations('runtime');

  return (
    <Loader2Icon
      role='status'
      aria-label={uiText('ui_loadingLabel')}
      className={cn('size-4 animate-spin', className)}
      {...props}
    />
  );
}

export { Spinner };
