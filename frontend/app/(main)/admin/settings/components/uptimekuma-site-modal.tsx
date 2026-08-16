'use client';

import { useTranslations } from 'next-intl';
import { useEffect, useMemo, useState } from 'react';
import { useQuery } from '@tanstack/react-query';

import { Button } from '@/components/ui/button';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { ProxyRouteService } from '@/lib/services/openflare';

type UptimeKumaSiteSelectModalProps = {
  open: boolean;
  selectedSites: string[];
  onOpenChange: (open: boolean) => void;
  onSave: (sites: string[]) => void;
};

export function UptimeKumaSiteSelectModal({
  open,
  selectedSites,
  onOpenChange,
  onSave,
}: UptimeKumaSiteSelectModalProps) {
  const t = useTranslations('openflareOps.siteModal');
  const tCommon = useTranslations('common');
  const [searchTerm, setSearchTerm] = useState('');
  const [tempSelected, setTempSelected] = useState<Set<string>>(new Set());

  const routesQuery = useQuery({
    queryKey: ['openflare', 'proxy-routes'],
    queryFn: () => ProxyRouteService.list(),
    enabled: open,
  });

  useEffect(() => {
    if (!open) return;
    setTempSelected(
      new Set(selectedSites.map((site) => site.trim()).filter(Boolean)),
    );
    setSearchTerm('');
  }, [open, selectedSites]);

  const filteredRoutes = useMemo(() => {
    const routes = routesQuery.data ?? [];
    const keyword = searchTerm.trim().toLowerCase();
    if (!keyword) return routes;
    return routes.filter((route) => {
      const domains = (route.zone_domains ?? [])
        .map((item) => item.domain)
        .join(' ');
      return (
        route.site_name.toLowerCase().includes(keyword) ||
        domains.toLowerCase().includes(keyword)
      );
    });
  }, [routesQuery.data, searchTerm]);

  const toggleSite = (siteName: string) => {
    setTempSelected((previous) => {
      const next = new Set(previous);
      if (next.has(siteName)) next.delete(siteName);
      else next.add(siteName);
      return next;
    });
  };

  const handleSelectAll = () => {
    setTempSelected((previous) => {
      const next = new Set(previous);
      filteredRoutes.forEach((route) => next.add(route.site_name));
      return next;
    });
  };

  const handleDeselectAll = () => {
    setTempSelected((previous) => {
      const next = new Set(previous);
      filteredRoutes.forEach((route) => next.delete(route.site_name));
      return next;
    });
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className='max-w-2xl'>
        <DialogHeader>
          <DialogTitle>{t('title')}</DialogTitle>
          <DialogDescription>{t('description')}</DialogDescription>
        </DialogHeader>

        <div className='space-y-4'>
          <div className='space-y-1.5'>
            <Label>{t('search')}</Label>
            <Input
              value={searchTerm}
              onChange={(event) => setSearchTerm(event.target.value)}
              placeholder={t('searchPlaceholder')}
            />
          </div>

          <div className='flex flex-wrap gap-2'>
            <Button
              type='button'
              variant='outline'
              size='sm'
              onClick={handleSelectAll}
            >
              {t('selectFiltered')}
            </Button>
            <Button
              type='button'
              variant='outline'
              size='sm'
              onClick={handleDeselectAll}
            >
              {t('clearFiltered')}
            </Button>
          </div>

          <div className='max-h-72 overflow-y-auto rounded-lg border'>
            {routesQuery.isLoading ? (
              <p className='p-4 text-sm text-muted-foreground'>
                {t('loading')}
              </p>
            ) : routesQuery.isError ? (
              <p className='p-4 text-sm text-destructive'>
                {routesQuery.error instanceof Error
                  ? routesQuery.error.message
                  : t('loadFailed')}
              </p>
            ) : filteredRoutes.length === 0 ? (
              <p className='p-4 text-sm text-muted-foreground'>
                {t('noMatch')}
              </p>
            ) : (
              <table className='w-full text-left text-sm'>
                <thead className='sticky top-0 bg-muted/60 text-xs uppercase text-muted-foreground'>
                  <tr>
                    <th className='w-12 px-3 py-2'>{t('select')}</th>
                    <th className='px-3 py-2'>{t('siteName')}</th>
                    <th className='px-3 py-2'>{t('primaryDomain')}</th>
                  </tr>
                </thead>
                <tbody className='divide-y'>
                  {filteredRoutes.map((route) => {
                    const checked = tempSelected.has(route.site_name);
                    return (
                      <tr
                        key={route.id}
                        className='cursor-pointer hover:bg-muted/30'
                        onClick={() => toggleSite(route.site_name)}
                      >
                        <td
                          className='px-3 py-2'
                          onClick={(event) => event.stopPropagation()}
                        >
                          <input
                            type='checkbox'
                            checked={checked}
                            onChange={() => toggleSite(route.site_name)}
                            className='size-4 rounded border-input'
                          />
                        </td>
                        <td className='px-3 py-2 font-medium'>
                          {route.site_name}
                        </td>
                        <td className='px-3 py-2 text-muted-foreground'>
                          {(route.zone_domains ?? [])
                            .map((item) => item.domain)
                            .join(', ') || '—'}
                        </td>
                      </tr>
                    );
                  })}
                </tbody>
              </table>
            )}
          </div>

          <p className='text-right text-xs text-muted-foreground'>
            {t('selectedCount', { count: tempSelected.size })}
          </p>
        </div>

        <DialogFooter>
          <Button
            type='button'
            variant='outline'
            onClick={() => onOpenChange(false)}
          >
            {tCommon('cancel')}
          </Button>
          <Button
            type='button'
            onClick={() => {
              onSave(Array.from(tempSelected));
              onOpenChange(false);
            }}
          >
            {t('save')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
