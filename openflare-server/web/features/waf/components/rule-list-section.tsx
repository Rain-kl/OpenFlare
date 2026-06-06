import {RuleChip} from './rule-chip';

export function RuleListSection({
  title,
  description,
  items,
  groupItems = [],
  tone,
  emptyText,
  onRemove,
  onRemoveGroup,
}: {
  title: string;
  description: string;
  items: string[];
  groupItems?: Array<{ id: number; name: string; enabled: boolean }>;
  tone: 'whitelist' | 'blacklist';
  emptyText: string;
  onRemove: (item: string) => void;
  onRemoveGroup?: (id: number) => void;
}) {
  const total = items.length + groupItems.length;
  return (
    <div className="rounded-[26px] border border-[var(--border-default)] bg-[var(--surface-elevated)] p-5">
      <div className="flex items-start justify-between gap-4">
        <div>
          <h3 className="text-sm font-semibold text-[var(--foreground-primary)]">
            {title}
          </h3>
          <p className="mt-1 text-xs leading-5 text-[var(--foreground-secondary)]">
            {description}
          </p>
        </div>
        <span className="shrink-0 rounded-full bg-[var(--surface-muted)] px-3 py-1 text-xs font-semibold text-[var(--foreground-primary)]">
          {total}
        </span>
      </div>
      <div className="mt-5">
        {total > 0 ? (
          <div className="flex flex-wrap gap-2">
            {groupItems.map((group) => (
              <RuleChip
                key={`group-${group.id}`}
                label={`IP组: ${group.name}${group.enabled ? '' : ' (停用)'}`}
                tone={tone}
                onRemove={() => onRemoveGroup?.(group.id)}
              />
            ))}
            {items.map((item) => (
              <RuleChip
                key={item}
                label={item}
                tone={tone}
                onRemove={() => onRemove(item)}
              />
            ))}
          </div>
        ) : (
          <p className="text-sm text-[var(--foreground-muted)]">{emptyText}</p>
        )}
      </div>
    </div>
  );
}
