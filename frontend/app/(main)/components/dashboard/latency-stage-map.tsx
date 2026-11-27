'use client';

import ReactEChartsCore from 'echarts-for-react/lib/core';
import type { EChartsCoreOption } from 'echarts/core';
import * as echarts from 'echarts/core';
import { ScatterChart } from 'echarts/charts';
import { GeoComponent, TooltipComponent } from 'echarts/components';
import { CanvasRenderer } from 'echarts/renderers';
import { useTheme } from 'next-themes';
import { useTranslations } from 'next-intl';
import { useEffect, useMemo, useRef, useState } from 'react';

import { EmptyState } from '@/components/layout/empty';
import { getFormattingLocale } from '@/i18n/format';

import type { CountryLatencyDatum } from './country-heat';
import { useMapPalette } from './use-map-palette';
import { ensureWorldMapRegistered } from './world-map-registry';

echarts.use([GeoComponent, TooltipComponent, ScatterChart, CanvasRenderer]);

export type LatencyTier = 'fast' | 'moderate' | 'slow' | 'critical';

export function getLatencyTier(ms: number): LatencyTier {
  if (ms < 100) return 'fast';
  if (ms < 300) return 'moderate';
  if (ms < 800) return 'slow';
  return 'critical';
}

export function formatLatencyMs(ms: number): string {
  if (!Number.isFinite(ms) || ms < 0) {
    return '—';
  }
  if (ms < 1000) {
    return `${ms.toFixed(1)} ms`;
  }
  return `${(ms / 1000).toFixed(2)} s`;
}

function escapeHtml(text: string): string {
  return text
    .replaceAll('&', '&amp;')
    .replaceAll('<', '&lt;')
    .replaceAll('>', '&gt;')
    .replaceAll('"', '&quot;')
    .replaceAll("'", '&#39;');
}

const TIER_COLORS: Record<
  'light' | 'dark',
  Record<LatencyTier, { area: string; dot: string }>
> = {
  light: {
    fast: { area: 'rgba(16, 185, 129, 0.52)', dot: '#10b981' },
    moderate: { area: 'rgba(245, 158, 11, 0.55)', dot: '#f59e0b' },
    slow: { area: 'rgba(249, 115, 22, 0.62)', dot: '#f97316' },
    critical: { area: 'rgba(239, 68, 68, 0.66)', dot: '#ef4444' },
  },
  dark: {
    fast: { area: 'rgba(52, 211, 153, 0.56)', dot: '#34d399' },
    moderate: { area: 'rgba(251, 191, 36, 0.58)', dot: '#fbbf24' },
    slow: { area: 'rgba(251, 146, 60, 0.64)', dot: '#fb923c' },
    critical: { area: 'rgba(248, 113, 113, 0.68)', dot: '#f87171' },
  },
};

export function LatencyStageMap({
  countryLatency,
}: {
  countryLatency: Map<string, CountryLatencyDatum>;
}) {
  const tm = useTranslations('dashboard.map');
  const tl = useTranslations('dashboard.latency');
  const { resolvedTheme } = useTheme();
  const isDark = resolvedTheme === 'dark';
  const themePalette = useMapPalette();
  const chartContainerRef = useRef<HTMLDivElement | null>(null);
  const [mapReady] = useState(() => ensureWorldMapRegistered());
  const [containerSize, setContainerSize] = useState({ width: 0, height: 0 });

  useEffect(() => {
    if (!mapReady) {
      return;
    }

    const container = chartContainerRef.current;
    if (!container) {
      return;
    }

    const updateSize = () => {
      const nextWidth = container.clientWidth;
      const nextHeight = container.clientHeight;
      if (nextWidth <= 0 || nextHeight <= 0) {
        return;
      }
      setContainerSize((previous) =>
        previous.width === nextWidth && previous.height === nextHeight
          ? previous
          : { width: nextWidth, height: nextHeight },
      );
    };

    updateSize();
    const frameId = window.requestAnimationFrame(updateSize);

    if (typeof ResizeObserver === 'undefined') {
      window.addEventListener('resize', updateSize);
      return () => {
        window.cancelAnimationFrame(frameId);
        window.removeEventListener('resize', updateSize);
      };
    }

    const observer = new ResizeObserver(() => {
      updateSize();
    });
    observer.observe(container);

    return () => {
      window.cancelAnimationFrame(frameId);
      observer.disconnect();
    };
  }, [mapReady]);

  const mapPalette = useMemo(
    () =>
      isDark
        ? {
            areaColor: themePalette?.surface ?? '#1e293b',
            borderColor: themePalette?.border ?? 'rgba(148,163,184,0.32)',
          }
        : {
            areaColor: themePalette?.surface ?? '#e2e8f0',
            borderColor: themePalette?.border ?? 'rgba(100,116,139,0.45)',
          },
    [isDark, themePalette],
  );

  const computedLayoutSize = Math.min(
    containerSize.width * 0.94,
    containerSize.height * 1.8,
  );

  const mapOption = useMemo<EChartsCoreOption>(() => {
    const scheme = isDark ? TIER_COLORS.dark : TIER_COLORS.light;
    const locale = getFormattingLocale();

    const buildTooltipHtml = (item: CountryLatencyDatum) => {
      const lines = [
        `<div style="font-weight:600;margin-bottom:4px;">${escapeHtml(item.name)}</div>`,
        `<div>${escapeHtml(
          tl('avgLatencyLabel', {
            value: formatLatencyMs(item.avgRequestTimeMs),
          }),
        )}</div>`,
      ];
      if (item.avgTcpRttMs > 0) {
        lines.push(
          `<div>${escapeHtml(
            tl('avgTcpRttLabel', { value: formatLatencyMs(item.avgTcpRttMs) }),
          )}</div>`,
        );
      }
      lines.push(
        `<div>${escapeHtml(
          tl('requestsCount', {
            count: item.requestCount.toLocaleString(locale),
          }),
        )}</div>`,
      );
      return lines.join('');
    };

    const countryRegions = [...countryLatency.values()]
      .filter((item) => item.hasBoundary)
      .map((item) => {
        const tier = getLatencyTier(item.avgRequestTimeMs);
        return {
          name: item.name,
          itemStyle: { areaColor: scheme[tier].area },
          tooltip: { show: true, formatter: () => buildTooltipHtml(item) },
        };
      });

    const markers = [...countryLatency.values()].flatMap((item) => {
      if (item.hasBoundary || !item.coordinate) {
        return [];
      }
      const tier = getLatencyTier(item.avgRequestTimeMs);
      return [
        {
          name: item.name,
          value: item.coordinate,
          symbolSize: Math.max(
            8,
            Math.min(16, 8 + Math.log10(item.requestCount + 1) * 2.5),
          ),
          itemStyle: {
            color: scheme[tier].dot,
            borderColor: scheme[tier].dot,
            borderWidth: 1.5,
            opacity: 0.9,
          },
          tooltip: {
            show: true,
            formatter: () => buildTooltipHtml(item),
          },
        },
      ];
    });

    return {
      animation: false,
      backgroundColor: 'transparent',
      tooltip: {
        trigger: 'item',
        transitionDuration: 0,
        backgroundColor: isDark
          ? 'rgba(24,24,27,0.96)'
          : 'rgba(255,255,255,0.98)',
        borderColor: isDark ? 'rgba(63,63,70,0.8)' : 'rgba(228,228,231,0.9)',
        borderWidth: 1,
        textStyle: {
          color: isDark ? '#fafafa' : '#18181b',
          fontSize: 12,
        },
        formatter: () => '',
      },
      geo: {
        map: 'world',
        roam: true,
        tooltip: { show: false },
        layoutCenter: ['50%', '50%'],
        layoutSize: computedLayoutSize,
        zoom: 1,
        scaleLimit: {
          min: 0.8,
          max: 2.5,
        },
        regions: countryRegions,
        itemStyle: {
          areaColor: mapPalette.areaColor,
          borderColor: mapPalette.borderColor,
          borderWidth: 0.7,
        },
        emphasis: {
          disabled: true,
        },
      },
      series: [
        {
          type: 'scatter',
          coordinateSystem: 'geo',
          data: markers,
          z: 2,
          tooltip: { show: true },
        },
      ],
    };
  }, [computedLayoutSize, countryLatency, isDark, mapPalette, tl]);

  if (!mapReady) {
    return (
      <div className='flex h-full items-center justify-center'>
        <EmptyState
          title={tm('loadFailed')}
          description={tm('loadFailedDesc')}
        />
      </div>
    );
  }

  const chartReady = containerSize.width > 0 && containerSize.height > 0;

  return (
    <div ref={chartContainerRef} className='h-full min-h-0 w-full'>
      {chartReady ? (
        <ReactEChartsCore
          echarts={echarts}
          option={mapOption}
          notMerge
          lazyUpdate
          opts={{ renderer: 'canvas' }}
          style={{
            height: containerSize.height,
            width: containerSize.width,
          }}
        />
      ) : (
        <div className='flex h-full items-center justify-center'>
          <EmptyState
            title={tm('loading')}
            description={tm('measuring')}
            iconSize='sm'
          />
        </div>
      )}
    </div>
  );
}
