'use client';

import Link from 'next/link';
import { useTranslations } from 'next-intl';
import { Bell } from 'lucide-react';
import { Switch } from '@/components/ui/switch';
import { Label } from '@/components/ui/label';
import { useNotificationSettings } from '@/contexts/notification-settings-context';
import {
  Breadcrumb,
  BreadcrumbItem,
  BreadcrumbLink,
  BreadcrumbList,
  BreadcrumbPage,
  BreadcrumbSeparator,
} from '@/components/ui/breadcrumb';

export function NotificationsMain() {
  const { showBell, setShowBell } = useNotificationSettings();
  const t = useTranslations('settings');
  const tn = useTranslations('settings.notifications');

  return (
    <div className='py-6 space-y-6'>
      <div className='font-semibold'>
        <Breadcrumb>
          <BreadcrumbList>
            <BreadcrumbItem>
              <BreadcrumbLink asChild>
                <Link href='/settings' className='text-base text-primary'>
                  {t('title')}
                </Link>
              </BreadcrumbLink>
            </BreadcrumbItem>
            <BreadcrumbSeparator />
            <BreadcrumbItem>
              <BreadcrumbPage className='text-base font-semibold'>
                {tn('breadcrumb')}
              </BreadcrumbPage>
            </BreadcrumbItem>
          </BreadcrumbList>
        </Breadcrumb>
      </div>

      <div className='space-y-6'>
        <div>
          <h2 className='font-medium text-sm text-foreground'>
            {tn('display')}
          </h2>
          <p className='text-xs text-muted-foreground'>{tn('displayDesc')}</p>
        </div>

        <div className='flex items-center justify-between rounded-lg border p-4'>
          <div className='flex items-center gap-3'>
            <Bell className='size-5 text-primary' />
            <div className='space-y-0.5'>
              <Label
                htmlFor='show-bell'
                className='text-sm font-medium cursor-pointer'
              >
                {tn('showBell')}
              </Label>
              <p className='text-xs text-muted-foreground'>
                {tn('showBellDesc')}
              </p>
            </div>
          </div>
          <Switch
            id='show-bell'
            checked={showBell}
            onCheckedChange={setShowBell}
          />
        </div>
      </div>
    </div>
  );
}
