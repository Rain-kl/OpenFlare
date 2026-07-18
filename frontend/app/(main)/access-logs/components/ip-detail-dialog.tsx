'use client';

import { useState } from 'react';

import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';

import { IpAnalysisPanel } from './ip-analysis-panel';

export function IpDetailDialog({
  open,
  remoteAddr,
  region,
  initialHours,
  onOpenChange,
}: {
  open: boolean;
  remoteAddr: string | null;
  region?: string;
  initialHours?: number;
  onOpenChange: (open: boolean) => void;
}) {
  const [displayAddr, setDisplayAddr] = useState<string | null>(remoteAddr);
  const [displayRegion, setDisplayRegion] = useState(region);
  if (remoteAddr && remoteAddr !== displayAddr) {
    setDisplayAddr(remoteAddr);
    setDisplayRegion(region);
  }
  const ip = remoteAddr ?? displayAddr ?? '';

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className='max-h-[90vh] sm:max-w-6xl md:max-w-6xl overflow-y-auto hide-scrollbar'>
        <DialogHeader>
          <DialogTitle>IP 详情</DialogTitle>
          <DialogDescription>
            <span className='font-mono text-foreground'>{ip || '—'}</span>
            {displayRegion ? (
              <span className='text-muted-foreground'> · {displayRegion}</span>
            ) : null}
            。查看该 IP 的访问趋势、分布与 WAF IP 组操作。
          </DialogDescription>
        </DialogHeader>
        <IpAnalysisPanel
          key={ip}
          ip={ip}
          enabled={open && ip !== ''}
          initialHours={initialHours}
        />
      </DialogContent>
    </Dialog>
  );
}
