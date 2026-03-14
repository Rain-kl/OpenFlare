'use client';

import type { EChartsOption } from 'echarts';
import ReactECharts from 'echarts-for-react';
import Link from 'next/link';
import { useRouter } from 'next/navigation';
import { useEffect, useMemo, useState } from 'react';

import { EmptyState } from '@/components/feedback/empty-state';
import { useTheme } from '@/components/providers/theme-provider';
import { StatusBadge } from '@/components/ui/status-badge';
import type {
  DashboardConfig,
  DashboardNodeHealth,
  DashboardRiskSummary,
  DashboardSummary,
  DistributionItem,
} from '@/features/dashboard/types';
import {
  getNodeStatusLabel,
  getNodeStatusVariant,
  getOpenrestyStatusLabel,
  getOpenrestyStatusVariant,
} from '@/features/nodes/utils';
import { cn } from '@/lib/utils/cn';
import { formatDateTime } from '@/lib/utils/date';

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
  tone: Tone;
  value: [number, number, number];
  itemStyle: {
    color: string;
    borderColor: string;
    borderWidth: number;
    shadowBlur: number;
    shadowColor: string;
  };
  emphasis: {
    itemStyle: {
      shadowBlur: number;
      shadowColor: string;
      borderColor: string;
      borderWidth: number;
    };
  };
};

function formatPercent(value: number) {
  if (!Number.isFinite(value)) {
    return '0%';
  }
  return `${value.toFixed(value >= 100 ? 0 : 1)}%`;
}

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
    coordinates: [...fallbackCoordinates[index % fallbackCoordinates.length]] as [
      number,
      number,
    ],
    derivedFromGeo: false,
  };
}

function HeroMetric({
  label,
  value,
  hint,
  isDark,
}: {
  label: string;
  value: string;
  hint: string;
  isDark: boolean;
}) {
  return (
    <div
      className={cn(
        'rounded-[24px] border px-4 py-4 backdrop-blur',
        isDark
          ? 'border-white/10 bg-white/6'
          : 'border-sky-100/90 bg-white/80 shadow-[0_18px_40px_rgba(148,163,184,0.12)]',
      )}
    >
      <p
        className={cn(
          'text-[11px] tracking-[0.26em] uppercase',
          isDark ? 'text-slate-300' : 'text-slate-500',
        )}
      >
        {label}
      </p>
      <p
        className={cn(
          'mt-3 text-2xl font-semibold',
          isDark ? 'text-white' : 'text-slate-950',
        )}
      >
        {value}
      </p>
      <p className={cn('mt-2 text-sm', isDark ? 'text-slate-300' : 'text-slate-600')}>
        {hint}
      </p>
    </div>
  );
}

function LegendPill({
  label,
  tone,
  isDark,
}: {
  label: string;
  tone: Tone;
  isDark: boolean;
}) {
  const toneClass =
    tone === 'healthy'
      ? isDark
        ? 'border-emerald-300/20 bg-emerald-400/10 text-emerald-100'
        : 'border-emerald-200 bg-emerald-50 text-emerald-700'
      : tone === 'warning'
        ? isDark
          ? 'border-amber-300/20 bg-amber-400/10 text-amber-100'
          : 'border-amber-200 bg-amber-50 text-amber-700'
        : isDark
          ? 'border-rose-300/20 bg-rose-400/10 text-rose-100'
          : 'border-rose-200 bg-rose-50 text-rose-700';

  return (
    <div className={cn('rounded-full border px-3 py-1 text-[11px]', toneClass)}>
      {label}
    </div>
  );
}

function CountrySignal({
  item,
  index,
  isDark,
}: {
  item: DistributionItem;
  index: number;
  isDark: boolean;
}) {
  const darkAccents = [
    'from-sky-400/35 to-cyan-400/10',
    'from-violet-400/35 to-fuchsia-400/10',
    'from-emerald-400/35 to-teal-400/10',
  ];
  const lightAccents = [
    'from-sky-100 via-white to-cyan-50',
    'from-indigo-100 via-white to-fuchsia-50',
    'from-emerald-100 via-white to-teal-50',
  ];

  return (
    <div
      className={cn(
        'rounded-2xl border bg-gradient-to-br px-4 py-3 backdrop-blur',
        isDark
          ? `border-white/10 ${darkAccents[index % darkAccents.length]}`
          : `border-slate-200/80 ${lightAccents[index % lightAccents.length]} shadow-[0_14px_30px_rgba(148,163,184,0.12)]`,
      )}
    >
      <p
        className={cn(
          'text-[11px] tracking-[0.24em] uppercase',
          isDark ? 'text-slate-200' : 'text-slate-500',
        )}
      >
        {item.key}
      </p>
      <p
        className={cn(
          'mt-2 text-lg font-semibold',
          isDark ? 'text-white' : 'text-slate-950',
        )}
      >
        {item.value.toLocaleString('zh-CN')}
      </p>
      <p className={cn('mt-1 text-xs', isDark ? 'text-slate-300' : 'text-slate-600')}>
        最近 24 小时来源信号
      </p>
    </div>
  );
}

export function WorldStage({
  generatedAt,
  summary,
  risk,
  config,
  nodes,
  sourceCountries,
}: {
  generatedAt: string;
  summary: DashboardSummary;
  risk: DashboardRiskSummary;
  config: DashboardConfig;
  nodes: DashboardNodeHealth[];
  sourceCountries: DistributionItem[];
}) {
  const router = useRouter();
  const { resolvedTheme } = useTheme();
  const isDark = resolvedTheme === 'dark';
  const [mapReady, setMapReady] = useState(false);
  const [mapFailed, setMapFailed] = useState(false);

  useEffect(() => {
    let disposed = false;

    import('echarts-maps/world.js')
      .then(() => {
        if (!disposed) {
          setMapReady(true);
          setMapFailed(false);
        }
      })
      .catch(() => {
        if (!disposed) {
          setMapReady(false);
          setMapFailed(true);
        }
      });

    return () => {
      disposed = true;
    };
  }, []);

  const onlineRate =
    summary.total_nodes > 0
      ? (summary.online_nodes / summary.total_nodes) * 100
      : 0;
  const syncedNodes = Math.max(
    0,
    summary.total_nodes - summary.lagging_nodes - summary.pending_nodes,
  );
  const syncRate =
    summary.total_nodes > 0 ? (syncedNodes / summary.total_nodes) * 100 : 0;
  const healthyNodes = Math.max(
    0,
    summary.online_nodes - summary.unhealthy_nodes - risk.offline_nodes,
  );
  const healthyRate =
    summary.total_nodes > 0 ? (healthyNodes / summary.total_nodes) * 100 : 0;
  const geoConfiguredNodes = nodes.filter(
    (node) =>
      typeof node.geo_latitude === 'number' &&
      typeof node.geo_longitude === 'number',
  ).length;

  const mapPalette = useMemo(
    () =>
      isDark
        ? {
            areaColor: '#13233b',
            borderColor: 'rgba(125,211,252,0.18)',
            shadowColor: 'rgba(8,15,31,0.35)',
            labelColor: '#e2e8f0',
            subLabelColor: '#94a3b8',
            healthyColor: '#34d399',
            warningColor: '#fbbf24',
            dangerColor: '#fb7185',
            healthyBorder: '#bbf7d0',
            warningBorder: '#fde68a',
            dangerBorder: '#fecdd3',
          }
        : {
            areaColor: '#eaf2ff',
            borderColor: 'rgba(71,85,105,0.16)',
            shadowColor: 'rgba(148,163,184,0.18)',
            labelColor: '#0f172a',
            subLabelColor: '#475569',
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
          tone,
          value: [coordinates[0], coordinates[1], Math.max(node.request_count, 1)],
          itemStyle: {
            color: toneColor,
            borderColor: toneBorder,
            borderWidth: 2,
            shadowBlur: 18,
            shadowColor: toneColor,
          },
          emphasis: {
            itemStyle: {
              shadowBlur: 24,
              shadowColor: toneColor,
              borderColor: toneBorder,
              borderWidth: 2,
            },
          },
        };
      }),
    [mapPalette, nodes],
  );

  const mapOption = useMemo<EChartsOption>(
    () => ({
      backgroundColor: 'transparent',
      tooltip: {
        trigger: 'item',
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
          const data = (params as { data?: MapNodeDatum }).data;
          if (!data) {
            return '';
          }

          const locationLine = data.derivedFromGeo
            ? data.geoName
            : `${data.geoName} · 备用落点`;

          return [
            `<div style="font-weight:600;margin-bottom:6px;">${data.name}</div>`,
            `<div>${locationLine}</div>`,
            `<div>请求 ${data.requestCount.toLocaleString('zh-CN')} · 错误 ${data.errorCount.toLocaleString('zh-CN')}</div>`,
            `<div>活动异常 ${data.activeEventCount} · 节点 ${getNodeStatusLabel(data.status)}</div>`,
            `<div>OpenResty ${getOpenrestyStatusLabel(data.openrestyStatus)}</div>`,
          ].join('');
        },
      },
      geo: {
        map: 'world',
        roam: false,
        silent: true,
        top: 18,
        bottom: 18,
        left: 10,
        right: 10,
        itemStyle: {
          areaColor: mapPalette.areaColor,
          borderColor: mapPalette.borderColor,
          borderWidth: 0.8,
          shadowBlur: 12,
          shadowColor: mapPalette.shadowColor,
        },
        emphasis: {
          disabled: true,
        },
      },
      series: [
        {
          type: 'effectScatter',
          coordinateSystem: 'geo',
          data: mapNodes,
          showEffectOn: 'render',
          rippleEffect: {
            scale: 4,
            brushType: 'stroke',
          },
          symbolSize: (value: unknown) => {
            const size = Array.isArray(value) && typeof value[2] === 'number' ? value[2] : 1;
            return Math.max(10, Math.min(24, 10 + Math.log10(size + 1) * 4.5));
          },
          label: {
            show: mapNodes.length <= 8,
            position: 'right',
            distance: 8,
            formatter: '{b}',
            color: mapPalette.labelColor,
            fontSize: 11,
            backgroundColor: isDark ? 'rgba(8,15,31,0.7)' : 'rgba(255,255,255,0.88)',
            borderColor: isDark
              ? 'rgba(148,163,184,0.16)'
              : 'rgba(148,163,184,0.2)',
            borderWidth: 1,
            borderRadius: 999,
            padding: [4, 8],
          },
          emphasis: {
            scale: true,
            label: {
              show: true,
            },
          },
        },
      ],
    }),
    [isDark, mapNodes, mapPalette],
  );

  return (
    <section
      className={cn(
        'overflow-hidden rounded-[32px] border transition-colors',
        isDark
          ? 'border-slate-800/70 bg-[radial-gradient(circle_at_top_left,rgba(56,189,248,0.18),transparent_28%),radial-gradient(circle_at_82%_18%,rgba(56,189,248,0.10),transparent_18%),linear-gradient(135deg,#08111f,#0f172a_45%,#111827)] shadow-[0_32px_80px_rgba(2,6,23,0.35)]'
          : 'border-sky-100/90 bg-[radial-gradient(circle_at_top_left,rgba(14,165,233,0.18),transparent_28%),radial-gradient(circle_at_82%_18%,rgba(59,130,246,0.12),transparent_20%),linear-gradient(135deg,#f8fbff,#edf5ff_45%,#ffffff)] shadow-[0_32px_80px_rgba(148,163,184,0.18)]',
      )}
    >
      <div
        className={cn(
          'border-b px-6 py-5 md:px-7',
          isDark ? 'border-white/8' : 'border-slate-200/70',
        )}
      >
        <div className="flex flex-col gap-4 xl:flex-row xl:items-end xl:justify-between">
          <div className="space-y-2">
            <p
              className={cn(
                'text-[11px] tracking-[0.34em] uppercase',
                isDark ? 'text-sky-200/80' : 'text-sky-700/80',
              )}
            >
              Global Stage
            </p>
            <h2
              className={cn(
                'text-2xl font-semibold',
                isDark ? 'text-white' : 'text-slate-950',
              )}
            >
              全球态势板
            </h2>
            <p
              className={cn(
                'max-w-3xl text-sm leading-6',
                isDark ? 'text-slate-300' : 'text-slate-600',
              )}
            >
              先把节点健康、配置追平、活动风险和全球来源信号拉到同一张首屏，
              让总览页真正承担值守入口的职责。
            </p>
          </div>
          <div
            className={cn(
              'rounded-full border px-4 py-2 text-sm backdrop-blur',
              isDark
                ? 'border-white/10 bg-white/6 text-slate-200'
                : 'border-slate-200/80 bg-white/80 text-slate-700',
            )}
          >
            数据生成于 {formatDateTime(generatedAt)}
          </div>
        </div>
      </div>

      <div className="grid gap-6 px-6 py-6 md:px-7 xl:grid-cols-[1.4fr_0.8fr]">
        <div className="space-y-4">
          <div
            className={cn(
              'relative min-h-[420px] overflow-hidden rounded-[28px] border',
              isDark
                ? 'border-white/10 bg-[linear-gradient(180deg,rgba(15,23,42,0.16),rgba(15,23,42,0.42))]'
                : 'border-slate-200/80 bg-[linear-gradient(180deg,rgba(255,255,255,0.88),rgba(239,246,255,0.92))]',
            )}
          >
            <div
              className={cn(
                'absolute left-6 top-6 z-10 rounded-full px-3 py-1 text-[11px] tracking-[0.22em] uppercase backdrop-blur',
                isDark
                  ? 'bg-sky-400/20 text-sky-100'
                  : 'bg-sky-100/90 text-sky-700',
              )}
            >
              {geoConfiguredNodes > 0 ? '真实节点点位' : '节点信号覆盖'}
            </div>

            <div className="absolute right-4 top-4 z-10 flex flex-wrap gap-2">
              <LegendPill label="绿色: 运行健康" tone="healthy" isDark={isDark} />
              <LegendPill label="黄色: 容量压力" tone="warning" isDark={isDark} />
              <LegendPill label="红色: 异常或离线" tone="danger" isDark={isDark} />
            </div>

            <div className="absolute inset-0">
              {mapReady ? (
                <ReactECharts
                  option={mapOption}
                  notMerge
                  lazyUpdate
                  onEvents={{
                    click: (params: { data?: MapNodeDatum }) => {
                      if (params.data?.route) {
                        router.push(params.data.route);
                      }
                    },
                  }}
                  style={{ height: '100%', width: '100%' }}
                />
              ) : (
                <div className="flex h-full items-center justify-center">
                  <EmptyState
                    title={mapFailed ? '全球地图加载失败' : '全球地图加载中'}
                    description={
                      mapFailed
                        ? 'ECharts 世界地图资源未能成功注册，请稍后刷新重试。'
                        : '正在准备 ECharts 世界地图与节点信号点位。'
                    }
                  />
                </div>
              )}
            </div>

            {mapReady && nodes.length === 0 ? (
              <div className="pointer-events-none absolute inset-x-6 bottom-24 z-10">
                <div
                  className={cn(
                    'rounded-2xl border border-dashed px-4 py-4 text-sm backdrop-blur',
                    isDark
                      ? 'border-white/15 bg-white/5 text-slate-300'
                      : 'border-slate-300/70 bg-white/78 text-slate-600',
                  )}
                >
                  当前还没有节点接入，地图已就绪，后续会在这里展示节点真实落点与健康颜色。
                </div>
              </div>
            ) : null}

            <div className="absolute bottom-4 left-4 right-4 z-10 grid gap-3 md:grid-cols-3">
              {sourceCountries.length > 0 ? (
                sourceCountries.slice(0, 3).map((item, index) => (
                  <CountrySignal
                    key={`${item.key}-${item.value}`}
                    item={item}
                    index={index}
                    isDark={isDark}
                  />
                ))
              ) : (
                <div
                  className={cn(
                    'rounded-2xl border border-dashed px-4 py-4 text-sm backdrop-blur md:col-span-3',
                    isDark
                      ? 'border-white/15 bg-white/5 text-slate-300'
                      : 'border-slate-300/70 bg-white/78 text-slate-600',
                  )}
                >
                  当前还没有可用于全球分布展示的来源国家数据。
                </div>
              )}
            </div>
          </div>

          <div
            className={cn(
              'rounded-[24px] border px-4 py-3 text-xs leading-6 backdrop-blur',
              isDark
                ? 'border-white/10 bg-white/5 text-slate-300'
                : 'border-slate-200/80 bg-white/80 text-slate-600',
            )}
          >
            当前已有 {geoConfiguredNodes}/{summary.total_nodes} 个节点配置了真实地图坐标。
            未配置坐标的节点会落在预设世界城市点位，保证首页仍能看到全球覆盖信号。
          </div>
        </div>

        <div className="space-y-4">
          <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-1">
            <HeroMetric
              label="在线覆盖"
              value={formatPercent(onlineRate)}
              hint={`${summary.online_nodes}/${summary.total_nodes} 节点在线`}
              isDark={isDark}
            />
            <HeroMetric
              label="运行健康"
              value={formatPercent(healthyRate)}
              hint={`${summary.unhealthy_nodes} 个 OpenResty 不健康`}
              isDark={isDark}
            />
            <HeroMetric
              label="配置追平"
              value={formatPercent(syncRate)}
              hint={`${summary.lagging_nodes} 个节点未追平 ${config.active_version || '当前激活版本'}`}
              isDark={isDark}
            />
            <HeroMetric
              label="活动风险"
              value={summary.active_alerts.toLocaleString('zh-CN')}
              hint={`${risk.critical_alerts} Critical · ${risk.warning_alerts} Warning`}
              isDark={isDark}
            />
          </div>

          <div
            className={cn(
              'rounded-[28px] border px-5 py-5 backdrop-blur',
              isDark
                ? 'border-white/10 bg-white/6'
                : 'border-slate-200/80 bg-white/80 shadow-[0_18px_40px_rgba(148,163,184,0.12)]',
            )}
          >
            <div className="flex items-center justify-between gap-3">
              <div>
                <p
                  className={cn(
                    'text-[11px] tracking-[0.24em] uppercase',
                    isDark ? 'text-slate-300' : 'text-slate-500',
                  )}
                >
                  节点健康清单
                </p>
                <p
                  className={cn(
                    'mt-2 text-lg font-semibold',
                    isDark ? 'text-white' : 'text-slate-950',
                  )}
                >
                  风险优先队列
                </p>
              </div>
              <Link
                href="/node"
                className={cn(
                  'rounded-full border px-3 py-1.5 text-xs transition',
                  isDark
                    ? 'border-white/12 text-slate-100 hover:bg-white/8'
                    : 'border-slate-200 text-slate-700 hover:bg-slate-100/80',
                )}
              >
                查看全部节点
              </Link>
            </div>
            <div className="mt-4 space-y-3">
              {nodes.slice(0, 4).map((node) => (
                <Link
                  key={node.node_id}
                  href={buildNodeDetailHref(node.id)}
                  className={cn(
                    'block rounded-2xl border px-4 py-4 transition',
                    isDark
                      ? 'border-white/8 bg-slate-950/20 hover:border-white/18 hover:bg-white/6'
                      : 'border-slate-200/80 bg-slate-50/70 hover:border-slate-300 hover:bg-white',
                  )}
                >
                  <div className="flex items-start justify-between gap-3">
                    <div>
                      <p
                        className={cn(
                          'text-sm font-semibold',
                          isDark ? 'text-white' : 'text-slate-950',
                        )}
                      >
                        {node.name}
                      </p>
                      <p
                        className={cn(
                          'mt-1 text-xs',
                          isDark ? 'text-slate-400' : 'text-slate-500',
                        )}
                      >
                        请求 {node.request_count.toLocaleString('zh-CN')} · 错误{' '}
                        {node.error_count.toLocaleString('zh-CN')}
                      </p>
                    </div>
                    <div className="flex flex-wrap justify-end gap-2">
                      <StatusBadge
                        label={getNodeStatusLabel(node.status)}
                        variant={getNodeStatusVariant(node.status)}
                      />
                      <StatusBadge
                        label={getOpenrestyStatusLabel(node.openresty_status)}
                        variant={getOpenrestyStatusVariant(
                          node.openresty_status,
                        )}
                      />
                    </div>
                  </div>
                </Link>
              ))}
              {nodes.length === 0 ? (
                <div
                  className={cn(
                    'rounded-2xl border border-dashed px-4 py-5 text-sm',
                    isDark
                      ? 'border-white/12 bg-slate-950/20 text-slate-300'
                      : 'border-slate-300/70 bg-slate-50/75 text-slate-600',
                  )}
                >
                  当前没有节点健康数据，等节点开始 heartbeat 后这里会出现风险优先队列。
                </div>
              ) : null}
            </div>
          </div>
        </div>
      </div>
    </section>
  );
}
