'use client';

import { ScrollText } from 'lucide-react';
import { useEffect, useRef, useState } from 'react';
import { useTranslations } from 'next-intl';

import { EmptyState } from '@/components/layout/empty';
import { Alert, AlertDescription } from '@/components/ui/alert';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card';
import { NodeService } from '@/lib/services/openflare';

const maxVisibleLines = 500;

type ConnectionStatus =
  | 'idle'
  | 'connecting'
  | 'connected'
  | 'disconnected'
  | 'failed';

type LogBatch = {
  lines?: string[];
  dropped?: number;
};

export function NodeRuntimeLogs({ nodeId }: { nodeId: number }) {
  const t = useTranslations('nodes.runtimeLogs');
  const [status, setStatus] = useState<ConnectionStatus>('idle');
  const [lines, setLines] = useState<string[]>([]);
  const [dropped, setDropped] = useState(0);
  const stream = useRef<EventSource | null>(null);
  const viewport = useRef<HTMLPreElement | null>(null);
  const follow = useRef(true);

  useEffect(() => {
    return () => {
      stream.current?.close();
      stream.current = null;
    };
  }, []);

  useEffect(() => {
    if (follow.current && viewport.current) {
      viewport.current.scrollTop = viewport.current.scrollHeight;
    }
  }, [lines]);

  function connect() {
    if (stream.current) return;
    setStatus('connecting');
    setLines([]);
    setDropped(0);
    follow.current = true;

    let source: EventSource;
    try {
      source = NodeService.openRuntimeLogs(nodeId);
    } catch {
      setStatus('failed');
      return;
    }
    stream.current = source;
    let ready = false;

    const close = (next: ConnectionStatus) => {
      if (stream.current !== source) return;
      stream.current = null;
      source.close();
      setStatus(next);
    };

    source.addEventListener('ready', () => {
      if (stream.current !== source) return;
      ready = true;
      setStatus('connected');
    });
    source.addEventListener('logs', (event) => {
      if (stream.current !== source) return;
      try {
        const batch = JSON.parse(
          (event as MessageEvent<string>).data,
        ) as LogBatch;
        if (batch.lines?.length) {
          const incoming = batch.lines;
          setLines((previous) =>
            [...previous, ...incoming].slice(-maxVisibleLines),
          );
        }
        if (batch.dropped) {
          const count = batch.dropped;
          setDropped((previous) => previous + count);
        }
      } catch {
        close('failed');
      }
    });
    source.addEventListener('closed', () => close('disconnected'));
    source.onerror = () => close(ready ? 'disconnected' : 'failed');
  }

  function disconnect() {
    stream.current?.close();
    stream.current = null;
    setStatus('disconnected');
  }

  const active = status === 'connecting' || status === 'connected';

  return (
    <Card>
      <CardHeader>
        <div className='flex flex-wrap items-start justify-between gap-3'>
          <div className='flex flex-col gap-1'>
            <CardTitle>{t('title')}</CardTitle>
            <CardDescription>{t('description')}</CardDescription>
          </div>
          <div className='flex items-center gap-2'>
            <Badge variant='secondary' role='status'>
              {t(status)}
            </Badge>
            <Button
              type='button'
              size='sm'
              variant={active ? 'outline' : 'default'}
              onClick={active ? disconnect : connect}
            >
              {active ? t('disconnect') : t('connect')}
            </Button>
          </div>
        </div>
      </CardHeader>
      <CardContent className='flex min-w-0 flex-col gap-3'>
        {status === 'failed' && (
          <Alert variant='destructive'>
            <AlertDescription>{t('connectionFailed')}</AlertDescription>
          </Alert>
        )}
        {dropped > 0 && (
          <p className='text-xs text-muted-foreground'>
            {t('dropped', { count: dropped })}
          </p>
        )}
        {lines.length > 0 ? (
          <pre
            ref={viewport}
            aria-label={t('title')}
            tabIndex={0}
            className='h-[min(60vh,36rem)] overflow-auto rounded-lg border border-dashed bg-muted/30 p-3 font-mono text-xs leading-relaxed whitespace-pre-wrap wrap-anywhere'
            onScroll={(event) => {
              const element = event.currentTarget;
              follow.current =
                element.scrollHeight -
                  element.scrollTop -
                  element.clientHeight <
                24;
            }}
          >
            {lines.join('\n')}
          </pre>
        ) : (
          <EmptyState
            icon={ScrollText}
            description={t(active ? 'waiting' : 'empty')}
          />
        )}
      </CardContent>
    </Card>
  );
}
