'use client';

import { useEffect } from 'react';
import { toast } from 'sonner';

type InlineMessageTone = 'info' | 'success' | 'danger';

interface InlineMessageProps {
  tone?: InlineMessageTone;
  message: string;
  className?: string;
  onClear?: () => void;
}

export function InlineMessage({
  tone = 'info',
  message,
  onClear,
}: InlineMessageProps) {
  useEffect(() => {
    if (!message) return;

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

    if (onClear) {
      const timer = setTimeout(() => {
        onClear();
      }, 0);
      return () => clearTimeout(timer);
    }
  }, [tone, message, onClear]);

  return null;
}
