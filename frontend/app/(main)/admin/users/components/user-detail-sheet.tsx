'use client';

import * as React from 'react';
import { useTranslations } from 'next-intl';
import {
  Globe,
  Loader2,
  Mail,
  MapPin,
  ShieldCheck,
  Smartphone,
  UserCheck,
} from 'lucide-react';

import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { Avatar, AvatarFallback, AvatarImage } from '@/components/ui/avatar';
import { Sheet, SheetContent, SheetTitle } from '@/components/ui/sheet';
import type { AdminUser } from '@/lib/services/admin';
import { formatDateTime } from '@/lib/utils';

interface UserDetailSheetProps {
  selectedUser: AdminUser | null;
  isOpen: boolean;
  onOpenChange: (open: boolean) => void;
  detailLoading: boolean;
  onStatusToggle: (user: AdminUser) => Promise<void>;
}

export function UserDetailSheet({
  selectedUser,
  isOpen,
  onOpenChange,
  detailLoading,
  onStatusToggle,
}: UserDetailSheetProps) {
  const t = useTranslations('admin.users');
  const displayValue = (value?: string) =>
    value && value.trim() ? value : '-';

  return (
    <Sheet open={isOpen} onOpenChange={onOpenChange}>
      <SheetContent className='sm:max-w-[400px] w-full p-0 flex flex-col gap-0'>
        <SheetTitle className='px-5 py-3'>{t('userProfile')}</SheetTitle>

        {selectedUser && (
          <>
            <div className='flex-1 overflow-y-auto scrollbar-thin scrollbar-thumb-border scrollbar-track-transparent'>
              <div className='flex flex-col pb-6'>
                <div className='px-5 py-6 border-b border-border/50'>
                  <div className='flex flex-col items-center text-center gap-3'>
                    <Avatar className='h-20 w-20 rounded-full border-4 border-background ring-1 ring-border/20'>
                      <AvatarImage src={selectedUser.avatar_url} />
                      <AvatarFallback className='rounded-full text-xl font-medium bg-secondary text-secondary-foreground'>
                        {selectedUser.username.substring(0, 2).toUpperCase()}
                      </AvatarFallback>
                    </Avatar>

                    <div className='space-y-1.5'>
                      <h3 className='text-lg font-bold tracking-tight'>
                        {selectedUser.nickname}
                      </h3>
                      <div className='flex items-center justify-center gap-2'>
                        <code className='px-1.5 py-0.5 rounded-md bg-muted text-[10px] font-mono text-muted-foreground'>
                          @{selectedUser.username}
                        </code>
                        <Badge
                          variant='secondary'
                          className='h-4.5 px-1.5 text-[9px] uppercase font-medium'
                        >
                          UID: {selectedUser.id}
                        </Badge>
                        {selectedUser.is_admin && (
                          <Badge className='h-4.5 px-1.5 text-[9px] uppercase font-medium'>
                            Admin
                          </Badge>
                        )}
                      </div>
                    </div>

                    {detailLoading && (
                      <div className='flex items-center gap-1 text-[10px] text-muted-foreground'>
                        <Loader2 className='size-3 animate-spin' />
                        {t('refreshingDetail')}
                      </div>
                    )}

                    <div className='gap-4 w-full max-w-[240px] mt-1 pt-4 border-t border-border/50'>
                      <div className='flex flex-col gap-0.5'>
                        <span className='text-[9px] uppercase tracking-widest text-muted-foreground font-medium'>
                          {t('registeredAt')}
                        </span>
                        <span className='font-mono text-xs font-semibold'>
                          {
                            formatDateTime(selectedUser.created_at).split(
                              ' ',
                            )[0]
                          }
                        </span>
                      </div>
                    </div>
                  </div>
                </div>

                <div className='p-6 space-y-6'>
                  <div className='space-y-4'>
                    <h4 className='text-xs font-semibold text-muted-foreground uppercase tracking-wider px-1'>
                      {t('personalInfo')}
                    </h4>
                    <div className='rounded-lg border divide-y bg-background/50'>
                      <div className='flex items-center justify-between gap-4 p-3.5 text-sm'>
                        <span className='flex items-center gap-2 text-[10px] text-muted-foreground'>
                          <Mail className='size-3' />
                          {t('email')}
                        </span>
                        <span className='min-w-0 truncate text-right text-[10px]'>
                          {displayValue(selectedUser.email)}
                        </span>
                      </div>
                      <div className='flex items-center justify-between gap-4 p-3.5 text-sm'>
                        <span className='flex items-center gap-2 text-[10px] text-muted-foreground'>
                          <Smartphone className='size-3' />
                          {t('phone')}
                        </span>
                        <span className='min-w-0 truncate text-right text-[10px]'>
                          {displayValue(selectedUser.phone)}
                        </span>
                      </div>
                      <div className='flex items-center justify-between gap-4 p-3.5 text-sm'>
                        <span className='flex items-center gap-2 text-[10px] text-muted-foreground'>
                          <span className='size-3 flex items-center justify-center font-bold text-[9px]'>
                            ⚧
                          </span>
                          {t('gender')}
                        </span>
                        <span className='min-w-0 truncate text-right text-[10px]'>
                          {displayValue(selectedUser.gender)}
                        </span>
                      </div>
                      <div className='flex items-center justify-between gap-4 p-3.5 text-sm'>
                        <span className='flex items-center gap-2 text-[10px] text-muted-foreground'>
                          <MapPin className='size-3' />
                          {t('location')}
                        </span>
                        <span className='min-w-0 truncate text-right text-[10px]'>
                          {displayValue(selectedUser.location)}
                        </span>
                      </div>
                      <div className='flex items-center justify-between gap-4 p-3.5 text-sm'>
                        <span className='flex items-center gap-2 text-[10px] text-muted-foreground'>
                          <Globe className='size-3' />
                          {t('website')}
                        </span>
                        <span className='min-w-0 truncate text-right text-[10px]'>
                          {displayValue(selectedUser.website)}
                        </span>
                      </div>
                      <div className='flex flex-col gap-2 p-3.5 text-sm'>
                        <span className='text-[10px] text-muted-foreground'>
                          {t('bio')}
                        </span>
                        <span className='break-words text-[10px] leading-5'>
                          {displayValue(selectedUser.bio)}
                        </span>
                      </div>
                    </div>
                  </div>

                  <div className='space-y-4'>
                    <h4 className='text-xs font-semibold text-muted-foreground uppercase tracking-wider px-1'>
                      {t('systemRecords')}
                    </h4>
                    <div className='rounded-lg border divide-y bg-background/50'>
                      <div className='flex items-center justify-between p-3.5 text-sm'>
                        <span className='text-[10px]'>
                          {t('accountStatus')}
                        </span>
                        <Badge
                          variant={
                            selectedUser.is_active ? 'secondary' : 'outline'
                          }
                          className='text-[10px]'
                        >
                          {selectedUser.is_active
                            ? t('statusActive')
                            : t('statusDisabled')}
                        </Badge>
                      </div>
                      <div className='flex items-center justify-between p-3.5 text-sm'>
                        <span className='text-[10px]'>{t('isAdmin')}</span>
                        <span className='font-mono text-[10px]'>
                          {selectedUser.is_admin ? t('yes') : t('no')}
                        </span>
                      </div>
                      <div className='flex items-center justify-between p-3.5 text-sm'>
                        <span className='text-[10px]'>{t('lastLogin')}</span>
                        <span className='font-mono text-[10px]'>
                          {formatDateTime(selectedUser.last_login_at)}
                        </span>
                      </div>
                      <div className='flex items-center justify-between p-3.5 text-sm'>
                        <span className='text-[10px]'>{t('registeredAt')}</span>
                        <span className='font-mono text-[10px]'>
                          {formatDateTime(selectedUser.created_at)}
                        </span>
                      </div>
                      <div className='flex items-center justify-between p-3.5 text-sm'>
                        <span className='text-[10px]'>{t('lastUpdate')}</span>
                        <span className='font-mono text-[10px]'>
                          {formatDateTime(selectedUser.updated_at)}
                        </span>
                      </div>
                    </div>
                  </div>
                </div>
              </div>

              {!selectedUser.is_admin && (
                <div className='p-4 border-t bg-background/80 backdrop-blur-md shrink-0'>
                  <Button
                    variant={selectedUser.is_active ? 'destructive' : 'default'}
                    className='w-full h-9 text-xs font-medium transition-all active:scale-[0.98]'
                    onClick={() => onStatusToggle(selectedUser)}
                  >
                    {selectedUser.is_active ? (
                      <>
                        <ShieldCheck className='size-3 mr-1' />
                        {t('banAccount')}
                      </>
                    ) : (
                      <>
                        <UserCheck className='size-3 mr-1' />
                        {t('unbanAccount')}
                      </>
                    )}
                  </Button>
                </div>
              )}
            </div>
          </>
        )}
      </SheetContent>
    </Sheet>
  );
}
