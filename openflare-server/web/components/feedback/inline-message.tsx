'use client';

import {useEffect, useRef} from 'react';
import {toast} from 'sonner';
import {cn} from '@/lib/utils/cn';

type InlineMessageTone = 'info' | 'success' | 'danger';

const toneClasses: Record<InlineMessageTone, string> = {
  info: 'border-[var(--status-info-border)] bg-[var(--status-info-soft)] text-[var(--status-info-foreground)]',
  success:
    'border-[var(--status-success-border)] bg-[var(--status-success-soft)] text-[var(--status-success-foreground)]',
  danger:
    'border-[var(--status-danger-border)] bg-[var(--status-danger-soft)] text-[var(--status-danger-foreground)]',
};

interface InlineMessageProps {
  tone?: InlineMessageTone;
  message: string;
  className?: string;
  onClear?: () => void;
}

export function InlineMessage({
  tone = 'info',
  message,
  className,
  onClear,
}: InlineMessageProps) {
  const onClearRef = useRef(onClear);

  useEffect(() => {
    onClearRef.current = onClear;
  }, [onClear]);

  useEffect(() => {
    // Only trigger toast if onClear is provided (dynamic feedback)
    if (!onClearRef.current || !message) return;

    const options = {
      position: 'bottom-right' as const,
    };

    if (tone === 'success') {
      toast.success(message, options);
    } else if (tone === 'danger') {
      toast.error(message, options);
    } else {
      toast(message, options);
    }

    const timer = setTimeout(() => {
      onClearRef.current?.();
    }, 0);
    return () => clearTimeout(timer);
  }, [tone, message]);

  // If onClear is provided, this is a toast feedback notice, so render nothing inline
  if (onClear) {
    return null;
  }

  // Otherwise, render as a static/persistent inline alert banner (original behavior)
  return (
    <div
      className={cn(
        'rounded-2xl border px-4 py-3 text-sm leading-6',
        toneClasses[tone],
        className,
      )}
    >
      {message}
    </div>
  );
}
