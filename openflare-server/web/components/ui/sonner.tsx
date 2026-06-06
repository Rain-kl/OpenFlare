'use client';

import { useTheme } from '@/components/providers/theme-provider';
import { Toaster as Sonner } from 'sonner';

type ToasterProps = React.ComponentProps<typeof Sonner>;

export function Toaster({ ...props }: ToasterProps) {
  const { resolvedTheme } = useTheme();

  return (
    <Sonner
      position="bottom-right"
      theme={resolvedTheme as ToasterProps['theme']}
      className="toaster group"
      toastOptions={{
        classNames: {
          toast:
            'group toast group-[.toaster]:bg-[var(--surface-elevated)] group-[.toaster]:text-[var(--foreground-primary)] group-[.toaster]:border-[var(--border-default)] group-[.toaster]:shadow-lg group-[.toaster]:rounded-2xl group-[.toaster]:font-sans',
          description: 'group-[.toast]:text-[var(--foreground-secondary)]',
          actionButton:
            'group-[.toast]:bg-[var(--primary)] group-[.toast]:text-[var(--primary-foreground)]',
          cancelButton:
            'group-[.toast]:bg-[var(--muted)] group-[.toast]:text-[var(--muted-foreground)]',
        },
      }}
      {...props}
    />
  );
}
