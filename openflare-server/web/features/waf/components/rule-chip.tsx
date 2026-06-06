import {cn} from '@/lib/utils/cn';

export function RuleChip({
  label,
  tone,
  onRemove,
}: {
  label: string;
  tone: 'whitelist' | 'blacklist';
  onRemove: () => void;
}) {
  return (
    <span
      className={cn(
        'inline-flex items-center gap-2 rounded-full border px-3 py-2 text-sm',
        tone === 'whitelist'
          ? 'border-emerald-500/30 bg-emerald-500/18 text-[var(--foreground-primary)]'
          : 'border-rose-500/30 bg-rose-500/18 text-[var(--foreground-primary)]',
      )}
    >
      <span className="break-all">{label}</span>
      <button
        type="button"
        onClick={onRemove}
        className="inline-flex h-5 w-5 items-center justify-center rounded-full border border-current/20 text-xs text-[var(--foreground-primary)] transition hover:bg-black/10"
        aria-label={`移除 ${label}`}
      >
        ×
      </button>
    </span>
  );
}
