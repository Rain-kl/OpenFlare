'use client';

import * as React from 'react';
import { X } from 'lucide-react';

import { Badge } from '@/components/ui/badge';
import { cn } from '@/lib/utils';

export type TagsInputProps = {
  value: string[];
  onChange: (value: string[]) => void;
  placeholder?: string;
  disabled?: boolean;
  className?: string;
  id?: string;
  /** Called when a tag fails a custom validator; return true if accepted. */
  validateTag?: (tag: string) => string | null;
  'aria-invalid'?: boolean;
};

function normalizeTag(raw: string) {
  return raw.trim();
}

function TagsInput({
  value,
  onChange,
  placeholder = '输入后按 Enter 添加',
  disabled = false,
  className,
  id,
  validateTag,
  'aria-invalid': ariaInvalid,
}: TagsInputProps) {
  const [draft, setDraft] = React.useState('');
  const inputRef = React.useRef<HTMLInputElement>(null);

  const commit = (raw: string) => {
    const tag = normalizeTag(raw);
    if (!tag) {
      setDraft('');
      return;
    }

    if (validateTag) {
      const error = validateTag(tag);
      if (error) {
        return;
      }
    }

    if (value.includes(tag)) {
      setDraft('');
      return;
    }

    onChange([...value, tag]);
    setDraft('');
  };

  const removeAt = (index: number) => {
    onChange(value.filter((_, i) => i !== index));
  };

  const handleKeyDown = (event: React.KeyboardEvent<HTMLInputElement>) => {
    if (disabled) return;

    if (event.key === 'Enter' || event.key === ',' || event.key === 'Tab') {
      if (draft.trim()) {
        event.preventDefault();
        commit(draft);
      }
      return;
    }

    if (event.key === 'Backspace' && !draft && value.length > 0) {
      event.preventDefault();
      removeAt(value.length - 1);
    }
  };

  const handlePaste = (event: React.ClipboardEvent<HTMLInputElement>) => {
    const text = event.clipboardData.getData('text');
    if (!text || (!text.includes(',') && !text.includes('\n'))) {
      return;
    }
    event.preventDefault();
    const parts = text
      .split(/[,\n]+/)
      .map(normalizeTag)
      .filter(Boolean);
    if (parts.length === 0) return;

    const next = [...value];
    for (const part of parts) {
      if (validateTag) {
        const error = validateTag(part);
        if (error) continue;
      }
      if (!next.includes(part)) {
        next.push(part);
      }
    }
    onChange(next);
    setDraft('');
  };

  return (
    <div
      data-slot='tags-input'
      className={cn(
        'border-input dark:bg-input/30 flex min-h-8 w-full flex-wrap items-center gap-1.5 rounded-md border bg-transparent px-2 py-1 text-xs transition-[color,box-shadow] outline-none',
        'focus-within:border-ring focus-within:ring-ring/30 focus-within:ring-[2px]',
        ariaInvalid &&
          'ring-destructive/20 dark:ring-destructive/40 border-destructive',
        disabled && 'pointer-events-none cursor-not-allowed opacity-50',
        className,
      )}
      onClick={() => inputRef.current?.focus()}
    >
      {value.map((tag, index) => (
        <Badge
          key={`${tag}-${index}`}
          variant='secondary'
          className='gap-1 pr-1 font-normal'
        >
          <span className='max-w-[12rem] truncate'>{tag}</span>
          <button
            type='button'
            className='hover:bg-muted rounded-sm p-0.5 outline-none focus-visible:ring-1 focus-visible:ring-ring'
            aria-label={`移除 ${tag}`}
            disabled={disabled}
            onClick={(event) => {
              event.stopPropagation();
              removeAt(index);
            }}
          >
            <X className='size-3' />
          </button>
        </Badge>
      ))}
      <input
        ref={inputRef}
        id={id}
        value={draft}
        disabled={disabled}
        aria-invalid={ariaInvalid}
        placeholder={value.length === 0 ? placeholder : undefined}
        className='placeholder:text-muted-foreground min-w-[8rem] flex-1 bg-transparent py-0.5 text-xs outline-none'
        onChange={(event) => setDraft(event.target.value)}
        onKeyDown={handleKeyDown}
        onPaste={handlePaste}
        onBlur={() => {
          if (draft.trim()) {
            commit(draft);
          }
        }}
      />
    </div>
  );
}

export { TagsInput };
