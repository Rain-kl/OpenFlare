import type { LucideIcon } from 'lucide-react';

export function StatCard({
  icon: Icon,
  label,
  value,
  hint,
}: {
  icon: LucideIcon;
  label: string;
  value: string;
  hint?: string;
}) {
  return (
    <div className="rounded-[26px] border border-[var(--border-default)] bg-[var(--surface-elevated)] p-5">
      <div className="flex items-start justify-between gap-4">
        <div className="space-y-2">
          <p className="text-xs font-medium tracking-[0.22em] text-[var(--foreground-muted)] uppercase">
            {label}
          </p>
          <p className="text-2xl font-semibold text-[var(--foreground-primary)]">
            {value}
          </p>
        </div>
        <span className="inline-flex h-11 w-11 items-center justify-center rounded-2xl border border-[var(--border-default)] bg-[var(--surface-panel)] text-[var(--foreground-primary)]">
          <Icon className="h-5 w-5" />
        </span>
      </div>
      {hint && (
        <p className="mt-3 text-sm leading-6 text-[var(--foreground-secondary)]">
          {hint}
        </p>
      )}
    </div>
  );
}
