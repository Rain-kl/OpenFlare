import { cn } from '@/lib/utils/cn';

export function TabButton({
  label,
  active,
  onClick,
}: {
  label: string;
  active: boolean;
  onClick: () => void;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={cn(
        'group rounded-[24px] border px-4 py-4 text-left transition',
        active
          ? 'border-[var(--border-strong)] bg-[var(--accent-soft)] shadow-[var(--shadow-soft)]'
          : 'border-[var(--border-default)] bg-[var(--surface-elevated)] hover:border-[var(--border-strong)] hover:bg-[var(--surface-muted)]',
      )}
    >
      <p className="text-sm font-semibold text-[var(--foreground-primary)]">
        {label}
      </p>
    </button>
  );
}
