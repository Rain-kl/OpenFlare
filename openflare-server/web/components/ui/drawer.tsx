'use client';

import {
    cloneElement,
    createContext,
    type HTMLAttributes,
    isValidElement,
    type ReactElement,
    type ReactNode,
    useContext,
    useEffect,
    useId,
    useMemo,
    useState,
} from 'react';
import {createPortal} from 'react-dom';

import {cn} from '@/lib/utils/cn';

type DrawerDirection = 'top' | 'right' | 'bottom' | 'left';

type DrawerContextValue = {
  open: boolean;
  setOpen: (open: boolean) => void;
  direction: DrawerDirection;
};

const DrawerContext = createContext<DrawerContextValue | null>(null);

function useDrawerContext() {
  const context = useContext(DrawerContext);

  if (!context) {
    throw new Error('Drawer components must be used within Drawer.');
  }

  return context;
}

function renderWithOptionalChild(
  child: ReactNode,
  props: Record<string, unknown>,
  fallback: ReactNode,
) {
  if (isValidElement(child)) {
    return cloneElement(child as ReactElement, props);
  }

  return fallback;
}

export function Drawer({
  children,
  open,
  defaultOpen = false,
  onOpenChange,
  direction = 'bottom',
  title,
  description,
  footer,
}: {
  children: ReactNode;
  open?: boolean;
  defaultOpen?: boolean;
  onOpenChange?: (open: boolean) => void;
  direction?: DrawerDirection;
  title?: string;
  description?: string;
  footer?: ReactNode;
  size?: 'md' | 'lg' | 'xl';
}) {
  const [internalOpen, setInternalOpen] = useState(defaultOpen);
  const isControlled = open !== undefined;
  const resolvedOpen = isControlled ? open : internalOpen;

  const value = useMemo<DrawerContextValue>(
    () => ({
      open: resolvedOpen,
      direction,
      setOpen: (nextOpen) => {
        if (!isControlled) {
          setInternalOpen(nextOpen);
        }
        onOpenChange?.(nextOpen);
      },
    }),
    [direction, isControlled, onOpenChange, resolvedOpen],
  );

  return (
    <DrawerContext.Provider value={value}>
      {title || description || footer ? (
        <DrawerContent
          aria-label={title}
          className={cn(
            'w-full md:w-[50vw] md:max-w-none',
            direction === 'right' || direction === 'left' ? '' : 'max-h-[85vh]',
          )}
        >
          <DrawerHeader className="flex items-start justify-between gap-4">
            <div className="min-w-0">
              {title ? <DrawerTitle>{title}</DrawerTitle> : null}
              {description ? (
                <DrawerDescription>{description}</DrawerDescription>
              ) : null}
            </div>
            <button
              type="button"
              onClick={() => value.setOpen(false)}
              className="inline-flex h-10 w-10 items-center justify-center rounded-2xl border border-[var(--border-default)] bg-[var(--control-background)] text-lg text-[var(--foreground-primary)] transition hover:bg-[var(--control-background-hover)]"
              aria-label="关闭抽屉"
            >
              ×
            </button>
          </DrawerHeader>
          <div className="min-h-0 flex-1 overflow-y-auto px-6 py-6">
            {children}
          </div>
          {footer ? <DrawerFooter>{footer}</DrawerFooter> : null}
        </DrawerContent>
      ) : (
        children
      )}
    </DrawerContext.Provider>
  );
}

export function DrawerTrigger({
  children,
  asChild = false,
}: {
  children: ReactNode;
  asChild?: boolean;
}) {
  const { setOpen } = useDrawerContext();

  if (asChild) {
    return renderWithOptionalChild(
      children,
      {
        onClick: () => setOpen(true),
      },
      children,
    );
  }

  return (
    <button type="button" onClick={() => setOpen(true)}>
      {children}
    </button>
  );
}

export function DrawerClose({
  children,
  asChild = false,
}: {
  children: ReactNode;
  asChild?: boolean;
}) {
  const { setOpen } = useDrawerContext();

  if (asChild) {
    return renderWithOptionalChild(
      children,
      {
        onClick: () => setOpen(false),
      },
      children,
    );
  }

  return (
    <button type="button" onClick={() => setOpen(false)}>
      {children}
    </button>
  );
}

export function DrawerContent({
  children,
  className,
  'aria-label': ariaLabel,
}: HTMLAttributes<HTMLDivElement>) {
  const { open, setOpen, direction } = useDrawerContext();
  const titleId = useId();
  const descriptionId = useId();

  useEffect(() => {
    if (!open) {
      return;
    }

    const previousOverflow = document.body.style.overflow;
    document.body.style.overflow = 'hidden';

    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        setOpen(false);
      }
    };

    window.addEventListener('keydown', handleKeyDown);

    return () => {
      document.body.style.overflow = previousOverflow;
      window.removeEventListener('keydown', handleKeyDown);
    };
  }, [open, setOpen]);

  if (!open || typeof document === 'undefined') {
    return null;
  }

  const positionClassName =
    direction === 'right'
      ? 'inset-y-0 right-0 h-full border-l'
      : direction === 'left'
        ? 'inset-y-0 left-0 h-full border-r'
        : direction === 'top'
          ? 'inset-x-0 top-0 border-b'
          : 'inset-x-0 bottom-0 border-t';

  return createPortal(
    <div className="fixed inset-0 z-50">
      <button
        type="button"
        className="absolute inset-0 bg-slate-950/45 backdrop-blur-[2px]"
        onClick={() => setOpen(false)}
        aria-label="关闭抽屉"
      />
      <div
        role="dialog"
        aria-modal="true"
        aria-label={ariaLabel}
        aria-labelledby={ariaLabel ? undefined : titleId}
        aria-describedby={descriptionId}
        className={cn(
          'absolute flex w-full flex-col overflow-hidden border-[var(--border-default)] bg-[var(--surface-panel)] shadow-2xl',
          positionClassName,
          className,
        )}
      >
        <DrawerMetaContext.Provider value={{ titleId, descriptionId }}>
          {children}
        </DrawerMetaContext.Provider>
      </div>
    </div>,
    document.body,
  );
}

type DrawerMetaContextValue = {
  titleId: string;
  descriptionId: string;
};

const DrawerMetaContext = createContext<DrawerMetaContextValue | null>(null);

function useDrawerMetaContext() {
  const context = useContext(DrawerMetaContext);

  if (!context) {
    throw new Error(
      'Drawer title and description must be used within DrawerContent.',
    );
  }

  return context;
}

export function DrawerHeader({
  className,
  ...props
}: HTMLAttributes<HTMLDivElement>) {
  return (
    <div
      className={cn(
        'border-b border-[var(--border-default)] px-6 py-5',
        className,
      )}
      {...props}
    />
  );
}

export function DrawerFooter({
  className,
  ...props
}: HTMLAttributes<HTMLDivElement>) {
  return (
    <div
      className={cn(
        'border-t border-[var(--border-default)] px-6 py-4',
        className,
      )}
      {...props}
    />
  );
}

export function DrawerTitle({
  className,
  ...props
}: HTMLAttributes<HTMLHeadingElement>) {
  const { titleId } = useDrawerMetaContext();

  return (
    <h2
      id={titleId}
      className={cn(
        'text-xl font-semibold text-[var(--foreground-primary)]',
        className,
      )}
      {...props}
    />
  );
}

export function DrawerDescription({
  className,
  ...props
}: HTMLAttributes<HTMLParagraphElement>) {
  const { descriptionId } = useDrawerMetaContext();

  return (
    <p
      id={descriptionId}
      className={cn(
        'mt-2 text-sm leading-6 text-[var(--foreground-secondary)]',
        className,
      )}
      {...props}
    />
  );
}
