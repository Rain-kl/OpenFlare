import * as React from 'react';

import { cn } from '@/lib/utils/cn';

const Label = React.forwardRef<
  HTMLLabelElement,
  React.ComponentProps<'label'>
>(({ className, ...props }, ref) => (
  <label
    ref={ref}
    className={cn(
      'text-sm font-medium leading-none text-[var(--foreground-primary)] peer-disabled:cursor-not-allowed peer-disabled:opacity-70',
      className,
    )}
    {...props}
  />
));

Label.displayName = 'Label';

export { Label };
