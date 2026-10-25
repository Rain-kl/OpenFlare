'use client';

import { Languages } from 'lucide-react';
import { useLocale } from 'next-intl';
import { Button } from '@/components/ui/button';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import { locales, type AppLocale } from '@/i18n/config';
import { switchLocale } from '@/i18n/client';
import { cn } from '@/lib/utils';
import enNames from '@/messages/fragments/languages.en.json';
import ruNames from '@/messages/fragments/languages.ru.json';
import zhCNNames from '@/messages/fragments/languages.zh-CN.json';

// Language names always use their own locale, regardless of the active UI.
const nativeNames: Record<AppLocale, string> = {
  en: enNames.languageNames.en,
  ru: ruNames.languageNames.ru,
  'zh-CN': zhCNNames.languageNames['zh-CN'],
};

type LanguageSwitcherProps = {
  variant?: 'icon' | 'full';
  className?: string;
};

export function LanguageSwitcher({
  variant = 'icon',
  className,
}: LanguageSwitcherProps) {
  const locale = useLocale() as AppLocale;

  if (variant === 'full') {
    return (
      <div className={cn('flex gap-2', className)}>
        {locales.map((item) => (
          <Button
            key={item}
            type='button'
            size='sm'
            variant={item === locale ? 'default' : 'secondary'}
            className='flex-1 text-xs'
            onClick={() => {
              if (item !== locale) switchLocale(item);
            }}
          >
            {nativeNames[item]}
          </Button>
        ))}
      </div>
    );
  }

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button
          variant='ghost'
          size='icon'
          className={cn(
            'size-9 text-muted-foreground hover:text-foreground',
            className,
          )}
          aria-label={nativeNames[locale]}
        >
          <Languages className='size-[18px]' />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align='end'>
        {locales.map((item) => (
          <DropdownMenuItem
            key={item}
            onClick={() => {
              if (item !== locale) switchLocale(item);
            }}
            className={item === locale ? 'bg-accent' : undefined}
          >
            {nativeNames[item]}
          </DropdownMenuItem>
        ))}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
