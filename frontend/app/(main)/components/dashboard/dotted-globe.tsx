'use client';
import { useCallback, useEffect, useMemo, useRef } from 'react';
import { useTranslations } from 'next-intl';
import { useMapPalette } from './use-map-palette';
import Link from 'next/link';
import type { Coordinate, CountryHeat } from './country-heat';
import world from './data/world-geo.json';
type GlobeNode = {
  id: number;
  name: string;
  coordinates: Coordinate;
  tone: 'healthy' | 'warning' | 'danger';
  detail: string;
};
const radians = Math.PI / 180;
const AUTO_ROTATE_DEG_PER_SEC = 4.5;
const ORBIT_RING_RAD_PER_SEC = 0.035;
const DRAG_RESUME_DELAY_MS = 900;
const RESUME_RAMP_MS = 700;

function normalizeLongitude(lon: number) {
  return ((((lon + 180) % 360) + 360) % 360) - 180;
}

function clampLatitude(lat: number) {
  return Math.max(-75, Math.min(75, lat));
}

function inside(point: Coordinate, ring: number[][]) {
  let result = false;
  for (let i = 0, j = ring.length - 1; i < ring.length; j = i++) {
    const a = ring[i],
      b = ring[j];
    if (
      a[1] > point[1] !== b[1] > point[1] &&
      point[0] < ((b[0] - a[0]) * (point[1] - a[1])) / (b[1] - a[1]) + a[0]
    )
      result = !result;
  }
  return result;
}
// Sample the bundled polygons once, including their holes.
const landCenters = new Map<
  string,
  { coordinate: Coordinate; count: number }
>();
const landDots = world.features.flatMap((feature) => {
  const country = feature.properties.name;
  const polygons = (
    feature.geometry.type === 'Polygon'
      ? [feature.geometry.coordinates]
      : feature.geometry.coordinates
  ) as number[][][][];
  return polygons.flatMap((polygon) => {
    const ring = polygon[0];
    const xs = ring.map((p) => p[0]),
      ys = ring.map((p) => p[1]);
    const minX = Math.min(...xs),
      maxX = Math.max(...xs);
    const minY = Math.min(...ys),
      maxY = Math.max(...ys);
    const dots: { coordinate: Coordinate; country: string }[] = [];
    for (let lat = Math.ceil(minY / 1.6) * 1.6; lat <= maxY; lat += 1.6) {
      const step = 1.6 / Math.max(0.2, Math.cos(lat * radians));
      for (let lon = Math.ceil(minX / step) * step; lon <= maxX; lon += step) {
        const coordinate: Coordinate = [lon, lat];
        if (
          inside(coordinate, ring) &&
          !polygon.slice(1).some((hole) => inside(coordinate, hole))
        )
          dots.push({ coordinate, country });
      }
    }
    // Use the main landmass so overseas territories cannot set the initial view.
    if (dots.length > (landCenters.get(country)?.count ?? 0)) {
      const sum: Coordinate = [0, 0];
      for (const { coordinate } of dots) {
        sum[0] += coordinate[0];
        sum[1] += coordinate[1];
      }
      landCenters.set(country, {
        coordinate: [sum[0] / dots.length, sum[1] / dots.length],
        count: dots.length,
      });
    }
    return dots;
  });
});

// Precompute faint spherical graticule arcs so rotation remains legible across oceans.
const graticuleLines: { points: Coordinate[]; isEquator: boolean }[] = (() => {
  const lines: { points: Coordinate[]; isEquator: boolean }[] = [];
  for (let lon = -150; lon <= 180; lon += 30) {
    const points: Coordinate[] = [];
    for (let lat = -78; lat <= 78; lat += 6) {
      points.push([lon, lat]);
    }
    lines.push({ points, isEquator: false });
  }
  for (const lat of [-60, -30, 0, 30, 60]) {
    const points: Coordinate[] = [];
    for (let lon = -180; lon <= 180; lon += 6) {
      points.push([lon, lat]);
    }
    lines.push({ points, isEquator: lat === 0 });
  }
  return lines;
})();

function project([lon, lat]: Coordinate, [longitude, latitude]: Coordinate) {
  const phi = lat * radians,
    lambda = (lon - longitude) * radians,
    tilt = latitude * radians;
  return {
    x: Math.cos(phi) * Math.sin(lambda),
    y: -(
      Math.sin(phi) * Math.cos(tilt) -
      Math.cos(phi) * Math.cos(lambda) * Math.sin(tilt)
    ),
    z:
      Math.sin(phi) * Math.sin(tilt) +
      Math.cos(phi) * Math.cos(lambda) * Math.cos(tilt),
  };
}

function getNodeColor(tone: GlobeNode['tone']) {
  return tone === 'healthy'
    ? '#10b981'
    : tone === 'warning'
      ? '#f59e0b'
      : 'var(--destructive)';
}

function getReducedMotion() {
  return (
    typeof window !== 'undefined' &&
    typeof window.matchMedia === 'function' &&
    window.matchMedia('(prefers-reduced-motion: reduce)').matches
  );
}

type GlobePalette = {
  background: string;
  surface: string;
  border: string;
  land: string;
  source: string;
  label: string;
};

function drawGlobeHalos(
  ctx: CanvasRenderingContext2D,
  center: number,
  radius: number,
  borderColor: string,
  orbitAngle: number,
) {
  for (const scale of [1.14, 1.07]) {
    ctx.beginPath();
    ctx.arc(center, center, radius * scale, 0, Math.PI * 2);
    ctx.fillStyle = `${borderColor}30`;
    ctx.fill();
  }

  // Subtle outer dashed orbital ring slowly rotating around the halo.
  ctx.save();
  ctx.translate(center, center);
  ctx.rotate(orbitAngle);
  ctx.beginPath();
  ctx.arc(0, 0, radius * 1.14, 0, Math.PI * 2);
  ctx.strokeStyle = borderColor;
  ctx.globalAlpha = 0.45;
  ctx.lineWidth = 1;
  ctx.setLineDash([4, 6]);
  ctx.stroke();
  ctx.restore();
}

function drawGraticule(
  ctx: CanvasRenderingContext2D,
  center: number,
  radius: number,
  view: Coordinate,
  borderColor: string,
) {
  ctx.save();
  ctx.strokeStyle = borderColor;
  ctx.lineWidth = 0.7;
  ctx.setLineDash([2, 3]);
  for (const line of graticuleLines) {
    ctx.globalAlpha = line.isEquator ? 0.42 : 0.24;
    ctx.beginPath();
    let drawing = false;
    for (const coord of line.points) {
      const p = project(coord, view);
      if (p.z <= 0.02) {
        drawing = false;
        continue;
      }
      const px = center + p.x * radius;
      const py = center + p.y * radius;
      if (!drawing) {
        ctx.moveTo(px, py);
        drawing = true;
      } else {
        ctx.lineTo(px, py);
      }
    }
    ctx.stroke();
  }
  ctx.restore();
}

function drawLandAndHeat(
  ctx: CanvasRenderingContext2D,
  size: number,
  center: number,
  radius: number,
  view: Coordinate,
  heatMap: Map<string, CountryHeat>,
  palette: GlobePalette,
) {
  for (const dot of landDots) {
    const p = project(dot.coordinate, view);
    if (p.z <= 0) continue;
    const heat = heatMap.get(dot.country);
    const limb = Math.min(1, p.z / 0.26);
    ctx.globalAlpha = heat
      ? (0.48 + heat.intensity * 0.52) * (0.28 + 0.72 * limb)
      : (0.24 + p.z * 0.42) * (0.35 + 0.65 * limb);
    ctx.fillStyle = heat ? palette.source : palette.land;
    const dotScale = heat ? 1.25 + heat.intensity * 0.8 : 1;
    ctx.beginPath();
    ctx.arc(
      center + p.x * radius,
      center + p.y * radius,
      dotScale * Math.max(0.55, size * 0.0021 * (0.5 + p.z * 0.5)),
      0,
      Math.PI * 2,
    );
    ctx.fill();
  }

  for (const { name, coordinate, intensity } of heatMap.values()) {
    if (!coordinate || landCenters.has(name)) continue;
    const p = project(coordinate, view);
    if (p.z <= 0) continue;
    const limb = Math.min(1, p.z / 0.26);
    ctx.globalAlpha = 0.25 + 0.75 * limb;
    ctx.beginPath();
    ctx.arc(
      center + p.x * radius,
      center + p.y * radius,
      2.5 + intensity * 3,
      0,
      Math.PI * 2,
    );
    ctx.strokeStyle = palette.source;
    ctx.lineWidth = 1.5;
    ctx.stroke();
  }
  ctx.globalAlpha = 1;
}

export function DottedGlobe({
  nodes,
  countryHeat,
}: {
  nodes: GlobeNode[];
  countryHeat: Map<string, CountryHeat>;
}) {
  const t = useTranslations('dashboard.map');
  const palette = useMapPalette();
  const canvasRef = useRef<HTMLCanvasElement>(null);
  const canvasMetricsRef = useRef({ size: 0, ratio: 1 });
  const initialView = useMemo<Coordinate>(() => {
    for (const { name, coordinate } of countryHeat.values()) {
      const center = landCenters.get(name)?.coordinate ?? coordinate;
      if (center) return center;
    }
    return [105, 20];
  }, [countryHeat]);

  const viewRef = useRef<Coordinate>(initialView);
  const orbitAngleRef = useRef(0);
  const dragRef = useRef<{
    x: number;
    y: number;
    view: Coordinate;
    lastX: number;
    lastY: number;
    lastTime: number;
  } | null>(null);
  const velocityRef = useRef<Coordinate>([0, 0]);
  const pauseUntilRef = useRef(0);
  const hasInteractedRef = useRef(false);
  const hasSyncedInitialHeatRef = useRef(countryHeat.size > 0);
  const hoveredNodeIdsRef = useRef<Set<number>>(new Set());
  const focusedNodeIdsRef = useRef<Set<number>>(new Set());
  const nodeElementsRef = useRef<Map<number, HTMLAnchorElement>>(new Map());
  const nodesRef = useRef(nodes);
  const paletteRef = useRef(palette);
  const countryHeatRef = useRef(countryHeat);

  const syncNodeMarkers = useCallback((view: Coordinate) => {
    for (const node of nodesRef.current) {
      const el = nodeElementsRef.current.get(node.id);
      if (!el) continue;
      const p = project(node.coordinates, view);
      if (p.z <= 0) {
        el.style.visibility = 'hidden';
        el.style.opacity = '0';
        el.style.pointerEvents = 'none';
        el.tabIndex = -1;
        el.setAttribute('aria-hidden', 'true');
        continue;
      }
      const horizonOpacity = Math.min(1, p.z / 0.22);
      const scale = 0.82 + 0.18 * Math.min(1, p.z / 0.35);
      el.style.visibility = 'visible';
      el.style.opacity = horizonOpacity.toFixed(3);
      el.style.pointerEvents = 'auto';
      el.style.left = `${50 + p.x * 43}%`;
      el.style.top = `${50 + p.y * 43}%`;
      el.style.transform = `scale(${scale.toFixed(3)})`;
      el.tabIndex = 0;
      el.removeAttribute('aria-hidden');
    }
  }, []);

  const drawFrame = useCallback(() => {
    const canvas = canvasRef.current;
    const activePalette = paletteRef.current;
    if (!canvas || !activePalette) return;

    const size = canvas.clientWidth;
    const ratio = Math.min(window.devicePixelRatio || 1, 2);
    if (!size) return;

    const metrics = canvasMetricsRef.current;
    if (metrics.size !== size || metrics.ratio !== ratio) {
      canvas.width = size * ratio;
      canvas.height = size * ratio;
      metrics.size = size;
      metrics.ratio = ratio;
    }

    const ctx = canvas.getContext('2d');
    if (!ctx) return;

    const view = viewRef.current;
    ctx.setTransform(ratio, 0, 0, ratio, 0, 0);
    ctx.clearRect(0, 0, size, size);

    const center = size / 2;
    const radius = size * 0.43;

    drawGlobeHalos(
      ctx,
      center,
      radius,
      activePalette.border,
      orbitAngleRef.current,
    );

    const gradient = ctx.createRadialGradient(
      center - radius * 0.5,
      center - radius * 0.6,
      0,
      center,
      center,
      radius,
    );
    gradient.addColorStop(0, activePalette.background);
    gradient.addColorStop(1, activePalette.surface);
    ctx.beginPath();
    ctx.arc(center, center, radius, 0, Math.PI * 2);
    ctx.fillStyle = gradient;
    ctx.fill();

    ctx.beginPath();
    ctx.arc(center, center, radius, 0, Math.PI * 2);
    ctx.strokeStyle = activePalette.border;
    ctx.globalAlpha = 0.35;
    ctx.lineWidth = 1;
    ctx.stroke();

    drawGraticule(ctx, center, radius, view, activePalette.border);
    drawLandAndHeat(
      ctx,
      size,
      center,
      radius,
      view,
      countryHeatRef.current,
      activePalette,
    );
  }, []);

  const renderNow = useCallback(() => {
    drawFrame();
    syncNodeMarkers(viewRef.current);
  }, [drawFrame, syncNodeMarkers]);

  useEffect(() => {
    nodesRef.current = nodes;
    paletteRef.current = palette;
    countryHeatRef.current = countryHeat;
    const validIds = new Set(nodes.map((n) => n.id));
    for (const id of hoveredNodeIdsRef.current) {
      if (!validIds.has(id)) hoveredNodeIdsRef.current.delete(id);
    }
    for (const id of focusedNodeIdsRef.current) {
      if (!validIds.has(id)) focusedNodeIdsRef.current.delete(id);
    }
    if (
      countryHeat.size > 0 &&
      !hasSyncedInitialHeatRef.current &&
      !hasInteractedRef.current
    ) {
      viewRef.current = initialView;
      hasSyncedInitialHeatRef.current = true;
    }
    renderNow();
  }, [countryHeat, initialView, nodes, palette, renderNow]);

  useEffect(() => {
    const canvas = canvasRef.current;
    if (!canvas || !palette) return;

    const observer = new ResizeObserver(renderNow);
    observer.observe(canvas);

    let frameId = 0;
    let lastTime = performance.now();
    let reducedMotion = getReducedMotion();

    const mediaQuery =
      typeof window.matchMedia === 'function'
        ? window.matchMedia('(prefers-reduced-motion: reduce)')
        : null;

    const stopLoop = () => {
      if (frameId) {
        window.cancelAnimationFrame(frameId);
        frameId = 0;
      }
    };

    const tick = (now: number) => {
      const dt = Math.min((now - lastTime) / 1000, 0.1);
      lastTime = now;

      if (!reducedMotion && !document.hidden && dragRef.current === null) {
        const [vx, vy] = velocityRef.current;
        if (Math.abs(vx) > 0.2 || Math.abs(vy) > 0.2) {
          viewRef.current = [
            normalizeLongitude(viewRef.current[0] + vx * dt),
            clampLatitude(viewRef.current[1] + vy * dt),
          ];
          const decay = Math.exp(-5.5 * dt);
          velocityRef.current = [vx * decay, vy * decay];
          renderNow();
        } else if (
          hoveredNodeIdsRef.current.size === 0 &&
          focusedNodeIdsRef.current.size === 0
        ) {
          velocityRef.current = [0, 0];
          const sinceResume = now - pauseUntilRef.current;
          const ramp =
            sinceResume <= 0 ? 0 : Math.min(1, sinceResume / RESUME_RAMP_MS);
          if (ramp > 0) {
            viewRef.current = [
              normalizeLongitude(
                viewRef.current[0] - AUTO_ROTATE_DEG_PER_SEC * ramp * dt,
              ),
              viewRef.current[1],
            ];
            orbitAngleRef.current =
              (orbitAngleRef.current + ORBIT_RING_RAD_PER_SEC * ramp * dt) %
              (Math.PI * 2);
            renderNow();
          }
        }
      }

      if (!reducedMotion) {
        frameId = window.requestAnimationFrame(tick);
      }
    };

    const startLoop = () => {
      if (reducedMotion || frameId) return;
      lastTime = performance.now();
      frameId = window.requestAnimationFrame(tick);
    };

    const handleVisibility = () => {
      lastTime = performance.now();
    };

    const handleMotionChange = (event: MediaQueryListEvent) => {
      reducedMotion = event.matches;
      if (reducedMotion) {
        stopLoop();
      } else {
        startLoop();
      }
    };

    renderNow();
    startLoop();
    document.addEventListener('visibilitychange', handleVisibility);
    mediaQuery?.addEventListener('change', handleMotionChange);

    return () => {
      stopLoop();
      observer.disconnect();
      document.removeEventListener('visibilitychange', handleVisibility);
      mediaQuery?.removeEventListener('change', handleMotionChange);
    };
  }, [palette, renderNow]);

  const handleNodeHover = (id: number, active: boolean) => {
    if (active) {
      hoveredNodeIdsRef.current.add(id);
    } else {
      hoveredNodeIdsRef.current.delete(id);
      pauseUntilRef.current = performance.now() + 300;
    }
  };

  const handleNodeFocus = (id: number, active: boolean) => {
    if (active) {
      focusedNodeIdsRef.current.add(id);
    } else {
      focusedNodeIdsRef.current.delete(id);
      pauseUntilRef.current = performance.now() + 300;
    }
  };

  return (
    <div
      className='absolute inset-0 flex items-center justify-center overflow-hidden'
      style={{ containerType: 'size' }}
    >
      <div className='relative size-[min(100cqw,100cqh)]'>
        <canvas
          ref={canvasRef}
          className='aspect-square w-full touch-none cursor-grab active:cursor-grabbing focus-visible:outline focus-visible:outline-ring'
          role='img'
          tabIndex={0}
          aria-label={t('globeLabel')}
          onPointerDown={(e) => {
            e.currentTarget.setPointerCapture(e.pointerId);
            hasInteractedRef.current = true;
            velocityRef.current = [0, 0];
            dragRef.current = {
              x: e.clientX,
              y: e.clientY,
              view: viewRef.current,
              lastX: e.clientX,
              lastY: e.clientY,
              lastTime: performance.now(),
            };
          }}
          onPointerMove={(e) => {
            const start = dragRef.current;
            if (!start) return;
            const now = performance.now();
            const dt = Math.max((now - start.lastTime) / 1000, 0.008);
            const dx = e.clientX - start.lastX;
            const dy = e.clientY - start.lastY;
            velocityRef.current = [
              Math.max(-90, Math.min(90, (-dx * 0.4) / dt)),
              Math.max(-60, Math.min(60, (dy * 0.4) / dt)),
            ];
            start.lastX = e.clientX;
            start.lastY = e.clientY;
            start.lastTime = now;
            viewRef.current = [
              normalizeLongitude(start.view[0] - (e.clientX - start.x) * 0.4),
              clampLatitude(start.view[1] + (e.clientY - start.y) * 0.4),
            ];
            renderNow();
          }}
          onPointerUp={() => {
            dragRef.current = null;
            pauseUntilRef.current = performance.now() + DRAG_RESUME_DELAY_MS;
          }}
          onPointerCancel={() => {
            dragRef.current = null;
            velocityRef.current = [0, 0];
            pauseUntilRef.current = performance.now() + DRAG_RESUME_DELAY_MS;
          }}
          onKeyDown={(e) => {
            const delta: Record<string, Coordinate> = {
              ArrowLeft: [-10, 0],
              ArrowRight: [10, 0],
              ArrowUp: [0, 10],
              ArrowDown: [0, -10],
            };
            const change = delta[e.key];
            if (change) {
              e.preventDefault();
              hasInteractedRef.current = true;
              velocityRef.current = [0, 0];
              pauseUntilRef.current = performance.now() + DRAG_RESUME_DELAY_MS;
              const [lon, lat] = viewRef.current;
              viewRef.current = [
                normalizeLongitude(lon + change[0]),
                clampLatitude(lat + change[1]),
              ];
              renderNow();
            }
          }}
        />
        {nodes.map((node) => {
          const p = project(node.coordinates, initialView);
          const color = getNodeColor(node.tone);
          const visible = p.z > 0;
          return (
            <Link
              key={node.id}
              ref={(el) => {
                if (el) {
                  nodeElementsRef.current.set(node.id, el);
                } else {
                  nodeElementsRef.current.delete(node.id);
                }
              }}
              href={`/nodes/detail?id=${node.id}`}
              aria-label={`${node.name} · ${node.detail}`}
              aria-hidden={visible ? undefined : true}
              tabIndex={visible ? 0 : -1}
              title={`${node.name}\n${node.detail}`}
              onPointerEnter={() => handleNodeHover(node.id, true)}
              onPointerLeave={() => handleNodeHover(node.id, false)}
              onFocus={() => handleNodeFocus(node.id, true)}
              onBlur={() => handleNodeFocus(node.id, false)}
              className='absolute size-3 -translate-x-1/2 -translate-y-1/2 rounded-full border-2 border-background ring-4 ring-background/30 transition-transform duration-150 hover:scale-125 focus-visible:outline focus-visible:outline-ring'
              style={{
                left: `${50 + p.x * 43}%`,
                top: `${50 + p.y * 43}%`,
                backgroundColor: color,
                visibility: visible ? 'visible' : 'hidden',
                opacity: visible ? Math.min(1, p.z / 0.22) : 0,
                transform: visible
                  ? `scale(${(0.82 + 0.18 * Math.min(1, p.z / 0.35)).toFixed(3)})`
                  : undefined,
                pointerEvents: visible ? 'auto' : 'none',
              }}
            >
              <span
                aria-hidden='true'
                className='pointer-events-none absolute -inset-1 rounded-full animate-ping opacity-35 motion-reduce:animate-none'
                style={{
                  animationDuration: '2.8s',
                  backgroundColor: color,
                }}
              />
            </Link>
          );
        })}
      </div>
    </div>
  );
}
