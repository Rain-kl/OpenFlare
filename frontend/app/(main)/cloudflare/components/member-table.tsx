import { ArrowRightLeft, CircleAlert, RefreshCw, Trash2 } from 'lucide-react';
import { useTranslations } from 'next-intl';

import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Checkbox } from '@/components/ui/checkbox';
import { DropdownMenuItem } from '@/components/ui/dropdown-menu';
import { Switch } from '@/components/ui/switch';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table';
import type { CloudflareMember } from '@/lib/services/openflare';
import { DnsActionMenu } from '../../dns/components/dns-action-menu';

export function MemberTable({
  members,
  selectedMemberIDs,
  dnsOnly,
  updatingProxy,
  syncing,
  onSelect,
  onProxy,
  onMove,
  onSync,
  onRemove,
}: {
  members: CloudflareMember[];
  selectedMemberIDs: ReadonlySet<number>;
  dnsOnly: boolean;
  updatingProxy: boolean;
  syncing: boolean;
  onSelect: (ids: number[], checked: boolean) => void;
  onProxy: (memberID: number, proxied: boolean) => void;
  onMove: (member: CloudflareMember) => void;
  onSync: (memberID: number) => void;
  onRemove: (member: CloudflareMember) => void;
}) {
  const t = useTranslations('cloudflare');
  const selectedCount = members.filter((member) =>
    selectedMemberIDs.has(member.id),
  ).length;

  return (
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead className='w-12'>
            <Checkbox
              checked={
                selectedCount === members.length
                  ? true
                  : selectedCount > 0
                    ? 'indeterminate'
                    : false
              }
              onCheckedChange={(checked) =>
                onSelect(
                  members.map((member) => member.id),
                  checked === true,
                )
              }
              aria-label={t('selectVisibleMembers')}
            />
          </TableHead>
          <TableHead>{t('columns.domain')}</TableHead>
          <TableHead>{t('columns.desiredIp')}</TableHead>
          <TableHead>{t('columns.status')}</TableHead>
          <TableHead>{t('columns.proxied')}</TableHead>
          <TableHead className='text-right'>{t('columns.actions')}</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {members.map((member) => (
          <TableRow
            key={member.id}
            data-state={
              selectedMemberIDs.has(member.id) ? 'selected' : undefined
            }
          >
            <TableCell className='w-12'>
              <Checkbox
                checked={selectedMemberIDs.has(member.id)}
                onCheckedChange={(checked) =>
                  onSelect([member.id], checked === true)
                }
                aria-label={t('selectMember', { name: member.domain })}
              />
            </TableCell>
            <TableCell className='min-w-44 whitespace-normal'>
              <div className='font-medium wrap-anywhere'>{member.domain}</div>
              {member.last_error ? (
                <p className='max-w-md text-xs wrap-anywhere text-destructive'>
                  {member.last_error}
                </p>
              ) : null}
            </TableCell>
            <TableCell>
              <span className='font-mono text-xs'>
                {member.desired_ip || '—'}
              </span>
            </TableCell>
            <TableCell>
              <Badge
                variant={
                  member.sync_status === 'ok'
                    ? 'default'
                    : member.sync_status === 'error'
                      ? 'outline'
                      : 'secondary'
                }
              >
                {member.sync_status === 'error' && <CircleAlert />}
                {t(`memberStatus.${member.sync_status}`)}
              </Badge>
            </TableCell>
            <TableCell>
              <Switch
                checked={dnsOnly ? false : member.proxied}
                aria-label={t('proxyForMember', {
                  name: member.domain,
                })}
                disabled={dnsOnly || updatingProxy}
                title={dnsOnly ? t('smartdnsProxyLocked') : undefined}
                onCheckedChange={(proxied) => onProxy(member.id, proxied)}
              />
            </TableCell>
            <TableCell>
              <div className='flex justify-end gap-1'>
                <Button
                  variant='ghost'
                  size='sm'
                  onClick={() => onMove(member)}
                >
                  <ArrowRightLeft data-icon='inline-start' />
                  {t('move')}
                </Button>
                <DnsActionMenu name={member.domain}>
                  <DropdownMenuItem
                    disabled={syncing}
                    onSelect={() => onSync(member.id)}
                  >
                    <RefreshCw />
                    {t('sync')}
                  </DropdownMenuItem>
                  <DropdownMenuItem
                    variant='destructive'
                    onSelect={() => onRemove(member)}
                  >
                    <Trash2 />
                    {t('remove')}
                  </DropdownMenuItem>
                </DnsActionMenu>
              </div>
            </TableCell>
          </TableRow>
        ))}
      </TableBody>
    </Table>
  );
}
