import { GitBranch, Play, Star, Trash2 } from 'lucide-react';
import Link from 'next/link';
import { useTranslations } from 'next-intl';

import { EmptyStateWithBorder } from '@/components/layout/empty';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { DropdownMenuItem } from '@/components/ui/dropdown-menu';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table';
import type { SmartDNSPolicy } from '@/lib/services/openflare';
import { DnsActionMenu } from '../components/dns-action-menu';

export function SmartDNSPolicyList({
  policies,
  onSimulate,
  onSetDefault,
  onDelete,
}: {
  policies: SmartDNSPolicy[];
  onSimulate: (id: number) => void;
  onSetDefault: (policy: SmartDNSPolicy) => void;
  onDelete: (policy: SmartDNSPolicy) => void;
}) {
  const t = useTranslations('smartdns.policies');
  if (policies.length === 0)
    return <EmptyStateWithBorder icon={GitBranch} description={t('empty')} />;

  return (
    <div className='rounded-lg border border-dashed'>
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead className='pl-4'>{t('name')}</TableHead>
            <TableHead>{t('strategyType')}</TableHead>
            <TableHead>{t('scopeType')}</TableHead>
            <TableHead>{t('statusColumn')}</TableHead>
            <TableHead className='pr-4 text-right'>
              {t('actionsColumn')}
            </TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {policies.map((policy) => (
            <TableRow key={policy.id}>
              <TableCell className='min-w-36 py-3 pl-4 whitespace-normal align-top'>
                <Button
                  variant='link'
                  className='h-auto p-0 text-left whitespace-normal text-foreground'
                  asChild
                >
                  <Link href={`/dns/smartdns/policies/editor?id=${policy.id}`}>
                    {policy.name}
                  </Link>
                </Button>
                {policy.description && (
                  <p className='text-xs text-muted-foreground'>
                    {policy.description}
                  </p>
                )}
              </TableCell>
              <TableCell className='py-3 align-top'>
                {t(
                  policy.strategy_type === 'custom'
                    ? 'strategyCustomJdm'
                    : policy.strategy_type === 'load_balance'
                      ? 'strategyLB'
                      : 'strategySmart',
                )}
                <p className='text-xs text-muted-foreground tabular-nums'>
                  TTL {policy.ttl}s ·{' '}
                  {t('maxAnswersSummary', { count: policy.max_answers })}
                </p>
              </TableCell>
              <TableCell className='min-w-36 py-3 whitespace-normal align-top'>
                <p className='wrap-anywhere'>
                  {policy.scope_type === 'domains'
                    ? (policy.scope_domains ?? []).join(', ')
                    : t('scopeAll')}
                </p>
                <p className='text-xs text-muted-foreground'>
                  {t('prioritySummary', { value: policy.priority })}
                </p>
              </TableCell>
              <TableCell className='py-3 align-top'>
                <div className='flex flex-wrap gap-1'>
                  <Badge variant={policy.enabled ? 'default' : 'secondary'}>
                    {t(policy.enabled ? 'enabled' : 'disabled')}
                  </Badge>
                  {policy.is_default && (
                    <Badge variant='outline'>{t('defaultBadge')}</Badge>
                  )}
                </div>
              </TableCell>
              <TableCell className='pr-4 text-right'>
                <div className='flex justify-end gap-1'>
                  <Button
                    variant='ghost'
                    size='sm'
                    onClick={() => onSimulate(policy.id)}
                  >
                    <Play data-icon='inline-start' />
                    {t('simulate')}
                  </Button>
                  {!policy.is_default && (
                    <DnsActionMenu name={policy.name}>
                      <DropdownMenuItem
                        disabled={!policy.enabled}
                        onSelect={() => onSetDefault(policy)}
                      >
                        <Star />
                        {t('setDefault')}
                      </DropdownMenuItem>
                      <DropdownMenuItem
                        variant='destructive'
                        onSelect={() => onDelete(policy)}
                      >
                        <Trash2 />
                        {t('delete')}
                      </DropdownMenuItem>
                    </DnsActionMenu>
                  )}
                </div>
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  );
}
