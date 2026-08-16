'use client';

import Link from 'next/link';
import { useTranslations } from 'next-intl';
import {
  Card,
  CardContent,
  CardDescription,
  CardTitle,
} from '@/components/ui/card';
import { Bell, Key, Palette, UserRound } from 'lucide-react';

export default function SettingsPage() {
  const t = useTranslations('settings');

  const settingsItems = [
    {
      title: t('items.profile'),
      description: t('items.profileDesc'),
      icon: UserRound,
      href: '/settings/profile',
      category: t('categories.personal'),
    },
    {
      title: t('items.accessToken'),
      description: t('items.accessTokenDesc'),
      icon: Key,
      href: '/settings/access-token',
      category: t('categories.personal'),
    },
    {
      title: t('items.notifications'),
      description: t('items.notificationsDesc'),
      icon: Bell,
      href: '/settings/notifications',
      category: t('categories.account'),
    },
    {
      title: t('items.appearance'),
      description: t('items.appearanceDesc'),
      icon: Palette,
      href: '/settings/appearance',
      category: t('categories.personal'),
    },
  ];

  const groupedSettings = settingsItems.reduce(
    (acc, item) => {
      if (!acc[item.category]) {
        acc[item.category] = [];
      }
      acc[item.category].push(item);
      return acc;
    },
    {} as Record<string, typeof settingsItems>,
  );

  return (
    <div className='space-y-6 py-6'>
      <div className='font-semibold text-lg'>{t('title')}</div>

      {Object.entries(groupedSettings).map(([category, items]) => (
        <div key={category} className='space-y-4'>
          <div className='font-medium text-sm text-muted-foreground'>
            {category}
          </div>
          <div className='grid grid-cols-2 gap-4'>
            {items.map((item) => (
              <Link key={item.href} href={item.href}>
                <Card className='py-2 border border-dashed shadow-none hover:bg-muted/50 transition-colors cursor-pointer h-full'>
                  <CardContent>
                    <div className='flex items-center gap-4'>
                      <item.icon className='size-5 text-primary' />
                      <div>
                        <CardTitle className='mb-1 text-sm'>
                          {item.title}
                        </CardTitle>
                        <CardDescription className='text-xs'>
                          {item.description}
                        </CardDescription>
                      </div>
                    </div>
                  </CardContent>
                </Card>
              </Link>
            ))}
          </div>
        </div>
      ))}
    </div>
  );
}
