'use client';

import { getFormattingLocale } from '@/i18n/format';
import ReactEChartsCore from 'echarts-for-react/lib/core';
import type { EChartsCoreOption } from 'echarts/core';
import * as echarts from 'echarts/core';
import { ScatterChart } from 'echarts/charts';
import { GeoComponent, TooltipComponent } from 'echarts/components';
import { CanvasRenderer } from 'echarts/renderers';
import { Globe, Map as MapIcon } from 'lucide-react';
import { useTheme } from 'next-themes';
import { useTranslations } from 'next-intl';
import { useRouter } from 'next/navigation';
import { useEffect, useMemo, useRef, useState } from 'react';

import { DottedGlobe } from './dotted-globe';
import { buildCountryHeat, type CountryHeat } from './country-heat';
import { useMapPalette } from './use-map-palette';
import { Button } from '@/components/ui/button';
import { EmptyState } from '@/components/layout/empty';
import type {
  DashboardNodeHealth,
  DistributionItem,
} from '@/lib/services/openflare';

import {
  getNodeStatusLabel,
  getOpenrestyStatusLabel,
} from '../../nodes/components/node-utils';
import countryCentroidsJson from './data/country-centroids.json';
import { ensureWorldMapRegistered } from './world-map-registry';

echarts.use([GeoComponent, TooltipComponent, ScatterChart, CanvasRenderer]);

const fallbackCoordinates = [
  [-122.4194, 37.7749],
  [-46.6333, -23.5505],
  [-0.1276, 51.5072],
  [2.3522, 48.8566],
  [77.209, 28.6139],
  [121.4737, 31.2304],
  [103.8198, 1.3521],
  [151.2093, -33.8688],
  [28.0473, -26.2041],
  [139.6917, 35.6895],
] as const;

type Tone = 'healthy' | 'warning' | 'danger';

type MapNodeDatum = {
  id: number;
  name: string;
  geoName: string;
  route: string;
  derivedFromGeo: boolean;
  requestCount: number;
  errorCount: number;
  activeEventCount: number;
  status: DashboardNodeHealth['status'];
  openrestyStatus: DashboardNodeHealth['openresty_status'];
  value: [number, number, number];
  itemStyle: {
    color: string;
    borderColor: string;
    borderWidth: number;
  };
  emphasis: {
    itemStyle: {
      borderColor: string;
      borderWidth: number;
    };
  };
};

function buildNodeDetailHref(id?: number | null) {
  if (!id) {
    return '/nodes';
  }
  return `/nodes/detail?id=${id}`;
}

function getNodeTone(node: DashboardNodeHealth): Tone {
  if (
    node.status === 'offline' ||
    node.openresty_status === 'unhealthy' ||
    node.active_event_count > 0
  ) {
    return 'danger';
  }

  if (
    node.cpu_usage_percent >= 80 ||
    node.memory_usage_percent >= 85 ||
    node.storage_usage_percent >= 85
  ) {
    return 'warning';
  }

  return 'healthy';
}

const countryCentroids = countryCentroidsJson as Record<string, number[]>;

type CountryCentroid = [number, number];

function toCountryCentroid(
  coords: number[] | undefined,
): CountryCentroid | null {
  if (!coords || coords.length < 2) {
    return null;
  }
  return [coords[0], coords[1]];
}

function resolveCountryCentroid(geoName: string): CountryCentroid | null {
  const trimmed = geoName.trim();
  if (!trimmed) {
    return null;
  }
  const candidates = new Set<string>([trimmed]);
  // ipinfo-style names: "Hong Kong, Hong Kong, HK"
  for (const part of trimmed.split(',').map((item) => item.trim())) {
    if (part) {
      candidates.add(part);
    }
  }
  // trailing ISO2 often present after city/region
  const isoMatch = trimmed.match(/\b([A-Z]{2})\b/g);
  if (isoMatch) {
    for (const code of isoMatch) {
      candidates.add(code);
    }
  }

  for (const candidate of candidates) {
    const direct = toCountryCentroid(countryCentroids[candidate]);
    if (direct) {
      return direct;
    }
    const lower = candidate.toLowerCase();
    for (const [name, coordinates] of Object.entries(countryCentroids)) {
      if (name.toLowerCase() === lower) {
        return toCountryCentroid(coordinates);
      }
    }
  }

  // Soft contains match for longer composite labels (prefer longer country names).
  const lowerFull = trimmed.toLowerCase();
  let best: { length: number; coords: CountryCentroid } | null = null;
  for (const [name, coordinates] of Object.entries(countryCentroids)) {
    const lowerName = name.toLowerCase();
    if (lowerFull.includes(lowerName) && lowerName.length >= 4) {
      const coords = toCountryCentroid(coordinates);
      if (!coords) {
        continue;
      }
      if (!best || lowerName.length > best.length) {
        best = { length: lowerName.length, coords };
      }
    }
  }
  return best?.coords ?? null;
}

function getNodeCoordinates(node: DashboardNodeHealth, index: number) {
  if (
    typeof node.geo_latitude === 'number' &&
    typeof node.geo_longitude === 'number'
  ) {
    return {
      coordinates: [node.geo_longitude, node.geo_latitude] as [number, number],
      derivedFromGeo: true,
    };
  }

  const countryCentroid = resolveCountryCentroid(node.geo_name || '');
  if (countryCentroid) {
    return {
      coordinates: countryCentroid,
      derivedFromGeo: true,
    };
  }

  return {
    coordinates: [
      ...fallbackCoordinates[index % fallbackCoordinates.length],
    ] as [number, number],
    derivedFromGeo: false,
  };
}

function FlatWorldMap({
  nodes,
  countryHeat,
}: {
  nodes: DashboardNodeHealth[];
  countryHeat: Map<string, CountryHeat>;
}) {
  const t = useTranslations('dashboard.map');
  const tn = useTranslations('nodes');
  const { resolvedTheme } = useTheme();
  const isDark = resolvedTheme === 'dark';
  const themePalette = useMapPalette();
  const router = useRouter();
  const chartContainerRef = useRef<HTMLDivElement | null>(null);
  const [mapReady, setMapReady] = useState(false);
  const [mapFailed, setMapFailed] = useState(false);
  const [containerSize, setContainerSize] = useState({ width: 0, height: 0 });

  useEffect(() => {
    const ready = ensureWorldMapRegistered();
    setMapReady(ready);
    setMapFailed(!ready);
  }, []);

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
            labelColor: themePalette?.label ?? '#e2e8f0',
            healthyColor: '#34d399',
            warningColor: '#fbbf24',
            dangerColor: '#f87171',
            healthyBorder: '#6ee7b7',
            warningBorder: '#fcd34d',
            dangerBorder: '#fca5a5',
          }
        : {
            areaColor: themePalette?.surface ?? '#e2e8f0',
            borderColor: themePalette?.border ?? 'rgba(100,116,139,0.45)',
            labelColor: themePalette?.label ?? '#18181b',
            healthyColor: '#10b981',
            warningColor: '#f59e0b',
            dangerColor: '#ef4444',
            healthyBorder: '#6ee7b7',
            warningBorder: '#fcd34d',
            dangerBorder: '#fca5a5',
          },
    [isDark, themePalette],
  );

  const mapNodes = useMemo<MapNodeDatum[]>(
    () =>
      nodes.map((node, index) => {
        const { coordinates, derivedFromGeo } = getNodeCoordinates(node, index);
        const tone = getNodeTone(node);
        const toneColor =
          tone === 'healthy'
            ? mapPalette.healthyColor
            : tone === 'warning'
              ? mapPalette.warningColor
              : mapPalette.dangerColor;
        const toneBorder =
          tone === 'healthy'
            ? mapPalette.healthyBorder
            : tone === 'warning'
              ? mapPalette.warningBorder
              : mapPalette.dangerBorder;

        return {
          id: node.id,
          name: node.name,
          geoName: node.geo_name || node.name,
          route: buildNodeDetailHref(node.id),
          derivedFromGeo,
          requestCount: node.request_count,
          errorCount: node.error_count,
          activeEventCount: node.active_event_count,
          status: node.status,
          openrestyStatus: node.openresty_status,
          value: [
            coordinates[0],
            coordinates[1],
            Math.max(node.request_count, 1),
          ],
          itemStyle: {
            color: toneColor,
            borderColor: toneBorder,
            borderWidth: 1.5,
          },
          emphasis: {
            itemStyle: {
              borderColor: toneBorder,
              borderWidth: 2,
            },
          },
        };
      }),
    [mapPalette, nodes],
  );

  const computedLayoutSize = Math.min(
    containerSize.width * 0.94,
    containerSize.height * 1.8,
  );

  const mapOption = useMemo<EChartsCoreOption>(() => {
    const sourceColor = themePalette?.source ?? '#3b82f6';
    const sourceTooltip = (name: string, value: number) =>
      `<strong>${name}</strong><br />${t('sourceRequests', {
        count: value.toLocaleString(getFormattingLocale()),
      })}`;
    const countryRegions = [...countryHeat.values()]
      .filter((item) => item.hasBoundary)
      .map(({ name, value, intensity }) => {
        const opacity = Math.round((0.24 + intensity * 0.72) * 255)
          .toString(16)
          .padStart(2, '0');
        return {
          name,
          itemStyle: { areaColor: `${sourceColor}${opacity}` },
          tooltip: { show: true, formatter: () => sourceTooltip(name, value) },
        };
      });
    const sourceMarkers = [...countryHeat.values()].flatMap((item) =>
      !item.hasBoundary && item.coordinate
        ? [
            {
              name: item.name,
              value: item.coordinate,
              symbolSize: 7 + item.intensity * 9,
              tooltip: {
                formatter: () => sourceTooltip(item.name, item.value),
              },
            },
          ]
        : [],
    );
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
        formatter: (params: unknown) => {
          const { data } = params as { data?: MapNodeDatum };
          if (!data || !('requestCount' in data)) {
            return '';
          }

          const locationLine = data.derivedFromGeo
            ? data.geoName
            : `${data.geoName} · ${t('presetPin')}`;

          return [
            `<div style="font-weight:600;margin-bottom:6px;">${data.name}</div>`,
            `<div>${locationLine}</div>`,
            `<div>${t('requestsErrors', {
              requests: data.requestCount.toLocaleString(getFormattingLocale()),
              errors: data.errorCount.toLocaleString(getFormattingLocale()),
            })}</div>`,
            `<div>${t('eventsStatus', {
              events: data.activeEventCount,
              status: getNodeStatusLabel(data.status, tn),
            })}</div>`,
            `<div>${t('openrestyStatus', {
              status: getOpenrestyStatusLabel(data.openrestyStatus, tn),
            })}</div>`,
          ].join('');
        },
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
          data: sourceMarkers,
          z: 2,
          tooltip: { show: true },
          itemStyle: {
            color: sourceColor,
            borderColor: sourceColor,
            borderWidth: 1.5,
            opacity: 0.85,
          },
        },
        {
          type: 'scatter',
          coordinateSystem: 'geo',
          data: mapNodes,
          z: 3,
          tooltip: { show: true },
          progressive: 64,
          large: true,
          largeThreshold: 24,
          symbolSize: (value: unknown) => {
            const size =
              Array.isArray(value) && typeof value[2] === 'number'
                ? value[2]
                : 1;
            const responsiveBase = 8 + Math.log10(size + 1) * 3.6;
            return Math.max(7, Math.min(18, responsiveBase));
          },
          label: {
            show: false,
            color: mapPalette.labelColor,
            fontSize: 11,
          },
          emphasis: {
            scale: 1.12,
            label: {
              show: mapNodes.length > 0 && mapNodes.length <= 5,
              position: 'right',
              distance: 8,
              formatter: '{b}',
              backgroundColor: isDark
                ? 'rgba(24,24,27,0.88)'
                : 'rgba(255,255,255,0.94)',
              borderColor: isDark
                ? 'rgba(63,63,70,0.7)'
                : 'rgba(228,228,231,0.9)',
              borderWidth: 1,
              borderRadius: 999,
              padding: [4, 8],
            },
          },
        },
      ],
    };
  }, [
    computedLayoutSize,
    countryHeat,
    isDark,
    mapNodes,
    mapPalette,
    t,
    themePalette,
    tn,
  ]);

  if (!mapReady) {
    return (
      <div className='flex h-full items-center justify-center'>
        <EmptyState
          title={mapFailed ? t('loadFailed') : t('loading')}
          description={mapFailed ? t('loadFailedDesc') : t('loadingDesc')}
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
          onEvents={{
            click: (params: { data?: MapNodeDatum }) => {
              if (params.data?.route) {
                router.push(params.data.route);
              }
            },
          }}
          style={{
            height: containerSize.height,
            width: containerSize.width,
          }}
        />
      ) : (
        <div className='flex h-full items-center justify-center'>
          <EmptyState
            title={t('loading')}
            description={t('measuring')}
            iconSize='sm'
          />
        </div>
      )}
    </div>
  );
}

export function WorldStageMap(props: {
  nodes: DashboardNodeHealth[];
  sourceCountries: DistributionItem[];
}) {
  const t = useTranslations('dashboard.map');
  const tn = useTranslations('nodes');
  const [mode, setMode] = useState('3d');
  const countryHeat = useMemo(
    () => buildCountryHeat(props.sourceCountries),
    [props.sourceCountries],
  );
  return (
    <div className='flex h-full min-h-0 flex-col gap-3'>
      <div className='flex items-center justify-between gap-2'>
        <span className='text-sm font-medium'>{t('geolocation')}</span>
        <Button
          type='button'
          variant='ghost'
          size='icon-sm'
          className='size-7 shrink-0 text-muted-foreground hover:bg-transparent hover:text-foreground'
          onClick={() => setMode((prev) => (prev === '3d' ? '2d' : '3d'))}
          aria-label={t(mode === '3d' ? 'switchTo2d' : 'switchTo3d')}
          title={t(mode === '3d' ? 'switchTo2d' : 'switchTo3d')}
        >
          {mode === '3d' ? (
            <MapIcon className='size-4' />
          ) : (
            <Globe className='size-4' />
          )}
        </Button>
      </div>
      <div className='relative min-h-0 flex-1'>
        {mode === '3d' ? (
          <DottedGlobe
            nodes={props.nodes.map((node, index) => {
              const { coordinates, derivedFromGeo } = getNodeCoordinates(
                node,
                index,
              );
              return {
                id: node.id,
                name: node.name,
                coordinates,
                tone: getNodeTone(node),
                detail: [
                  node.geo_name || node.name,
                  ...(derivedFromGeo ? [] : [t('presetPin')]),
                  t('requestsErrors', {
                    requests: node.request_count.toLocaleString(
                      getFormattingLocale(),
                    ),
                    errors: node.error_count.toLocaleString(
                      getFormattingLocale(),
                    ),
                  }),
                  t('eventsStatus', {
                    events: node.active_event_count,
                    status: getNodeStatusLabel(node.status, tn),
                  }),
                  t('openrestyStatus', {
                    status: getOpenrestyStatusLabel(node.openresty_status, tn),
                  }),
                ].join(' · '),
              };
            })}
            countryHeat={countryHeat}
          />
        ) : (
          <FlatWorldMap nodes={props.nodes} countryHeat={countryHeat} />
        )}
      </div>
    </div>
  );
}
