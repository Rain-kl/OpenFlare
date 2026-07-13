/*
Copyright 2026 Arctel.net

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

'use client';

import { memo, useCallback, useEffect, useRef, useState } from 'react';
import { useVirtualizer } from '@tanstack/react-virtual';
import { toast } from 'sonner';
import { ArrowDown, ChevronUp, Loader2 } from 'lucide-react';

import services from '@/lib/services';
import { ErrorInline } from '@/components/layout/error';
import { LoadingStateWithBorder } from '@/components/layout/loading';
import { Button } from '@/components/ui/button';

interface LogEntry {
  index: number;
  data: string;
}

// Distance (px) from bottom to treat as "at bottom"
const BOTTOM_THRESHOLD = 40;
const LOG_LINE_ESTIMATE_PX = 20;
const LOG_VIRTUAL_OVERSCAN = 24;

const LogLine = memo(function LogLine({ data }: { data: string }) {
  const level = parseLogLevel(data);
  const color =
    level === 'error'
      ? 'text-red-400'
      : level === 'warn'
        ? 'text-yellow-400'
        : level === 'debug'
          ? 'text-gray-500'
          : 'text-gray-300';

  return (
    <div
      className={`${color} whitespace-pre-wrap break-all px-2 py-0.5 rounded hover:bg-white/5`}
    >
      {data}
    </div>
  );
});

function getApiBaseUrl(): string {
  if (typeof window !== 'undefined') {
    // If NEXT_PUBLIC_WAVELET_BACKEND_URL is set, use it. Otherwise, use origin.
    const base = process.env.NEXT_PUBLIC_WAVELET_BACKEND_URL || '';
    if (base.startsWith('http')) return base;

    // Relative URL fallback
    const proto = window.location.protocol;
    const host = window.location.host;
    return `${proto}//${host}${base}`;
  }
  return process.env.NEXT_PUBLIC_WAVELET_BACKEND_URL || '';
}

function buildWsUrl(): string {
  const base = getApiBaseUrl();
  const wsBase = base.replace(/^http/, 'ws');
  return `${wsBase}/api/v1/admin/logs/ws`;
}

function parseLogLevel(
  line: string,
): 'debug' | 'info' | 'warn' | 'error' | 'unknown' {
  const lower = line.toLowerCase();
  if (lower.includes('"level":"error"') || lower.includes('level=error'))
    return 'error';
  if (lower.includes('"level":"warn"') || lower.includes('level=warn'))
    return 'warn';
  if (lower.includes('"level":"debug"') || lower.includes('level=debug'))
    return 'debug';
  if (lower.includes('"level":"info"') || lower.includes('level=info'))
    return 'info';
  return 'unknown';
}

export function AppLogs() {
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<Error | null>(null);

  const [logs, setLogs] = useState<LogEntry[]>([]);
  const [hasMore, setHasMore] = useState(false);
  const [nextCursor, setNextCursor] = useState(0);
  const [loadingMore, setLoadingMore] = useState(false);

  // autoScroll = true → new logs auto-scroll to bottom
  const [autoScroll, setAutoScroll] = useState(true);

  const containerRef = useRef<HTMLDivElement>(null);
  const wsRef = useRef<WebSocket | null>(null);
  const autoScrollRef = useRef(autoScroll);
  const isUserScrolling = useRef(false);

  useEffect(() => {
    autoScrollRef.current = autoScroll;
  }, [autoScroll]);

  const rowVirtualizer = useVirtualizer({
    count: logs.length,
    getScrollElement: () => containerRef.current,
    estimateSize: () => LOG_LINE_ESTIMATE_PX,
    overscan: LOG_VIRTUAL_OVERSCAN,
    measureElement: (element) => element.getBoundingClientRect().height,
  });

  // ---- Scroll detection ------------------------------------------------
  const handleScroll = useCallback(() => {
    const el = containerRef.current;
    if (!el) return;

    if (!isUserScrolling.current) return;

    const atBottom =
      el.scrollHeight - el.scrollTop - el.clientHeight < BOTTOM_THRESHOLD;
    if (atBottom && !autoScrollRef.current) {
      setAutoScroll(true);
    } else if (!atBottom && autoScrollRef.current) {
      setAutoScroll(false);
    }
  }, []);

  const handleWheel = useCallback(() => {
    isUserScrolling.current = true;
  }, []);
  const handleTouchStart = useCallback(() => {
    isUserScrolling.current = true;
  }, []);

  useEffect(() => {
    const el = containerRef.current;
    if (!el) return;

    let timer: ReturnType<typeof setTimeout>;
    const onScrollEnd = () => {
      clearTimeout(timer);
      timer = setTimeout(() => {
        isUserScrolling.current = false;
      }, 150);
    };
    el.addEventListener('scroll', onScrollEnd, { passive: true });
    return () => {
      el.removeEventListener('scroll', onScrollEnd);
      clearTimeout(timer);
    };
  }, []);

  // ---- Auto-scroll to bottom when new logs arrive ----------------------
  useEffect(() => {
    if (!autoScroll || !containerRef.current) return;
    isUserScrolling.current = false;
    const el = containerRef.current;
    requestAnimationFrame(() => {
      el.scrollTop = el.scrollHeight;
    });
  }, [logs, autoScroll]);

  // ---- Data fetching ---------------------------------------------------
  const fetchLogs = useCallback(async (cursor: number = 0) => {
    try {
      return await services.adminLog.getLogs(cursor);
    } catch (err) {
      throw err instanceof Error ? err : new Error('获取日志失败');
    }
  }, []);

  const loadHistory = useCallback(
    async (cursor: number = 0) => {
      const isInitial = cursor === 0;
      if (isInitial) {
        setLoading(true);
        setError(null);
      } else {
        setLoadingMore(true);
      }

      try {
        const data = await fetchLogs(cursor);
        if (isInitial) {
          setLogs(data.lines || []);
        } else {
          const el = containerRef.current;
          const previousScrollHeight = el?.scrollHeight ?? 0;
          setLogs((prev) => [...(data.lines || []), ...prev]);

          requestAnimationFrame(() => {
            if (!el) return;
            isUserScrolling.current = false;
            el.scrollTop = el.scrollHeight - previousScrollHeight;
          });
        }
        setHasMore(data.has_more);
        setNextCursor(data.next_cursor);
      } catch (err) {
        if (isInitial) {
          setError(err instanceof Error ? err : new Error('获取日志失败'));
        } else {
          toast.error('加载更早日志失败');
        }
      } finally {
        if (isInitial) setLoading(false);
        else setLoadingMore(false);
      }
    },
    [fetchLogs],
  );

  // ---- WebSocket -------------------------------------------------------
  const connectWs = useCallback(() => {
    if (wsRef.current) wsRef.current.close();

    const ws = new WebSocket(buildWsUrl());
    wsRef.current = ws;

    ws.onmessage = (event) => {
      try {
        const msg = JSON.parse(event.data);
        if (msg.type === 'log' && msg.data) {
          const entry: LogEntry = msg.data;
          setLogs((prev) => {
            const next = [...prev, entry];
            return next.length > 2000 ? next.slice(-2000) : next;
          });
        }
      } catch {
        /* ignore */
      }
    };

    ws.onclose = () => {
      wsRef.current = null;
    };
  }, []);

  // ---- Initialize ------------------------------------------------------
  useEffect(() => {
    loadHistory(0).then(() => connectWs());
    return () => {
      wsRef.current?.close();
      wsRef.current = null;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // ---- Actions ---------------------------------------------------------
  const scrollToBottom = useCallback(() => {
    setAutoScroll(true);
    requestAnimationFrame(() => {
      if (containerRef.current) {
        containerRef.current.scrollTop = containerRef.current.scrollHeight;
      }
    });
  }, []);

  const handleLoadMore = useCallback(() => {
    if (nextCursor > 0) loadHistory(nextCursor);
  }, [nextCursor, loadHistory]);

  // ---- Render ----------------------------------------------------------
  if (loading) return <LoadingStateWithBorder />;
  if (error)
    return <ErrorInline error={error} onRetry={() => loadHistory(0)} />;

  return (
    <div className='flex flex-col h-full relative'>
      {/* Log viewer — fixed height, scrollable */}
      <div
        ref={containerRef}
        onScroll={handleScroll}
        onWheel={handleWheel}
        onTouchStart={handleTouchStart}
        className='h-[calc(100vh-270px)] overflow-y-auto overflow-x-hidden rounded-lg border border-border/40 bg-[#0d1117] font-mono text-[13px] leading-5 relative'
      >
        {/* Load older logs */}
        {hasMore && (
          <div className='sticky top-0 z-10 flex justify-center py-1.5 bg-[#0d1117]/90 backdrop-blur-sm'>
            <Button
              variant='ghost'
              size='sm'
              onClick={handleLoadMore}
              disabled={loadingMore}
              className='text-muted-foreground hover:text-foreground h-7 text-xs'
            >
              {loadingMore ? (
                <>
                  <Loader2 className='size-3 mr-1.5 animate-spin' />
                  加载中...
                </>
              ) : (
                <>
                  <ChevronUp className='size-3 mr-1.5' />
                  加载更早日志
                </>
              )}
            </Button>
          </div>
        )}

        {logs.length === 0 ? (
          <div className='px-4 py-8 text-center text-gray-500'>暂无日志</div>
        ) : (
          <div
            className='relative px-3 py-3'
            style={{ height: `${rowVirtualizer.getTotalSize()}px` }}
          >
            {rowVirtualizer.getVirtualItems().map((virtualRow) => {
              const entry = logs[virtualRow.index];
              if (!entry) return null;

              return (
                <div
                  key={entry.index}
                  ref={rowVirtualizer.measureElement}
                  data-index={virtualRow.index}
                  className='absolute top-0 left-0 w-full'
                  style={{ transform: `translateY(${virtualRow.start}px)` }}
                >
                  <LogLine data={entry.data} />
                </div>
              );
            })}
          </div>
        )}
      </div>

      {/* Floating "back to latest" button */}
      {!autoScroll && (
        <div className='absolute bottom-6 left-1/2 -translate-x-1/2 z-20'>
          <Button
            variant='outline'
            size='sm'
            onClick={scrollToBottom}
            className='shadow-lg bg-background/80 backdrop-blur-sm border-border/50'
          >
            <ArrowDown className='size-3.5 mr-1.5' />
            回到最新
          </Button>
        </div>
      )}
    </div>
  );
}
