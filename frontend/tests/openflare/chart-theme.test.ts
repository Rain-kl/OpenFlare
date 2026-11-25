import { describe, expect, it } from 'vitest';

import { resolveChartColor } from '@/components/data/chart-theme';
import themes from '@/lib/theme/themes.json';

const ZRENDER_SAFE_RE =
  /^(?:#[0-9a-f]{6}|rgba\(\d{1,3},\s*\d{1,3},\s*\d{1,3},\s*(?:0|1|0?\.\d+)\))$/i;

const CHART_THEME_TOKENS = [
  'primary',
  'destructive',
  'foreground',
  'muted-foreground',
  'border',
  'popover',
  'popover-foreground',
  'chart-1',
  'chart-2',
  'chart-3',
  'chart-4',
  'chart-5',
] as const;

describe('resolveChartColor', () => {
  it('converts oklch colors (decimal, percentage, angle units, and alpha) into zrender-safe colors', () => {
    expect(resolveChartColor('oklch(0.646 0.222 41.116)', '#000000')).toBe(
      '#f54900',
    );
    expect(resolveChartColor('oklch(51.1% 0.262 276.966)', '#000000')).toBe(
      '#4f39f6',
    );
    expect(resolveChartColor('oklch(58.5% 51% 277.117deg)', '#000000')).toMatch(
      ZRENDER_SAFE_RE,
    );
    expect(resolveChartColor('oklch(0.6 0.15 0.5turn)', '#000000')).toMatch(
      ZRENDER_SAFE_RE,
    );
    expect(resolveChartColor('oklch(1 0 0 / 10%)', '#000000')).toBe(
      'rgba(255, 255, 255, 0.1)',
    );
  });

  it('converts CSS Color 4 space-separated hsl/rgb values and preserves hex/rgba', () => {
    expect(resolveChartColor('hsl(240 5.9% 10%)', '#000000')).toBe('#18181b');
    expect(resolveChartColor('hsl(217.2 91.2% 59.8% / 50%)', '#000000')).toBe(
      'rgba(59, 130, 246, 0.5)',
    );
    expect(resolveChartColor('rgb(56 189 248 / 25%)', '#000000')).toBe(
      'rgba(56, 189, 248, 0.25)',
    );
    expect(resolveChartColor('#4f46e5', '#000000')).toBe('#4f46e5');
    expect(resolveChartColor('rgba(15, 23, 42, 0.92)', '#000000')).toBe(
      'rgba(15, 23, 42, 0.92)',
    );
  });

  it('falls back safely for empty or unsupported strings', () => {
    expect(resolveChartColor('', '#4f46e5')).toBe('#4f46e5');
    expect(resolveChartColor('not-a-color', '#4f46e5')).toBe('#4f46e5');
  });

  it('converts every chart palette color in themes.json without leaving oklch values', () => {
    for (const theme of themes) {
      for (const mode of ['light', 'dark'] as const) {
        const modeColors = theme.colors[mode] as Record<
          string,
          string | undefined
        >;
        for (const token of CHART_THEME_TOKENS) {
          const raw = modeColors[token];
          if (!raw) continue;
          const resolved = resolveChartColor(raw, '#000000');
          expect(resolved).not.toContain('oklch');
          expect(resolved).toMatch(ZRENDER_SAFE_RE);
        }
      }
    }
  });
});
