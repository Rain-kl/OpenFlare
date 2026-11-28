'use client';

import type { ComponentProps, HTMLAttributes, ReactNode } from 'react';
import type { LucideIcon } from 'lucide-react';
import Link from 'next/link';
import { Button } from '@/components/ui/button';
import { Switch } from '@/components/ui/switch';
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip';
import { cn } from '@/lib/utils';

export type ListActionsAlign = 'center' | 'end' | 'start';
export type ListActionTone = 'default' | 'destructive' | 'primary';

export interface ListActionsProps extends HTMLAttributes<HTMLDivElement> {
  /** Horizontal alignment inside table cell or card row (defaults to center). */
  align?: ListActionsAlign;
  /** Stop click events from bubbling to clickable parent rows. */
  stopPropagation?: boolean;
  children: ReactNode;
}

/**
 * Compact horizontal container for icon-only row actions matching /admin/users.
 */
export function ListActions({
  align = 'center',
  stopPropagation = false,
  className,
  onClick,
  children,
  ...props
}: ListActionsProps) {
  return (
    <div
      className={cn(
        'flex items-center gap-0.5 whitespace-nowrap',
        align === 'center' && 'justify-center',
        align === 'end' && 'justify-end',
        align === 'start' && 'justify-start',
        className,
      )}
      onClick={
        stopPropagation
          ? (e) => {
              e.stopPropagation();
              onClick?.(e);
            }
          : onClick
      }
      {...props}
    >
      {children}
    </div>
  );
}

export interface ListActionButtonProps
  extends Omit<ComponentProps<typeof Button>, 'variant' | 'size' | 'children'> {
  icon: LucideIcon;
  /** Accessible name used for aria-label, sr-only link text, and default tooltip. */
  label: string;
  /** Custom tooltip content; pass null to disable tooltip rendering. */
  tooltip?: ReactNode | null;
  tooltipSide?: ComponentProps<typeof TooltipContent>['side'];
  /** Route target when the action navigates instead of firing an imperative callback. */
  href?: string;
  /** Open href in a new tab with noopener noreferrer. */
  external?: boolean;
  tone?: ListActionTone;
  /** Disables the button and spins the icon during async mutations. */
  loading?: boolean;
  iconClassName?: string;
}

const TONE_CLASS_MAP: Record<ListActionTone, string> = {
  default: 'text-muted-foreground hover:text-foreground',
  destructive:
    'text-muted-foreground hover:text-destructive hover:bg-destructive/10',
  primary: 'text-muted-foreground hover:text-primary',
};

/**
 * Compact 24x24 ghost icon button with accessible label and top tooltip.
 */
export function ListActionButton({
  icon: Icon,
  label,
  tooltip,
  tooltipSide = 'top',
  href,
  external = false,
  tone = 'default',
  loading = false,
  disabled = false,
  iconClassName,
  className,
  type = 'button',
  ...buttonProps
}: ListActionButtonProps) {
  const isDisabled = Boolean(disabled || loading);
  const resolvedTooltip = tooltip === undefined ? label : tooltip;
  const buttonClassName = cn('size-6', TONE_CLASS_MAP[tone], className);
  const iconNode = (
    <Icon
      className={cn('size-3', loading && 'animate-spin', iconClassName)}
      aria-hidden='true'
    />
  );

  let control: ReactNode;
  if (href && !isDisabled) {
    control = (
      <Button
        variant='ghost'
        size='icon'
        className={buttonClassName}
        asChild
        {...buttonProps}
      >
        {external ? (
          <a
            href={href}
            target='_blank'
            rel='noopener noreferrer'
            aria-label={label}
          >
            {iconNode}
            <span className='sr-only'>{label}</span>
          </a>
        ) : (
          <Link href={href} aria-label={label}>
            {iconNode}
            <span className='sr-only'>{label}</span>
          </Link>
        )}
      </Button>
    );
  } else {
    control = (
      <Button
        type={type}
        variant='ghost'
        size='icon'
        disabled={isDisabled}
        aria-label={label}
        className={buttonClassName}
        {...buttonProps}
      >
        {iconNode}
      </Button>
    );
  }

  if (!resolvedTooltip) {
    return control;
  }

  return (
    <Tooltip>
      <TooltipTrigger asChild>
        {isDisabled ? <span className='inline-flex'>{control}</span> : control}
      </TooltipTrigger>
      <TooltipContent side={tooltipSide} className='text-xs'>
        {resolvedTooltip}
      </TooltipContent>
    </Tooltip>
  );
}

export interface ListActionSwitchProps {
  checked: boolean;
  onCheckedChange: (checked: boolean) => void;
  disabled?: boolean;
  tooltip?: ReactNode;
  tooltipSide?: ComponentProps<typeof TooltipContent>['side'];
  ariaLabel: string;
  className?: string;
}

/**
 * Compact scaled row status switch wrapped with an optional tooltip.
 */
export function ListActionSwitch({
  checked,
  onCheckedChange,
  disabled = false,
  tooltip,
  tooltipSide = 'top',
  ariaLabel,
  className,
}: ListActionSwitchProps) {
  const control = (
    <span className='inline-flex items-center'>
      <Switch
        checked={checked}
        onCheckedChange={onCheckedChange}
        disabled={disabled}
        aria-label={ariaLabel}
        className={cn('scale-75', className)}
      />
    </span>
  );

  if (!tooltip) {
    return control;
  }

  return (
    <Tooltip>
      <TooltipTrigger asChild>{control}</TooltipTrigger>
      <TooltipContent side={tooltipSide} className='text-xs'>
        {tooltip}
      </TooltipContent>
    </Tooltip>
  );
}
