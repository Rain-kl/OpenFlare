'use client';

import { useMutation, useQuery } from '@tanstack/react-query';
import { Loader2, Play } from 'lucide-react';
import { useTranslations } from 'next-intl';
import { useEffect, useState } from 'react';
import { toast } from 'sonner';

import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table';
import {
  SmartDNSService,
  smartdnsQueryKey,
  type SmartDNSSimulateResult,
} from '@/lib/services/openflare';
import { getErrorMessage } from '../../websites/components/website-utils';

export function SmartDNSSimulatorDrawer({
  open,
  onOpenChange,
  initialPolicyId,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  initialPolicyId?: number;
}) {
  const t = useTranslations('smartdns.simulator');
  const [domain, setDomain] = useState('www.example.com');
  const [qtype, setQType] = useState<'A' | 'AAAA'>('A');
  const [clientIP, setClientIP] = useState('223.5.5.5');
  const [policyId, setPolicyId] = useState<string>('auto');
  const [result, setResult] = useState<SmartDNSSimulateResult | null>(null);

  const policiesQuery = useQuery({
    queryKey: [...smartdnsQueryKey, 'policies'],
    queryFn: () => SmartDNSService.listPolicies(),
    enabled: open,
  });

  useEffect(() => {
    if (open && initialPolicyId) {
      setPolicyId(String(initialPolicyId));
    }
  }, [initialPolicyId, open]);

  const simulateMutation = useMutation({
    mutationFn: () =>
      SmartDNSService.simulate({
        domain: domain.trim(),
        qtype,
        client_ip: clientIP.trim(),
        policy_id: policyId !== 'auto' ? Number(policyId) : undefined,
      }),
    onSuccess: (res) => setResult(res),
    onError: (err) => toast.error(getErrorMessage(err)),
  });

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent className='w-full overflow-y-auto sm:max-w-3xl'>
        <SheetHeader>
          <SheetTitle>{t('title')}</SheetTitle>
          <SheetDescription>{t('description')}</SheetDescription>
        </SheetHeader>

        <div className='space-y-5 px-4 pb-6'>
          <div className='grid gap-3 rounded-lg border border-dashed p-4 sm:grid-cols-2'>
            <div className='space-y-1.5'>
              <Label htmlFor='sim-domain'>{t('domain')}</Label>
              <Input
                id='sim-domain'
                value={domain}
                placeholder='app.example.com'
                onChange={(e) => setDomain(e.target.value)}
              />
            </div>
            <div className='space-y-1.5'>
              <Label htmlFor='sim-client-ip'>{t('clientIp')}</Label>
              <Input
                id='sim-client-ip'
                value={clientIP}
                placeholder='223.5.5.5'
                onChange={(e) => setClientIP(e.target.value)}
              />
            </div>
            <div className='space-y-1.5'>
              <Label htmlFor='sim-qtype'>{t('qtype')}</Label>
              <Select
                value={qtype}
                onValueChange={(v) => setQType(v as 'A' | 'AAAA')}
              >
                <SelectTrigger id='sim-qtype' className='w-full'>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value='A'>A (IPv4)</SelectItem>
                  <SelectItem value='AAAA'>AAAA (IPv6)</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div className='space-y-1.5'>
              <Label htmlFor='sim-policy'>{t('policy')}</Label>
              <Select value={policyId} onValueChange={setPolicyId}>
                <SelectTrigger id='sim-policy' className='w-full'>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value='auto'>{t('policyAuto')}</SelectItem>
                  {(policiesQuery.data ?? []).map((p) => (
                    <SelectItem key={p.id} value={String(p.id)}>
                      {p.name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <div className='sm:col-span-2 flex justify-end'>
              <Button
                disabled={
                  simulateMutation.isPending ||
                  !domain.trim() ||
                  !clientIP.trim()
                }
                onClick={() => simulateMutation.mutate()}
              >
                {simulateMutation.isPending ? (
                  <Loader2 data-icon='inline-start' className='animate-spin' />
                ) : (
                  <Play data-icon='inline-start' />
                )}
                {t('run')}
              </Button>
            </div>
          </div>

          {result ? (
            <div className='space-y-4'>
              <div className='grid gap-3 rounded-lg border border-dashed p-3 text-xs sm:grid-cols-2'>
                <div>
                  <span className='text-muted-foreground'>
                    {t('clientPrefix')}:{' '}
                  </span>
                  <span className='font-mono'>
                    {result.client.client_prefix}
                  </span>
                </div>
                <div>
                  <span className='text-muted-foreground'>Geo / ASN: </span>
                  <span>
                    {result.client.country_iso || '-'}
                    {result.client.region_code
                      ? `-${result.client.region_code}`
                      : ''}{' '}
                    · AS{result.client.asn} {result.client.asn_name ?? ''}
                  </span>
                </div>
                <div>
                  <span className='text-muted-foreground'>
                    {t('matchedPolicy')}:{' '}
                  </span>
                  <span className='font-medium'>
                    {result.selection.policy_name} ({result.selection.mode}, TTL{' '}
                    {result.selection.ttl}s)
                  </span>
                </div>
                <div className='flex flex-wrap gap-1.5'>
                  {result.selection.hysteresis_applied ? (
                    <Badge variant='secondary'>{t('hysteresisBadge')}</Badge>
                  ) : null}
                  {result.selection.fallback_used ? (
                    <Badge variant='destructive'>
                      {t('fallbackBadge')}: {result.selection.fallback_reason}
                    </Badge>
                  ) : null}
                </div>
              </div>

              <div className='space-y-2'>
                <h4 className='text-sm font-medium'>{t('selectedAnswers')}</h4>
                {result.selection.answers.length === 0 ? (
                  <p className='text-xs text-muted-foreground'>
                    {t('noAnswers')}
                  </p>
                ) : (
                  <div className='flex flex-wrap gap-2'>
                    {result.selection.answers.map((ans) => (
                      <Badge
                        key={`${ans.node_id}-${ans.ip}`}
                        variant='default'
                        className='font-mono'
                      >
                        {ans.name} → {ans.ip} ({ans.score.toFixed(3)})
                      </Badge>
                    ))}
                  </div>
                )}
              </div>

              <div className='space-y-2'>
                <h4 className='text-sm font-medium'>{t('candidateTrace')}</h4>
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>{t('colNode')}</TableHead>
                      <TableHead>{t('colTier')}</TableHead>
                      <TableHead>{t('colRttLoss')}</TableHead>
                      <TableHead>{t('colLoad')}</TableHead>
                      <TableHead>{t('colScore')}</TableHead>
                      <TableHead>{t('colState')}</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {result.selection.candidates.map((c) => (
                      <TableRow key={c.node_id}>
                        <TableCell>
                          <div className='font-medium text-xs'>{c.name}</div>
                          <div className='font-mono text-[11px] text-muted-foreground'>
                            {c.ip || '-'}
                          </div>
                        </TableCell>
                        <TableCell className='text-xs'>
                          {c.source_tier || '-'}
                        </TableCell>
                        <TableCell className='text-xs tabular-nums'>
                          {c.effective_rtt_ms.toFixed(1)} ms /{' '}
                          {(c.loss_rate * 100).toFixed(1)}%
                        </TableCell>
                        <TableCell className='text-xs tabular-nums'>
                          {(c.load_index * 100).toFixed(0)}%
                        </TableCell>
                        <TableCell className='font-mono text-xs tabular-nums'>
                          {c.final_score.toFixed(3)}
                        </TableCell>
                        <TableCell>
                          {c.selected ? (
                            <Badge variant='default'>
                              {t('stateSelected')}
                            </Badge>
                          ) : c.overloaded ? (
                            <Badge variant='destructive'>
                              {t('stateOverloaded')}
                            </Badge>
                          ) : c.exclusion_reason ? (
                            <Badge variant='outline'>
                              {c.exclusion_reason}
                            </Badge>
                          ) : (
                            <Badge variant='secondary'>
                              {t('stateCandidate')}
                            </Badge>
                          )}
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              </div>
            </div>
          ) : null}
        </div>
      </SheetContent>
    </Sheet>
  );
}
