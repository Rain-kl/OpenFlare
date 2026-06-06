import type {ReactNode} from 'react';

interface EmptyStateProps {
  title: string;
  description?: string;
  children?: ReactNode;
}

export function EmptyState({ title, description, children }: EmptyStateProps) {
  return (
    <div className="rounded-2xl border border-dashed border-[var(--border-default)] bg-[var(--surface-muted)] px-5 py-6 text-sm">
      <p className="text-base font-semibold text-[var(--foreground-primary)]">
        {title}
      </p>
      {description ? (
        <p className="mt-2 leading-6 text-[var(--foreground-secondary)]">
          {description}
        </p>
      ) : null}
      {children ? <div className="mt-4">{children}</div> : null}
    </div>
  );
}
