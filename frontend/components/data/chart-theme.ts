'use client';

import { useEffect, useState } from 'react';
import { useTheme } from 'next-themes';

import { useOptionalCustomTheme } from '@/lib/theme/context';

export interface ChartThemePalette {
  primary: string;
  destructive: string;
  foreground: string;
  mutedForeground: string;
  border: string;
  popover: string;
  popoverForeground: string;
  series: string[];
}

const DEFAULT_PALETTE: ChartThemePalette = {
  primary: '#4f46e5',
  destructive: '#dc2626',
  foreground: '#0f172a',
  mutedForeground: '#64748b',
  border: '#e2e8f0',
  popover: '#ffffff',
  popoverForeground: '#0f172a',
  series: [
    '#4f46e5',
    '#0d9488',
    '#d97706',
    '#2563eb',
    '#7c3aed',
    '#db2777',
    '#059669',
    '#ea580c',
  ],
};

const HEX_COLOR_RE = /^#(?:[0-9a-f]{3}|[0-9a-f]{4}|[0-9a-f]{6}|[0-9a-f]{8})$/i;
const LEGACY_RGB_RE =
  /^rgba?\(\s*\d+(?:\.\d+)?\s*,\s*\d+(?:\.\d+)?\s*,\s*\d+(?:\.\d+)?(?:\s*,\s*(?:0|1|0?\.\d+|\d+(?:\.\d+)?%?))?\s*\)$/i;
const OKLCH_RE =
  /^oklch\(\s*([^\s/]+)\s+([^\s/]+)\s+([^\s/]+)(?:\s*\/\s*([^\s)]+))?\s*\)$/i;
const SPACE_RGB_RE =
  /^rgba?\(\s*([^\s/]+)\s+([^\s/]+)\s+([^\s/]+)(?:\s*\/\s*([^\s)]+))?\s*\)$/i;
const HSL_RE =
  /^hsla?\(\s*([^\s/,]+)(?:[\s,]+([^\s/,]+))(?:[\s,]+([^\s/,)]+))(?:\s*[,/]\s*([^\s)]+))?\s*\)$/i;

function clampUnit(value: number) {
  return Math.min(1, Math.max(0, value));
}

function clampByte(value: number) {
  return Math.round(clampUnit(value / 255) * 255);
}

function parseNumericToken(token: string, percentageScale: number): number {
  const normalized = token.trim().toLowerCase();
  if (normalized === 'none') return 0;
  if (normalized.endsWith('%')) {
    const pct = Number.parseFloat(normalized.slice(0, -1));
    return Number.isFinite(pct) ? (pct / 100) * percentageScale : Number.NaN;
  }
  const num = Number.parseFloat(normalized);
  return Number.isFinite(num) ? num : Number.NaN;
}

function parseHueDegrees(token: string): number {
  const normalized = token.trim().toLowerCase();
  if (normalized === 'none') return 0;
  if (normalized.endsWith('deg')) {
    return Number.parseFloat(normalized.slice(0, -3));
  }
  if (normalized.endsWith('rad')) {
    return (Number.parseFloat(normalized.slice(0, -3)) * 180) / Math.PI;
  }
  if (normalized.endsWith('grad')) {
    return Number.parseFloat(normalized.slice(0, -4)) * 0.9;
  }
  if (normalized.endsWith('turn')) {
    return Number.parseFloat(normalized.slice(0, -4)) * 360;
  }
  return Number.parseFloat(normalized);
}

function formatSrgbOutput(r: number, g: number, b: number, alpha: number) {
  const rr = clampByte(r);
  const gg = clampByte(g);
  const bb = clampByte(b);
  const clampedAlpha = clampUnit(alpha);
  if (clampedAlpha >= 1) {
    return `#${[rr, gg, bb]
      .map((channel) => channel.toString(16).padStart(2, '0'))
      .join('')}`;
  }
  return `rgba(${rr}, ${gg}, ${bb}, ${Number(clampedAlpha.toFixed(3))})`;
}

function linearChannelToSrgbByte(linear: number) {
  const clamped = clampUnit(linear);
  const gamma =
    clamped <= 0.0031308
      ? 12.92 * clamped
      : 1.055 * Math.pow(clamped, 1 / 2.4) - 0.055;
  return Math.round(clampUnit(gamma) * 255);
}

function convertOklchToZRenderColor(input: string): string | null {
  const match = input.match(OKLCH_RE);
  if (!match) return null;

  const L = clampUnit(parseNumericToken(match[1], 1));
  const C = Math.max(0, parseNumericToken(match[2], 0.4));
  const H = parseHueDegrees(match[3]);
  const A =
    match[4] !== undefined ? clampUnit(parseNumericToken(match[4], 1)) : 1;

  if ([L, C, H, A].some((value) => Number.isNaN(value))) {
    return null;
  }

  const hRad = (H * Math.PI) / 180;
  const a = C * Math.cos(hRad);
  const b = C * Math.sin(hRad);

  const l_ = L + 0.3963377774 * a + 0.2158037573 * b;
  const m_ = L - 0.1055613458 * a - 0.0638541728 * b;
  const s_ = L - 0.0894841775 * a - 1.291485548 * b;

  const l = l_ * l_ * l_;
  const m = m_ * m_ * m_;
  const s = s_ * s_ * s_;

  const lr = +4.0767416621 * l - 3.3077115913 * m + 0.2309699292 * s;
  const lg = -1.2684380046 * l + 2.6097574011 * m - 0.3413193965 * s;
  const lb = -0.0041960863 * l - 0.7034186147 * m + 1.707614701 * s;

  return formatSrgbOutput(
    linearChannelToSrgbByte(lr),
    linearChannelToSrgbByte(lg),
    linearChannelToSrgbByte(lb),
    A,
  );
}

function convertSpaceRgbToZRenderColor(input: string): string | null {
  const match = input.match(SPACE_RGB_RE);
  if (!match) return null;

  const r = parseNumericToken(match[1], 255);
  const g = parseNumericToken(match[2], 255);
  const b = parseNumericToken(match[3], 255);
  const a =
    match[4] !== undefined ? clampUnit(parseNumericToken(match[4], 1)) : 1;

  if ([r, g, b, a].some((value) => Number.isNaN(value))) {
    return null;
  }
  return formatSrgbOutput(r, g, b, a);
}

function convertHslToZRenderColor(input: string): string | null {
  const match = input.match(HSL_RE);
  if (!match) return null;

  const hueRaw = parseHueDegrees(match[1]);
  const s = clampUnit(parseNumericToken(match[2], 1));
  const l = clampUnit(parseNumericToken(match[3], 1));
  const a =
    match[4] !== undefined ? clampUnit(parseNumericToken(match[4], 1)) : 1;

  if ([hueRaw, s, l, a].some((value) => Number.isNaN(value))) {
    return null;
  }

  const h = (((hueRaw % 360) + 360) % 360) / 360;
  if (s === 0) {
    const gray = Math.round(l * 255);
    return formatSrgbOutput(gray, gray, gray, a);
  }

  const q = l < 0.5 ? l * (1 + s) : l + s - l * s;
  const p = 2 * l - q;
  const hue2rgb = (t: number) => {
    let wrapped = t;
    if (wrapped < 0) wrapped += 1;
    if (wrapped > 1) wrapped -= 1;
    if (wrapped < 1 / 6) return p + (q - p) * 6 * wrapped;
    if (wrapped < 1 / 2) return q;
    if (wrapped < 2 / 3) return p + (q - p) * (2 / 3 - wrapped) * 6;
    return p;
  };

  return formatSrgbOutput(
    Math.round(hue2rgb(h + 1 / 3) * 255),
    Math.round(hue2rgb(h) * 255),
    Math.round(hue2rgb(h - 1 / 3) * 255),
    a,
  );
}

function convertViaCanvas(
  color: string,
  ctx?: CanvasRenderingContext2D | null,
): string | null {
  if (!ctx) return null;
  const hasCssSupports =
    typeof CSS !== 'undefined' && typeof CSS.supports === 'function';
  if (hasCssSupports && !CSS.supports('color', color)) {
    return null;
  }
  if (!hasCssSupports) {
    ctx.fillStyle = '#010203';
    ctx.fillStyle = color;
    if (
      ctx.fillStyle === '#010203' &&
      color.toLowerCase() !== '#010203' &&
      color.toLowerCase() !== 'rgb(1, 2, 3)'
    ) {
      return null;
    }
  }
  ctx.clearRect(0, 0, 1, 1);
  ctx.fillStyle = color;
  ctx.fillRect(0, 0, 1, 1);
  const [r, g, b, alphaByte] = ctx.getImageData(0, 0, 1, 1).data;
  if (
    alphaByte === 0 &&
    !/\/\s*0+%?\s*\)|,\s*0+\s*\)|transparent/i.test(color)
  ) {
    return null;
  }
  return formatSrgbOutput(r, g, b, alphaByte / 255);
}

/**
 * Normalizes CSS Color 4 values (such as `oklch(...)` and space-separated `hsl(...)`)
 * into `#rrggbb` or `rgba(...)` strings that ECharts/ZRender `liftColor` can parse on hover.
 */
export function resolveChartColor(
  rawColor: string,
  fallback: string,
  ctx?: CanvasRenderingContext2D | null,
): string {
  const trimmed = rawColor.trim();
  if (!trimmed) return fallback;

  if (HEX_COLOR_RE.test(trimmed) || LEGACY_RGB_RE.test(trimmed)) {
    return trimmed;
  }

  return (
    convertOklchToZRenderColor(trimmed) ??
    convertSpaceRgbToZRenderColor(trimmed) ??
    convertHslToZRenderColor(trimmed) ??
    convertViaCanvas(trimmed, ctx) ??
    fallback
  );
}

/**
 * Resolves OpenFlare CSS custom properties into ZRender-safe sRGB strings ECharts canvas can render and emphasize.
 */
export function useChartThemePalette(): ChartThemePalette {
  const { resolvedTheme } = useTheme();
  const customTheme = useOptionalCustomTheme();
  const currentTheme = customTheme?.currentTheme ?? null;
  const [palette, setPalette] = useState<ChartThemePalette>(DEFAULT_PALETTE);

  useEffect(() => {
    if (typeof window === 'undefined') return;
    const style = window.getComputedStyle(document.documentElement);
    const canvas = document.createElement('canvas');
    canvas.width = 1;
    canvas.height = 1;
    const ctx = canvas.getContext('2d', { willReadFrequently: true });
    const mode = resolvedTheme === 'dark' ? 'dark' : 'light';
    const themeColors = currentTheme?.colors[mode];

    const readColor = (token: string, fallback: string) => {
      const raw =
        themeColors?.[token] || style.getPropertyValue(`--${token}`).trim();
      return resolveChartColor(raw, fallback, ctx);
    };

    const primary = readColor('primary', DEFAULT_PALETTE.primary);
    const chart1 = readColor('chart-1', DEFAULT_PALETTE.series[0]);
    const chart2 = readColor('chart-2', DEFAULT_PALETTE.series[1]);
    const chart3 = readColor('chart-3', DEFAULT_PALETTE.series[2]);
    const chart4 = readColor('chart-4', DEFAULT_PALETTE.series[3]);
    const chart5 = readColor('chart-5', DEFAULT_PALETTE.series[4]);

    setPalette({
      primary,
      destructive: readColor('destructive', DEFAULT_PALETTE.destructive),
      foreground: readColor('foreground', DEFAULT_PALETTE.foreground),
      mutedForeground: readColor(
        'muted-foreground',
        DEFAULT_PALETTE.mutedForeground,
      ),
      border: readColor('border', DEFAULT_PALETTE.border),
      popover: readColor('popover', DEFAULT_PALETTE.popover),
      popoverForeground: readColor(
        'popover-foreground',
        DEFAULT_PALETTE.popoverForeground,
      ),
      series: [
        chart1,
        chart2,
        chart3,
        chart4,
        chart5,
        primary,
        '#0ea5e9',
        '#10b981',
      ],
    });
  }, [resolvedTheme, currentTheme]);

  return palette;
}
