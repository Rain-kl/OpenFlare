'use client';

import { useMemo, useRef, useState } from 'react';
import { Download, Loader2, RefreshCw, ZoomIn, ZoomOut } from 'lucide-react';
import { useTranslations } from 'next-intl';

import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import type {
  NetToolsASNUpstreamGraphResponse,
  NetToolsASUpstreamEdge,
  NetToolsASUpstreamNode,
} from '@/lib/services/openflare';

type UpstreamRoutingGraphProps = {
  data?: NetToolsASNUpstreamGraphResponse;
  isLoading: boolean;
  isFetching: boolean;
  onRefresh: () => void;
  onSelectAsn: (asn: number) => void;
};

type PlacedNode = {
  node: NetToolsASUpstreamNode;
  x: number;
  y: number;
  width: number;
  height: number;
};

type RoutedEdge = {
  key: string;
  edge: NetToolsASUpstreamEdge;
  d: string;
};

type GroupBox = {
  key: string;
  kind: 'origins' | 'upstreams' | 'tier1';
  x: number;
  y: number;
  width: number;
  height: number;
};

type GraphLayout = {
  width: number;
  height: number;
  nodes: PlacedNode[];
  edges: RoutedEdge[];
  groups: GroupBox[];
};

const NODE_W = 140;
const NODE_H = 56;
const ROW_GAP = 18;
const OUTER_PAD_X = 28;
const OUTER_PAD_TOP = 54;
const OUTER_PAD_BOTTOM = 36;
const GROUP_PAD_X = 18;
const GROUP_PAD_TOP = 34;
const GROUP_PAD_BOTTOM = 18;
const BASE_GUTTER = 76;
const LANE_SPACING = 8;
const GROUP_GAP_EXTRA = 26;
const SKIP_CORRIDOR_STEP = 10;

function computeColumnOffsets(
  layerIndices: number[],
  nodesByLayer: Map<number, NetToolsASUpstreamNode[]>,
  edges: NetToolsASUpstreamEdge[],
  nodeLayer: Map<number, number>,
): { colX: Map<number, number>; totalWidth: number } {
  const gutterCount = new Map<number, number>();
  for (const e of edges) {
    const lFrom = nodeLayer.get(e.from);
    const lTo = nodeLayer.get(e.to);
    if (lFrom !== undefined && lTo !== undefined && lTo > lFrom) {
      gutterCount.set(lFrom, (gutterCount.get(lFrom) ?? 0) + 1);
    }
  }

  const colX = new Map<number, number>();
  let curX = OUTER_PAD_X + GROUP_PAD_X;
  for (let i = 0; i < layerIndices.length; i++) {
    const layer = layerIndices[i];
    colX.set(layer, curX);
    if (i + 1 < layerIndices.length) {
      const nextLayer = layerIndices[i + 1];
      const count = gutterCount.get(layer) ?? 0;
      const laneSpan = Math.max(0, count - 1) * LANE_SPACING;
      const curRoles = nodesByLayer.get(layer) ?? [];
      const nextRoles = nodesByLayer.get(nextLayer) ?? [];
      const crossesGroupBoundary =
        curRoles[0]?.role !== nextRoles[0]?.role &&
        !(
          curRoles[0]?.role === 'tier1_upstream' &&
          nextRoles[0]?.role === 'tier1'
        );
      const extra = crossesGroupBoundary
        ? GROUP_PAD_X * 2 + GROUP_GAP_EXTRA
        : 0;
      const gutter = Math.max(BASE_GUTTER, 44 + laneSpan) + extra;
      curX += NODE_W + gutter;
    }
  }
  const totalWidth = curX + NODE_W + GROUP_PAD_X + OUTER_PAD_X;
  return { colX, totalWidth };
}

function distributePortY(nodeY: number, index: number, total: number): number {
  const usable = NODE_H - 16;
  if (total <= 1) {
    return nodeY + NODE_H / 2;
  }
  const step = usable / (total - 1);
  return nodeY + 8 + index * step;
}

function buildGraphLayout(
  data?: NetToolsASNUpstreamGraphResponse,
): GraphLayout | null {
  if (!data || data.nodes.length === 0) {
    return null;
  }

  const nodesByLayer = new Map<number, NetToolsASUpstreamNode[]>();
  const nodeLayer = new Map<number, number>();
  for (const n of data.nodes) {
    nodeLayer.set(n.asn, n.layer);
    const bucket = nodesByLayer.get(n.layer) ?? [];
    bucket.push(n);
    nodesByLayer.set(n.layer, bucket);
  }
  const layerIndices = Array.from(nodesByLayer.keys()).sort((a, b) => a - b);

  let maxColHeight = 0;
  for (const list of nodesByLayer.values()) {
    const h = list.length * NODE_H + Math.max(0, list.length - 1) * ROW_GAP;
    if (h > maxColHeight) maxColHeight = h;
  }

  const skipEdgesCount = data.edges.filter((e) => {
    const lFrom = nodeLayer.get(e.from) ?? 0;
    const lTo = nodeLayer.get(e.to) ?? 0;
    return lTo - lFrom > 1;
  }).length;
  const skipReserve =
    skipEdgesCount > 0 ? 18 + skipEdgesCount * SKIP_CORRIDOR_STEP : 0;

  const { colX, totalWidth } = computeColumnOffsets(
    layerIndices,
    nodesByLayer,
    data.edges,
    nodeLayer,
  );

  const placedByAsn = new Map<number, PlacedNode>();
  const placedNodes: PlacedNode[] = [];

  for (const layer of layerIndices) {
    const list = nodesByLayer.get(layer) ?? [];
    const colH = list.length * NODE_H + Math.max(0, list.length - 1) * ROW_GAP;
    const topY = OUTER_PAD_TOP + GROUP_PAD_TOP + (maxColHeight - colH) / 2;
    const x = colX.get(layer) ?? OUTER_PAD_X;
    list.forEach((node, idx) => {
      const placed: PlacedNode = {
        node,
        x,
        y: topY + idx * (NODE_H + ROW_GAP),
        width: NODE_W,
        height: NODE_H,
      };
      placedNodes.push(placed);
      placedByAsn.set(node.asn, placed);
    });
  }

  const contentBottom =
    OUTER_PAD_TOP + GROUP_PAD_TOP + maxColHeight + GROUP_PAD_BOTTOM;
  const totalHeight = contentBottom + skipReserve + OUTER_PAD_BOTTOM;

  // Build group backdrop boxes: Origins (role=origin), Upstreams (role=transit), Tier 1 ISPs (tier1_upstream + tier1)
  const groups: GroupBox[] = [];
  const collectGroup = (
    key: string,
    kind: GroupBox['kind'],
    predicate: (n: PlacedNode) => boolean,
  ) => {
    const members = placedNodes.filter(predicate);
    if (members.length === 0) return;
    const minX = Math.min(...members.map((m) => m.x)) - GROUP_PAD_X;
    const maxX = Math.max(...members.map((m) => m.x + m.width)) + GROUP_PAD_X;
    const minY = Math.min(...members.map((m) => m.y)) - GROUP_PAD_TOP;
    const maxY =
      Math.max(...members.map((m) => m.y + m.height)) + GROUP_PAD_BOTTOM;
    groups.push({
      key,
      kind,
      x: minX,
      y: minY,
      width: maxX - minX,
      height: maxY - minY,
    });
  };
  collectGroup('origins', 'origins', (p) => p.node.role === 'origin');
  collectGroup('upstreams', 'upstreams', (p) => p.node.role === 'transit');
  collectGroup(
    'tier1',
    'tier1',
    (p) => p.node.role === 'tier1_upstream' || p.node.role === 'tier1',
  );

  // Order outgoing edges per source and incoming edges per target by neighbor Y
  const outByAsn = new Map<number, NetToolsASUpstreamEdge[]>();
  const inByAsn = new Map<number, NetToolsASUpstreamEdge[]>();
  const validEdges = data.edges.filter(
    (e) => placedByAsn.has(e.from) && placedByAsn.has(e.to),
  );

  for (const e of validEdges) {
    const o = outByAsn.get(e.from) ?? [];
    o.push(e);
    outByAsn.set(e.from, o);

    const inc = inByAsn.get(e.to) ?? [];
    inc.push(e);
    inByAsn.set(e.to, inc);
  }
  for (const list of outByAsn.values()) {
    list.sort(
      (a, b) =>
        (placedByAsn.get(a.to)?.y ?? 0) - (placedByAsn.get(b.to)?.y ?? 0) ||
        a.to - b.to,
    );
  }
  for (const list of inByAsn.values()) {
    list.sort(
      (a, b) =>
        (placedByAsn.get(a.from)?.y ?? 0) - (placedByAsn.get(b.from)?.y ?? 0) ||
        a.from - b.from,
    );
  }

  // Group edges by left-hand gutter index to spread parallel vertical bus lanes cleanly
  const gutterEdges = new Map<number, NetToolsASUpstreamEdge[]>();
  for (const e of validEdges) {
    const lFrom = placedByAsn.get(e.from)?.node.layer ?? 0;
    const bucket = gutterEdges.get(lFrom) ?? [];
    bucket.push(e);
    gutterEdges.set(lFrom, bucket);
  }
  const edgeLaneIndex = new Map<string, number>();
  for (const [lFrom, bucket] of gutterEdges.entries()) {
    bucket.sort((a, b) => {
      const ayFrom = placedByAsn.get(a.from)?.y ?? 0;
      const byFrom = placedByAsn.get(b.from)?.y ?? 0;
      if (ayFrom !== byFrom) return ayFrom - byFrom;
      const ayTo = placedByAsn.get(a.to)?.y ?? 0;
      const byTo = placedByAsn.get(b.to)?.y ?? 0;
      return ayTo - byTo;
    });
    bucket.forEach((e, idx) => {
      edgeLaneIndex.set(`${lFrom}:${e.from}-${e.to}`, idx);
    });
  }

  const gutterNextX = (layerIdx: number): number => {
    const nextLayer = layerIndices.find((l) => l > layerIdx);
    const x1 = (colX.get(layerIdx) ?? 0) + NODE_W;
    const x2 =
      nextLayer !== undefined ? (colX.get(nextLayer) ?? x1 + 80) : x1 + 80;
    return (x1 + x2) / 2;
  };

  let skipCorridorIdx = 0;
  const routedEdges: RoutedEdge[] = validEdges.map((edge) => {
    const src = placedByAsn.get(edge.from)!;
    const dst = placedByAsn.get(edge.to)!;
    const outs = outByAsn.get(edge.from) ?? [edge];
    const ins = inByAsn.get(edge.to) ?? [edge];
    const outIdx = Math.max(
      0,
      outs.findIndex((x) => x.to === edge.to),
    );
    const inIdx = Math.max(
      0,
      ins.findIndex((x) => x.from === edge.from),
    );

    const startX = src.x + src.width;
    const startY = distributePortY(src.y, outIdx, outs.length);
    const endX = dst.x;
    const endY = distributePortY(dst.y, inIdx, ins.length);

    const bucketLen = gutterEdges.get(src.node.layer)?.length ?? 1;
    const laneIdx =
      edgeLaneIndex.get(`${src.node.layer}:${edge.from}-${edge.to}`) ?? 0;
    const centerX = gutterNextX(src.node.layer);
    const laneOffset = (laneIdx - (bucketLen - 1) / 2) * LANE_SPACING;
    const minSafeX = startX + 12;
    const maxSafeX = endX - 14;
    const laneX = Math.max(minSafeX, Math.min(maxSafeX, centerX + laneOffset));

    let d: string;
    if (dst.node.layer - src.node.layer <= 1) {
      d = `M ${startX} ${startY} L ${laneX} ${startY} L ${laneX} ${endY} L ${endX} ${endY}`;
    } else {
      // Bypass intermediate column(s) beneath the main box stacks cleanly
      const bypassY = contentBottom + 12 + skipCorridorIdx * SKIP_CORRIDOR_STEP;
      skipCorridorIdx += 1;
      const entryPrevGutter = dst.node.layer - 1;
      const entryCenterX = gutterNextX(entryPrevGutter);
      const entryX = Math.min(
        endX - 14,
        Math.max((colX.get(entryPrevGutter) ?? 0) + NODE_W + 14, entryCenterX),
      );
      d = `M ${startX} ${startY} L ${laneX} ${startY} L ${laneX} ${bypassY} L ${entryX} ${bypassY} L ${entryX} ${endY} L ${endX} ${endY}`;
    }

    return {
      key: `${edge.from}-${edge.to}`,
      edge,
      d,
    };
  });

  return {
    width: Math.max(520, Math.round(totalWidth)),
    height: Math.max(220, Math.round(totalHeight)),
    nodes: placedNodes,
    edges: routedEdges,
    groups,
  };
}

export function UpstreamRoutingGraph({
  data,
  isLoading,
  isFetching,
  onRefresh,
  onSelectAsn,
}: UpstreamRoutingGraphProps) {
  const t = useTranslations('networkTools');
  const svgRef = useRef<SVGSVGElement | null>(null);
  const [zoom, setZoom] = useState(1);
  const [hoveredAsn, setHoveredAsn] = useState<number | null>(null);
  const [hoveredEdgeKey, setHoveredEdgeKey] = useState<string | null>(null);

  const layout = useMemo(() => buildGraphLayout(data), [data]);

  const activeHighlight = useMemo(() => {
    const highlightedNodes = new Set<number>();
    const highlightedEdges = new Set<string>();
    if (!data) return { highlightedNodes, highlightedEdges, hasActive: false };

    if (hoveredEdgeKey) {
      const matched = data.edges.find(
        (e) => `${e.from}-${e.to}` === hoveredEdgeKey,
      );
      if (matched) {
        highlightedEdges.add(hoveredEdgeKey);
        highlightedNodes.add(matched.from);
        highlightedNodes.add(matched.to);
        return { highlightedNodes, highlightedEdges, hasActive: true };
      }
    }
    if (hoveredAsn !== null) {
      highlightedNodes.add(hoveredAsn);
      for (const e of data.edges) {
        if (e.from === hoveredAsn || e.to === hoveredAsn) {
          highlightedEdges.add(`${e.from}-${e.to}`);
          highlightedNodes.add(e.from);
          highlightedNodes.add(e.to);
        }
      }
      return { highlightedNodes, highlightedEdges, hasActive: true };
    }
    return { highlightedNodes, highlightedEdges, hasActive: false };
  }, [data, hoveredAsn, hoveredEdgeKey]);

  const hoveredNodeDetail = useMemo(() => {
    if (hoveredAsn === null || !data) return null;
    return data.nodes.find((n) => n.asn === hoveredAsn) ?? null;
  }, [data, hoveredAsn]);

  const hoveredEdgeDetail = useMemo(() => {
    if (!hoveredEdgeKey || !data) return null;
    return (
      data.edges.find((e) => `${e.from}-${e.to}` === hoveredEdgeKey) ?? null
    );
  }, [data, hoveredEdgeKey]);

  const handleExportSvg = () => {
    const el = svgRef.current;
    if (!el || !data) return;
    const serializer = new XMLSerializer();
    const source = serializer.serializeToString(el);
    const blob = new Blob([source], { type: 'image/svg+xml;charset=utf-8' });
    const url = URL.createObjectURL(blob);
    const link = document.createElement('a');
    link.href = url;
    link.download = `as${data.origin_asn}-upstream-routing.svg`;
    document.body.appendChild(link);
    link.click();
    link.remove();
    URL.revokeObjectURL(url);
  };

  const groupLabel = (kind: GroupBox['kind']) => {
    if (kind === 'origins') return t('asn.upstreamGraph.groups.origins');
    if (kind === 'upstreams') return t('asn.upstreamGraph.groups.upstreams');
    return t('asn.upstreamGraph.groups.tier1');
  };

  const sourceLabel = (
    src: NetToolsASNUpstreamGraphResponse['data_source'],
  ) => {
    return t(`asn.upstreamGraph.sources.${src}`);
  };

  return (
    <Card>
      <CardHeader className='flex flex-row flex-wrap items-center justify-between gap-3 pb-3'>
        <div className='space-y-1'>
          <div className='flex flex-wrap items-center gap-2'>
            <CardTitle className='text-base font-medium'>
              {t('asn.upstreamGraph.title')}
            </CardTitle>
            {data ? (
              <Badge variant='secondary' className='text-xs'>
                {sourceLabel(data.data_source)}
              </Badge>
            ) : null}
            {data?.sampled_prefix ? (
              <Badge variant='outline' className='font-mono text-xs'>
                {data.sampled_prefix}
              </Badge>
            ) : null}
            {data?.total_paths_parsed ? (
              <span className='text-xs text-muted-foreground'>
                {t('asn.upstreamGraph.pathCount', {
                  count: data.total_paths_parsed,
                })}
              </span>
            ) : null}
          </div>
          <p className='text-xs text-muted-foreground'>
            {t('asn.upstreamGraph.subtitle')}
          </p>
        </div>

        <div className='flex flex-wrap items-center gap-1.5'>
          <Button
            type='button'
            variant='outline'
            size='sm'
            disabled={zoom <= 0.6}
            onClick={() =>
              setZoom((z) => Math.max(0.6, Number((z - 0.15).toFixed(2))))
            }
            aria-label={t('asn.upstreamGraph.zoomOut')}
          >
            <ZoomOut className='size-3.5' />
          </Button>
          <Button
            type='button'
            variant='outline'
            size='sm'
            onClick={() => setZoom(1)}
            className='min-w-[54px] font-mono text-xs'
          >
            {Math.round(zoom * 100)}%
          </Button>
          <Button
            type='button'
            variant='outline'
            size='sm'
            disabled={zoom >= 1.8}
            onClick={() =>
              setZoom((z) => Math.min(1.8, Number((z + 0.15).toFixed(2))))
            }
            aria-label={t('asn.upstreamGraph.zoomIn')}
          >
            <ZoomIn className='size-3.5' />
          </Button>
          <Button
            type='button'
            variant='outline'
            size='sm'
            disabled={!layout}
            onClick={handleExportSvg}
          >
            <Download className='mr-1.5 size-3.5' />
            {t('asn.upstreamGraph.exportSvg')}
          </Button>
          <Button
            type='button'
            variant='outline'
            size='sm'
            disabled={isFetching}
            onClick={onRefresh}
          >
            {isFetching ? (
              <Loader2 className='mr-1.5 size-3.5 animate-spin' />
            ) : (
              <RefreshCw className='mr-1.5 size-3.5' />
            )}
            {t('asn.upstreamGraph.refresh')}
          </Button>
        </div>
      </CardHeader>

      <CardContent className='space-y-3'>
        {isLoading ? (
          <div className='flex items-center justify-center rounded-xl border border-dashed py-14 text-sm text-muted-foreground'>
            <Loader2 className='mr-2 size-4 animate-spin' />
            {t('asn.upstreamGraph.loading')}
          </div>
        ) : !layout || (data?.edges.length ?? 0) === 0 ? (
          <div className='rounded-xl border border-dashed py-10 text-center text-sm text-muted-foreground'>
            {t('asn.upstreamGraph.empty')}
          </div>
        ) : (
          <>
            <div className='overflow-x-auto rounded-xl border border-dashed bg-muted/20 p-3'>
              <svg
                ref={svgRef}
                xmlns='http://www.w3.org/2000/svg'
                width={Math.round(layout.width * zoom)}
                height={Math.round(layout.height * zoom)}
                viewBox={`0 0 ${layout.width} ${layout.height}`}
                className='mx-auto block select-none text-foreground'
                role='img'
                aria-label={t('asn.upstreamGraph.title')}
              >
                <defs>
                  <marker
                    id='upstream-arrow-default'
                    viewBox='0 0 10 8'
                    refX='9'
                    refY='4'
                    markerWidth='7'
                    markerHeight='7'
                    orient='auto'
                  >
                    <path d='M 0 0 L 10 4 L 0 8 z' fill='currentColor' />
                  </marker>
                  <marker
                    id='upstream-arrow-active'
                    viewBox='0 0 10 8'
                    refX='9'
                    refY='4'
                    markerWidth='8'
                    markerHeight='8'
                    orient='auto'
                  >
                    <path d='M 0 0 L 10 4 L 0 8 z' fill='#0284c7' />
                  </marker>
                </defs>

                {layout.groups.map((group) => {
                  const fill =
                    group.kind === 'origins'
                      ? 'rgba(74, 222, 128, 0.34)'
                      : group.kind === 'tier1'
                        ? 'rgba(125, 211, 252, 0.34)'
                        : 'rgba(148, 163, 184, 0.14)';
                  const stroke =
                    group.kind === 'origins'
                      ? 'rgba(34, 197, 94, 0.5)'
                      : group.kind === 'tier1'
                        ? 'rgba(56, 189, 248, 0.5)'
                        : 'rgba(148, 163, 184, 0.4)';
                  return (
                    <g key={group.key}>
                      <rect
                        x={group.x}
                        y={group.y}
                        width={group.width}
                        height={group.height}
                        rx={4}
                        fill={fill}
                        stroke={stroke}
                        strokeWidth={1}
                      />
                      <text
                        x={group.x + group.width / 2}
                        y={group.y + 20}
                        textAnchor='middle'
                        fill='currentColor'
                        fontSize={13}
                        fontFamily='ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace'
                        opacity={0.85}
                      >
                        {groupLabel(group.kind)}
                      </text>
                    </g>
                  );
                })}

                {layout.edges.map(({ key, edge, d }) => {
                  const isHighlighted =
                    activeHighlight.highlightedEdges.has(key);
                  const opacity = activeHighlight.hasActive
                    ? isHighlighted
                      ? 1
                      : 0.18
                    : 0.85;
                  return (
                    <path
                      key={key}
                      d={d}
                      fill='none'
                      stroke={isHighlighted ? '#0284c7' : 'currentColor'}
                      strokeWidth={isHighlighted ? 2 : 1.25}
                      strokeOpacity={opacity}
                      strokeDasharray={
                        edge.kind === 'observed' ? '4 3' : undefined
                      }
                      markerEnd={
                        isHighlighted
                          ? 'url(#upstream-arrow-active)'
                          : 'url(#upstream-arrow-default)'
                      }
                      className='cursor-pointer transition-opacity duration-150'
                      onMouseEnter={() => setHoveredEdgeKey(key)}
                      onMouseLeave={() => setHoveredEdgeKey(null)}
                    />
                  );
                })}

                {layout.nodes.map(({ node, x, y, width, height }) => {
                  const isOrigin = node.role === 'origin';
                  const isHighlighted = activeHighlight.highlightedNodes.has(
                    node.asn,
                  );
                  const dimmed = activeHighlight.hasActive && !isHighlighted;
                  const borderColor = isOrigin
                    ? '#16a34a'
                    : node.is_tier1
                      ? '#0369a1'
                      : '#475569';
                  return (
                    <g
                      key={node.asn}
                      opacity={dimmed ? 0.4 : 1}
                      className={
                        isOrigin
                          ? 'transition-opacity duration-150'
                          : 'cursor-pointer transition-opacity duration-150'
                      }
                      onMouseEnter={() => setHoveredAsn(node.asn)}
                      onMouseLeave={() => setHoveredAsn(null)}
                      onClick={() => {
                        if (!isOrigin) {
                          onSelectAsn(node.asn);
                        }
                      }}
                    >
                      <rect
                        x={x}
                        y={y}
                        width={width}
                        height={height}
                        rx={2}
                        className='fill-background'
                        stroke={borderColor}
                        strokeWidth={isHighlighted ? 3.5 : 2.5}
                      />
                      <text
                        x={x + width / 2}
                        y={y + 24}
                        textAnchor='middle'
                        fill='currentColor'
                        fontSize={16}
                        fontWeight={700}
                        fontFamily='ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace'
                      >
                        {`AS${node.asn}`}
                      </text>
                      <text
                        x={x + width / 2}
                        y={y + 42}
                        textAnchor='middle'
                        fill='currentColor'
                        fontSize={11.5}
                        fontFamily='ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace'
                        opacity={0.78}
                      >
                        {node.short_name || node.name || `AS${node.asn}`}
                      </text>
                    </g>
                  );
                })}
              </svg>
            </div>

            <div className='flex flex-wrap items-center justify-between gap-2 text-xs text-muted-foreground'>
              <div>
                {hoveredNodeDetail ? (
                  <span className='font-medium text-foreground'>
                    {`AS${hoveredNodeDetail.asn}`} · {hoveredNodeDetail.name}
                    {hoveredNodeDetail.country
                      ? ` (${hoveredNodeDetail.country})`
                      : ''}
                    {hoveredNodeDetail.path_hits > 0
                      ? ` · ${t('asn.upstreamGraph.edgeHits', { count: hoveredNodeDetail.path_hits })}`
                      : ''}
                  </span>
                ) : hoveredEdgeDetail ? (
                  <span className='font-medium text-foreground'>
                    {`AS${hoveredEdgeDetail.from} → AS${hoveredEdgeDetail.to}`}{' '}
                    ·{' '}
                    {t('asn.upstreamGraph.edgeHits', {
                      count: hoveredEdgeDetail.path_hits,
                    })}
                    {hoveredEdgeDetail.observed_rtt_ms
                      ? ` · ${hoveredEdgeDetail.observed_rtt_ms.toFixed(1)} ms`
                      : ''}
                  </span>
                ) : (
                  <span>{t('asn.upstreamGraph.clickHint')}</span>
                )}
              </div>
              {data?.attribution ? <span>{data.attribution}</span> : null}
            </div>
          </>
        )}
      </CardContent>
    </Card>
  );
}
