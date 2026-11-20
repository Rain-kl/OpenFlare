'use client';

import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { ChevronRight, Unlink } from 'lucide-react';
import { useTranslations } from 'next-intl';
import Link from 'next/link';
import { useState } from 'react';
import { toast } from 'sonner';

import { EmptyStateWithBorder } from '@/components/layout/empty';
import { ErrorInline } from '@/components/layout/error';
import { LoadingStateWithBorder } from '@/components/layout/loading';
import {
  AlertDialog,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger,
} from '@/components/ui/alert-dialog';
import { Button } from '@/components/ui/button';
import { Checkbox } from '@/components/ui/checkbox';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import type { WAFRule } from '@/lib/services/openflare';
import {
  ruleGroupSitesQueryKey,
  WafService,
} from '@/lib/services/openflare/waf.service';

import { getErrorMessage } from './helpers';

export function RuleGroupSitesDialog({ group }: { group: WAFRule }) {
  const t = useTranslations('waf');
  const scope = group.is_global
    ? t('allSites')
    : t('siteCount', { count: group.applied_site_count });
  return (
    <Dialog>
      <DialogTrigger asChild>
        <Button
          type='button'
          variant='link'
          className='h-auto px-0'
          aria-label={t('sites.open', { name: group.name, scope })}
        >
          {scope}
          <ChevronRight data-icon='inline-end' />
        </Button>
      </DialogTrigger>
      <DialogContent className='md:max-w-2xl'>
        <DialogHeader className='pr-6'>
          <DialogTitle className='wrap-anywhere'>
            {t('sites.title', { name: group.name })}
          </DialogTitle>
          <DialogDescription className='text-left'>
            {t(
              group.is_global ? 'sites.globalDescription' : 'sites.description',
            )}
          </DialogDescription>
        </DialogHeader>
        <RuleGroupSites group={group} />
      </DialogContent>
    </Dialog>
  );
}

function RuleGroupSites({ group }: { group: WAFRule }) {
  const t = useTranslations('waf.sites');
  const tw = useTranslations('waf');
  const tc = useTranslations('common');
  const queryClient = useQueryClient();
  const [search, setSearch] = useState('');
  const [selected, setSelected] = useState<Set<number>>(() => new Set());
  const [confirmIDs, setConfirmIDs] = useState<number[] | null>(null);
  const sitesKey = [...ruleGroupSitesQueryKey, group.id];

  const sitesQuery = useQuery({
    queryKey: sitesKey,
    queryFn: () => WafService.listRuleGroupSites(group.id),
    refetchOnMount: 'always',
  });
  const removal = useMutation({
    mutationFn: (ids: number[]) =>
      WafService.removeRuleGroupSites(group.id, ids),
    onSuccess: async ({ removed_count }) => {
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: sitesKey }),
        queryClient.invalidateQueries({
          queryKey: ['openflare', 'waf', 'rule-groups'],
        }),
        queryClient.invalidateQueries({
          queryKey: ['openflare', 'waf', 'site-rule-groups'],
        }),
      ]);
      setConfirmIDs(null);
      setSelected(new Set());
      toast.success(
        removed_count
          ? t('removed', { count: removed_count })
          : t('alreadyRemoved'),
      );
    },
    onError: (error) =>
      toast.error(getErrorMessage(error, tw('operationFailed'))),
  });

  if (sitesQuery.isPending) {
    return <LoadingStateWithBorder description={t('loading')} />;
  }
  if (sitesQuery.isError) {
    return (
      <ErrorInline
        message={getErrorMessage(sitesQuery.error, tw('operationFailed'))}
        onRetry={() => void sitesQuery.refetch()}
      />
    );
  }

  const sites = sitesQuery.data;
  const keyword = search.trim().toLowerCase();
  const visible = sites.filter(
    (site) =>
      site.site_name.toLowerCase().includes(keyword) ||
      site.domains.some((domain) => domain.toLowerCase().includes(keyword)),
  );
  const selectedIDs = sites
    .filter((site) => selected.has(site.id))
    .map((site) => site.id);
  const selectedVisibleCount = visible.filter((site) =>
    selected.has(site.id),
  ).length;
  const allVisibleSelected =
    visible.length > 0 && selectedVisibleCount === visible.length;

  function toggle(ids: number[], checked: boolean) {
    setSelected((current) => {
      const next = new Set(current);
      for (const id of ids) {
        if (checked) next.add(id);
        else next.delete(id);
      }
      return next;
    });
  }

  return (
    <>
      {sites.length > 0 && (
        <Input
          type='search'
          aria-label={t('search')}
          placeholder={t('search')}
          value={search}
          onChange={(event) => setSearch(event.target.value)}
        />
      )}
      {visible.length > 0 ? (
        <div className='min-w-0 rounded-lg border border-dashed'>
          {!group.is_global && (
            <label className='flex items-center gap-3 border-b border-dashed px-3 py-2 text-sm text-muted-foreground'>
              <Checkbox
                checked={
                  allVisibleSelected ||
                  (selectedVisibleCount > 0 && 'indeterminate')
                }
                onCheckedChange={(checked) =>
                  toggle(
                    visible.map((site) => site.id),
                    checked === true,
                  )
                }
              />
              {t('selectVisible')}
            </label>
          )}
          <ul className='max-h-[45dvh] divide-y divide-dashed overflow-y-auto'>
            {visible.map((site) => {
              const name =
                site.site_name ||
                site.domains[0] ||
                t('unnamed', { id: site.id });
              const domains = site.site_name
                ? site.domains
                : site.domains.slice(1);
              return (
                <li
                  className='flex min-w-0 items-center gap-3 px-3 py-2.5'
                  key={site.id}
                >
                  {!group.is_global && (
                    <Checkbox
                      aria-label={t('select', { name })}
                      checked={selected.has(site.id)}
                      onCheckedChange={(checked) =>
                        toggle([site.id], checked === true)
                      }
                    />
                  )}
                  <Link
                    href={`/proxy-routes/detail?id=${site.id}&section=waf`}
                    className='flex min-w-0 flex-1 items-center justify-between gap-3 rounded-sm text-primary outline-offset-4 focus-visible:outline-2'
                  >
                    <span className='flex min-w-0 flex-col gap-0.5 text-sm wrap-anywhere'>
                      <span className='font-medium underline-offset-4 hover:underline'>
                        {name}
                      </span>
                      {domains.length > 0 && (
                        <span className='text-xs text-muted-foreground'>
                          {domains.join(' · ')}
                        </span>
                      )}
                    </span>
                    <ChevronRight
                      aria-hidden='true'
                      className='size-4 shrink-0'
                    />
                  </Link>
                </li>
              );
            })}
          </ul>
        </div>
      ) : (
        <EmptyStateWithBorder
          description={t(sites.length ? 'noResults' : 'empty')}
        />
      )}
      {!group.is_global && (sites.length > 0 || confirmIDs !== null) && (
        <div className='flex flex-wrap items-center justify-between gap-3'>
          <span className='text-sm text-muted-foreground' role='status'>
            {t('selected', { count: selectedIDs.length })}
          </span>
          <AlertDialog
            open={confirmIDs !== null}
            onOpenChange={(open) => {
              if (!removal.isPending) setConfirmIDs(open ? selectedIDs : null);
            }}
          >
            <AlertDialogTrigger asChild>
              <Button
                type='button'
                variant='destructive'
                size='sm'
                disabled={selectedIDs.length === 0}
              >
                <Unlink data-icon='inline-start' />
                {t('removeSelected')}
              </Button>
            </AlertDialogTrigger>
            <AlertDialogContent>
              <AlertDialogHeader>
                <AlertDialogTitle>{t('confirmTitle')}</AlertDialogTitle>
                <AlertDialogDescription>
                  {t('confirmDescription', {
                    name: group.name,
                    count: confirmIDs?.length ?? 0,
                  })}
                </AlertDialogDescription>
              </AlertDialogHeader>
              <AlertDialogFooter>
                <AlertDialogCancel disabled={removal.isPending}>
                  {tc('cancel')}
                </AlertDialogCancel>
                <Button
                  type='button'
                  variant='destructive'
                  disabled={removal.isPending || !confirmIDs?.length}
                  onClick={() => confirmIDs && removal.mutate(confirmIDs)}
                >
                  {t(removal.isPending ? 'removing' : 'removeSelected')}
                </Button>
              </AlertDialogFooter>
            </AlertDialogContent>
          </AlertDialog>
        </div>
      )}
    </>
  );
}
