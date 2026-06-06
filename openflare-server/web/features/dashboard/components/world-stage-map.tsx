'use client';

import ReactEChartsCore from 'echarts-for-react/lib/core';
import type { EChartsCoreOption } from 'echarts/core';
import * as echarts from 'echarts/core';
import { MapChart, ScatterChart } from 'echarts/charts';
import { GeoComponent, TooltipComponent } from 'echarts/components';
import { CanvasRenderer } from 'echarts/renderers';
import { useRouter } from 'next/navigation';
import { useEffect, useMemo, useRef, useState } from 'react';

import { EmptyState } from '@/components/feedback/empty-state';
import worldGeoJson from '@/features/dashboard/data/world-geo.json';
import type {
  DashboardNodeHealth,
  DistributionItem,
} from '@/features/dashboard/types';
import {
  getNodeStatusLabel,
  getOpenrestyStatusLabel,
} from '@/features/nodes/utils';

echarts.use([
  GeoComponent,
  TooltipComponent,
  MapChart,
  ScatterChart,
  CanvasRenderer,
]);

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

type WorldGeoJson = {
  type: string;
  features?: WorldFeature[];
};

type WorldFeature = {
  properties?: {
    name?: string;
  };
  geometry?: {
    type?: 'Polygon' | 'MultiPolygon';
    coordinates?: number[][][] | number[][][][];
  };
};

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

type CountryRegionDatum = {
  name: string;
  value: number;
  itemStyle: {
    areaColor: string;
    borderColor: string;
  };
  emphasis: {
    itemStyle: {
      areaColor: string;
      borderColor: string;
    };
  };
};

let worldMapRegistrationAttempted = false;
let worldMapRegistrationSucceeded = false;

const baseWorldMapLayoutSizePercent = 168;
const baseWorldMapZoom = 1;

function buildNodeDetailHref(id?: number | null) {
  if (!id) {
    return '/node';
  }
  return `/node/detail?id=${id}`;
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

  return {
    coordinates: [
      ...fallbackCoordinates[index % fallbackCoordinates.length],
    ] as [number, number],
    derivedFromGeo: false,
  };
}

function ensureWorldMapRegistered() {
  if (worldMapRegistrationSucceeded || echarts.getMap('world')) {
    worldMapRegistrationSucceeded = true;
    return true;
  }

  if (worldMapRegistrationAttempted) {
    return false;
  }

  worldMapRegistrationAttempted = true;

  try {
    const geoJson = worldGeoJson as WorldGeoJson;

    if (
      geoJson.type !== 'FeatureCollection' ||
      !Array.isArray(geoJson.features)
    ) {
      throw new Error('invalid world geojson payload');
    }

    echarts.registerMap(
      'world',
      geoJson as unknown as Parameters<typeof echarts.registerMap>[1],
    );

    if (!echarts.getMap('world')) {
      throw new Error('world map registration failed');
    }

    worldMapRegistrationSucceeded = true;
    return true;
  } catch (error) {
    const registrationError =
      error instanceof Error
        ? error
        : new Error('unknown world map registration error');
    console.error('Failed to register ECharts world map', registrationError);
    return false;
  }
}

export function WorldStageMap({
  isDark,
  nodes,
  sourceCountries,
}: {
  isDark: boolean;
  nodes: DashboardNodeHealth[];
  sourceCountries: DistributionItem[];
}) {
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
    const container = chartContainerRef.current;
    if (!container) {
      return;
    }

    const updateSize = () => {
      const nextWidth = container.clientWidth;
      const nextHeight = container.clientHeight;
      setContainerSize((previous) =>
        previous.width === nextWidth && previous.height === nextHeight
          ? previous
          : { width: nextWidth, height: nextHeight },
      );
    };

    updateSize();

    if (typeof ResizeObserver === 'undefined') {
      window.addEventListener('resize', updateSize);
      return () => {
        window.removeEventListener('resize', updateSize);
      };
    }

    const observer = new ResizeObserver(() => {
      updateSize();
    });
    observer.observe(container);

    return () => {
      observer.disconnect();
    };
  }, []);

  const mapPalette = useMemo(
    () =>
      isDark
        ? {
            areaColor: '#13233b',
            borderColor: 'rgba(125,211,252,0.14)',
            labelColor: '#e2e8f0',
            sourceAreaColor: 'rgba(56, 189, 248, 0.18)',
            sourceAreaBorder: 'rgba(125, 211, 252, 0.45)',
            healthyColor: '#34d399',
            warningColor: '#fbbf24',
            dangerColor: '#fb7185',
            healthyBorder: '#bbf7d0',
            warningBorder: '#fde68a',
            dangerBorder: '#fecdd3',
          }
        : {
            areaColor: '#eaf2ff',
            borderColor: 'rgba(71,85,105,0.12)',
            labelColor: '#0f172a',
            sourceAreaColor: 'rgba(14, 165, 233, 0.22)',
            sourceAreaBorder: 'rgba(14, 165, 233, 0.46)',
            healthyColor: '#10b981',
            warningColor: '#f59e0b',
            dangerColor: '#f43f5e',
            healthyBorder: '#d1fae5',
            warningBorder: '#fde68a',
            dangerBorder: '#fecdd3',
          },
    [isDark],
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

  const countryRegions = useMemo<CountryRegionDatum[]>(() => {
    const maxRequests = Math.max(
      1,
      ...sourceCountries.map((item) => item.value || 0),
    );

    return sourceCountries
      .filter((item) => item.key && item.value > 0)
      .map((item) => {
        const intensity = Math.max(0.18, item.value / maxRequests);
        const areaOpacity = Number((0.14 + intensity * 0.58).toFixed(3));
        const borderOpacity = Number((0.22 + intensity * 0.48).toFixed(3));
        const areaColor = isDark
          ? `rgba(56, 189, 248, ${areaOpacity})`
          : `rgba(14, 165, 233, ${areaOpacity})`;
        const borderColor = isDark
          ? `rgba(125, 211, 252, ${borderOpacity})`
          : `rgba(14, 165, 233, ${borderOpacity})`;

        return {
          name: item.key,
          value: item.value,
          itemStyle: {
            areaColor,
            borderColor,
          },
          emphasis: {
            itemStyle: {
              areaColor,
              borderColor,
            },
          },
        };
      });
  }, [isDark, sourceCountries]);

  const responsiveMapScale = useMemo(() => {
    const { width, height } = containerSize;
    if (width <= 0 || height <= 0) {
      return 1;
    }

    const widthScale = Math.min(Math.max(width / 960, 0.52), 1.08);
    const heightScale = Math.min(Math.max(height / 520, 0.72), 1.1);
    const compactViewportScale = width < 640 ? 0.9 : 1;

    return Number(
      (Math.min(widthScale, heightScale) * compactViewportScale).toFixed(3),
    );
  }, [containerSize]);

  const computedLayoutSize = useMemo(
    () => `${Math.round(baseWorldMapLayoutSizePercent * responsiveMapScale)}%`,
    [responsiveMapScale],
  );
  const computedZoom = useMemo(
    () => Number((baseWorldMapZoom * responsiveMapScale).toFixed(3)),
    [responsiveMapScale],
  );

  const mapOption = useMemo<EChartsCoreOption>(
    () => ({
      animation: false,
      backgroundColor: 'transparent',
      tooltip: {
        trigger: 'item',
        transitionDuration: 0,
        backgroundColor: isDark
          ? 'rgba(15,23,42,0.96)'
          : 'rgba(255,255,255,0.96)',
        borderColor: isDark
          ? 'rgba(148,163,184,0.2)'
          : 'rgba(148,163,184,0.24)',
        borderWidth: 1,
        textStyle: {
          color: isDark ? '#e2e8f0' : '#0f172a',
          fontSize: 12,
        },
        formatter: (params: unknown) => {
          const payload = params as {
            data?: MapNodeDatum | CountryRegionDatum;
            seriesType?: string;
            name?: string;
          };
          const data = payload.data;
          if (
            payload.seriesType !== 'scatter' &&
            data &&
            'value' in data &&
            typeof data.value === 'number'
          ) {
            return [
              `<div style="font-weight:600;margin-bottom:6px;">${payload.name ?? data.name}</div>`,
              `<div>最近 24 小时来源请求 ${data.value.toLocaleString('zh-CN')}</div>`,
            ].join('');
          }

          if (!data || !('requestCount' in data)) {
            return '';
          }

          const locationLine = data.derivedFromGeo
            ? data.geoName
            : `${data.geoName} · 预设落点`;

          return [
            `<div style="font-weight:600;margin-bottom:6px;">${data.name}</div>`,
            `<div>${locationLine}</div>`,
            `<div>请求量 ${data.requestCount.toLocaleString('zh-CN')} · 错误数 ${data.errorCount.toLocaleString('zh-CN')}</div>`,
            `<div>活动事件 ${data.activeEventCount} · 节点状态 ${getNodeStatusLabel(data.status)}</div>`,
            `<div>OpenResty 状态 ${getOpenrestyStatusLabel(data.openrestyStatus)}</div>`,
          ].join('');
        },
      },
      geo: {
        map: 'world',
        roam: true,
        silent: true,
        layoutCenter: ['50%', '50%'],
        layoutSize: computedLayoutSize,
        zoom: computedZoom,
        scaleLimit: {
          min: Math.max(Number((computedZoom * 0.78).toFixed(3)), 0.35),
          max: Math.max(Number((computedZoom * 2.2).toFixed(3)), 1.4),
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
          type: 'map',
          map: 'world',
          geoIndex: 0,
          data: countryRegions,
          silent: true,
          z: 1,
          emphasis: {
            disabled: true,
          },
        },
        {
          type: 'scatter',
          coordinateSystem: 'geo',
          data: mapNodes,
          z: 3,
          progressive: 64,
          large: true,
          largeThreshold: 24,
          symbolSize: (value: unknown) => {
            const size =
              Array.isArray(value) && typeof value[2] === 'number'
                ? value[2]
                : 1;
            const responsiveBase = 8 + Math.log10(size + 1) * 3.6;
            return Math.max(
              7,
              Math.min(18, responsiveBase * Math.max(responsiveMapScale, 0.88)),
            );
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
                ? 'rgba(8,15,31,0.7)'
                : 'rgba(255,255,255,0.88)',
              borderColor: isDark
                ? 'rgba(148,163,184,0.16)'
                : 'rgba(148,163,184,0.2)',
              borderWidth: 1,
              borderRadius: 999,
              padding: [4, 8],
            },
          },
        },
      ],
    }),
    [
      computedLayoutSize,
      computedZoom,
      countryRegions,
      isDark,
      mapNodes,
      mapPalette,
      responsiveMapScale,
    ],
  );

  if (!mapReady) {
    return (
      <div className="flex h-full items-center justify-center">
        <EmptyState
          title={mapFailed ? '全球地图加载失败' : '全球地图加载中'}
          description={
            mapFailed
              ? 'ECharts 世界地图资源未能成功注册，请稍后刷新重试。'
              : '正在按需初始化全球地图，这一步会延后到首屏内容稳定后执行。'
          }
        />
      </div>
    );
  }

  return (
    <div ref={chartContainerRef} className="h-full w-full">
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
        style={{ height: '100%', width: '100%' }}
      />
    </div>
  );
}
