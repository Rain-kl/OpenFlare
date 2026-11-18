'use client';
import { useEffect, useMemo, useRef, useState } from 'react';
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
  const drag = useRef<{ x: number; y: number; view: Coordinate } | null>(null);
  const [manualView, setManualView] = useState<Coordinate | null>(null);
  const initialView = useMemo<Coordinate>(() => {
    for (const { name, coordinate } of countryHeat.values()) {
      const center = landCenters.get(name)?.coordinate ?? coordinate;
      if (center) return center;
    }
    return [105, 20];
  }, [countryHeat]);
  const view = manualView ?? initialView;
  useEffect(() => {
    const canvas = canvasRef.current;
    if (!canvas || !palette) return;
    const draw = () => {
      const size = canvas.clientWidth,
        ratio = Math.min(window.devicePixelRatio || 1, 2);
      canvas.width = size * ratio;
      canvas.height = size * ratio;
      const ctx = canvas.getContext('2d');
      if (!ctx || !size) return;
      ctx.scale(ratio, ratio);
      const center = size / 2,
        radius = size * 0.43;
      for (const scale of [1.14, 1.07]) {
        ctx.beginPath();
        ctx.arc(center, center, radius * scale, 0, Math.PI * 2);
        ctx.fillStyle = `${palette.border}30`;
        ctx.fill();
      }
      const gradient = ctx.createRadialGradient(
        center - radius * 0.5,
        center - radius * 0.6,
        0,
        center,
        center,
        radius,
      );
      gradient.addColorStop(0, palette.background);
      gradient.addColorStop(1, palette.surface);
      ctx.beginPath();
      ctx.arc(center, center, radius, 0, Math.PI * 2);
      ctx.fillStyle = gradient;
      ctx.fill();
      for (const dot of landDots) {
        const p = project(dot.coordinate, view);
        if (p.z <= 0) continue;
        const heat = countryHeat.get(dot.country);
        ctx.globalAlpha = heat
          ? 0.48 + heat.intensity * 0.52
          : 0.25 + p.z * 0.4;
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
      ctx.globalAlpha = 1;
      for (const { name, coordinate, intensity } of countryHeat.values()) {
        if (!coordinate || landCenters.has(name)) continue;
        const p = project(coordinate, view);
        if (p.z <= 0) continue;
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
    };
    draw();
    const observer = new ResizeObserver(draw);
    observer.observe(canvas);
    return () => observer.disconnect();
  }, [view, countryHeat, palette]);
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
            drag.current = { x: e.clientX, y: e.clientY, view };
          }}
          onPointerMove={(e) => {
            const start = drag.current;
            if (start)
              setManualView([
                start.view[0] - (e.clientX - start.x) * 0.4,
                Math.max(
                  -75,
                  Math.min(75, start.view[1] + (e.clientY - start.y) * 0.4),
                ),
              ]);
          }}
          onPointerUp={() => {
            drag.current = null;
          }}
          onPointerCancel={() => {
            drag.current = null;
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
              setManualView((previous) => {
                const [lon, lat] = previous ?? initialView;
                return [
                  lon + change[0],
                  Math.max(-75, Math.min(75, lat + change[1])),
                ];
              });
            }
          }}
        />
        {nodes.map((node) => {
          const p = project(node.coordinates, view);
          if (p.z <= 0) return null;
          return (
            <Link
              key={node.id}
              href={`/nodes/detail?id=${node.id}`}
              aria-label={`${node.name} · ${node.detail}`}
              title={`${node.name}\n${node.detail}`}
              className='absolute size-3 -translate-x-1/2 -translate-y-1/2 rounded-full border-2 border-background ring-4 ring-background/30 focus-visible:outline focus-visible:outline-ring'
              style={{
                left: `${50 + p.x * 43}%`,
                top: `${50 + p.y * 43}%`,
                backgroundColor:
                  node.tone === 'healthy'
                    ? '#10b981'
                    : node.tone === 'warning'
                      ? '#f59e0b'
                      : 'var(--destructive)',
              }}
            />
          );
        })}
      </div>
    </div>
  );
}
