'use client';

import {X} from 'lucide-react';

import {Badge} from '@/components/ui/badge';
import {Button} from '@/components/ui/button';

interface RuleListSectionProps {
  title: string;
  description: string;
  items: string[];
  groupItems?: Array<{ id: number; name: string; enabled: boolean }>;
  tone: 'whitelist' | 'blacklist';
  emptyText: string;
  onRemove: (item: string) => void;
  onRemoveGroup?: (id: number) => void;
}

export function RuleListSection({
  title,
  description,
  items,
  groupItems = [],
  tone,
  emptyText,
  onRemove,
  onRemoveGroup,
}: RuleListSectionProps) {
  const total = items.length + groupItems.length;
  const badgeVariant = tone === 'whitelist' ? 'secondary' : 'destructive';

  return (
    <div className="rounded-lg border border-dashed p-4 space-y-3">
      <div className="flex items-start justify-between gap-3">
        <div>
          <h4 className="text-sm font-semibold">{title}</h4>
          <p className="text-xs text-muted-foreground mt-1">{description}</p>
        </div>
        <Badge variant="outline">{total}</Badge>
      </div>
      {total > 0 ? (
        <div className="flex flex-wrap gap-2">
          {groupItems.map((group) => (
            <Badge key={`group-${group.id}`} variant={badgeVariant} className="gap-1 pr-1">
              IP组: {group.name}
              {!group.enabled ? ' (停用)' : ''}
              <Button
                type="button"
                variant="ghost"
                size="icon"
                className="size-4 hover:bg-transparent"
                onClick={() => onRemoveGroup?.(group.id)}
              >
                <X className="size-3" />
              </Button>
            </Badge>
          ))}
          {items.map((item) => (
            <Badge key={item} variant={badgeVariant} className="gap-1 pr-1 font-mono text-xs">
              {item}
              <Button
                type="button"
                variant="ghost"
                size="icon"
                className="size-4 hover:bg-transparent"
                onClick={() => onRemove(item)}
              >
                <X className="size-3" />
              </Button>
            </Badge>
          ))}
        </div>
      ) : (
        <p className="text-sm text-muted-foreground">{emptyText}</p>
      )}
    </div>
  );
}
