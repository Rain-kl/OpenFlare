import { cn } from '@/lib/utils';

interface BrandMarkProps {
  className?: string;
  variant?: 'icon' | 'wordmark';
}

export function BrandMark({ className, variant = 'icon' }: BrandMarkProps) {
  return (
    <span
      aria-hidden='true'
      className={cn(
        'openflare-brand inline-block h-6 shrink-0',
        variant === 'icon' ? 'aspect-square' : 'aspect-[1886/457]',
        className,
      )}
      data-variant={variant}
    />
  );
}
