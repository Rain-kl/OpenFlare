import * as React from 'react';

import { cn } from '@/lib/utils/cn';

type SwitchProps = Omit<React.ComponentProps<'button'>, 'onChange'> & {
  checked?: boolean;
  defaultChecked?: boolean;
  disabled?: boolean;
  onCheckedChange?: (checked: boolean) => void;
};

const Switch = React.forwardRef<HTMLButtonElement, SwitchProps>(
  (
    {
      className,
      checked,
      defaultChecked = false,
      disabled,
      onCheckedChange,
      ...props
    },
    ref,
  ) => {
    const [internalChecked, setInternalChecked] = React.useState(defaultChecked);
    const isControlled = checked !== undefined;
    const isChecked = isControlled ? checked : internalChecked;

    const updateChecked = React.useCallback(
      (nextChecked: boolean) => {
        if (!isControlled) {
          setInternalChecked(nextChecked);
        }
        onCheckedChange?.(nextChecked);
      },
      [isControlled, onCheckedChange],
    );

    return (
      <button
        ref={ref}
        type="button"
        role="switch"
        aria-checked={isChecked}
        data-state={isChecked ? 'checked' : 'unchecked'}
        disabled={disabled}
        className={cn(
          'peer inline-flex h-6 w-11 shrink-0 items-center rounded-full border border-transparent bg-[var(--control-background-hover)] transition-colors outline-none focus-visible:ring-2 focus-visible:ring-[var(--accent-soft)] focus-visible:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50 data-[state=checked]:bg-[var(--brand-primary)]',
          className,
        )}
        onClick={() => updateChecked(!isChecked)}
        onKeyDown={(event) => {
          if (event.key === 'Enter' || event.key === ' ') {
            event.preventDefault();
            updateChecked(!isChecked);
          }
        }}
        {...props}
      >
        <span
          data-state={isChecked ? 'checked' : 'unchecked'}
          className="pointer-events-none block h-5 w-5 translate-x-0.5 rounded-full bg-white shadow-sm transition-transform data-[state=checked]:translate-x-5"
        />
      </button>
    );
  },
);

Switch.displayName = 'Switch';

export { Switch };
