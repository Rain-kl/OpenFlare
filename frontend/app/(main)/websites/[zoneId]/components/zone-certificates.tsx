'use client';

import Link from 'next/link';
import { useMemo } from 'react';
import { ExternalLink, FileKey } from 'lucide-react';

import { EmptyStateWithBorder } from '@/components/layout/empty';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table';
import type {
  TlsCertificateItem,
  ZoneDomainItem,
} from '@/lib/services/openflare';
import { formatDateTime } from '@/lib/utils';

import { getCertificateStatus } from '../../components/website-utils';

export function ZoneCertificatesPanel({
  domains,
  certificates,
}: {
  domains: ZoneDomainItem[];
  certificates: TlsCertificateItem[];
}) {
  const certificateMap = useMemo(
    () =>
      new Map(certificates.map((certificate) => [certificate.id, certificate])),
    [certificates],
  );

  const boundCertificates = useMemo(() => {
    const rows = new Map<
      number,
      { certificate: TlsCertificateItem; domains: ZoneDomainItem[] }
    >();

    for (const domain of domains) {
      if (domain.cert_id == null) {
        continue;
      }
      const certificate = certificateMap.get(domain.cert_id);
      if (!certificate) {
        continue;
      }
      const existing = rows.get(certificate.id);
      if (existing) {
        existing.domains.push(domain);
      } else {
        rows.set(certificate.id, { certificate, domains: [domain] });
      }
    }

    return Array.from(rows.values()).sort((left, right) =>
      left.certificate.name.localeCompare(right.certificate.name),
    );
  }, [certificateMap, domains]);

  const unboundCount = domains.filter(
    (domain) => domain.cert_id == null,
  ).length;

  return (
    <div className='space-y-4'>
      <Card className='shadow-none'>
        <CardHeader className='flex flex-row items-start justify-between gap-3 space-y-0'>
          <div>
            <CardTitle className='text-base'>本 Zone 使用的证书</CardTitle>
            <CardDescription>
              证书在全局证书库管理；此处仅展示已绑定到本 Zone 域名的证书。
            </CardDescription>
          </div>
          <Button variant='outline' size='sm' className='h-7 text-xs' asChild>
            <Link href='/certificates'>
              证书库
              <ExternalLink className='ml-1 size-3.5' />
            </Link>
          </Button>
        </CardHeader>
        <CardContent className='space-y-3'>
          {unboundCount > 0 ? (
            <p className='text-xs text-muted-foreground'>
              还有 {unboundCount} 个域名未绑定证书，可在「域名」Tab 中选择证书。
            </p>
          ) : null}

          {boundCertificates.length === 0 ? (
            <EmptyStateWithBorder
              icon={FileKey}
              description='暂无绑定证书。请先在域名行选择证书，或前往证书库导入/申请。'
            />
          ) : (
            <div className='rounded-lg border'>
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>证书</TableHead>
                    <TableHead>状态</TableHead>
                    <TableHead>覆盖域名</TableHead>
                    <TableHead>到期时间</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {boundCertificates.map(
                    ({ certificate, domains: boundDomains }) => {
                      const status = getCertificateStatus(certificate);
                      return (
                        <TableRow key={certificate.id}>
                          <TableCell className='font-medium'>
                            {certificate.name}
                            <p className='mt-0.5 text-xs text-muted-foreground'>
                              {certificate.primary_domain ||
                                `证书 #${certificate.id}`}
                            </p>
                          </TableCell>
                          <TableCell>
                            <Badge variant='outline' className='text-[10px]'>
                              {status.label}
                            </Badge>
                          </TableCell>
                          <TableCell className='max-w-xs truncate text-muted-foreground'>
                            {boundDomains
                              .map((domain) => domain.domain)
                              .join(' · ')}
                          </TableCell>
                          <TableCell className='text-muted-foreground'>
                            {formatDateTime(certificate.not_after)}
                          </TableCell>
                        </TableRow>
                      );
                    },
                  )}
                </TableBody>
              </Table>
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
