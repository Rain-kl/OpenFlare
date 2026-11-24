'use client';

import { useEffect, useState } from 'react';
import { useTheme } from 'next-themes';

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

function readCssVar(
  style: CSSStyleDeclaration,
  name: string,
  fallback: string,
) {
  const value = style.getPropertyValue(name).trim();
  return value || fallback;
}

/**
 * Resolves OpenFlare CSS custom properties into runtime strings ECharts canvas can render.
 */
export function useChartThemePalette(): ChartThemePalette {
  const { resolvedTheme } = useTheme();
  const [palette, setPalette] = useState<ChartThemePalette>(DEFAULT_PALETTE);

  useEffect(() => {
    if (typeof window === 'undefined') return;
    const style = window.getComputedStyle(document.documentElement);
    const primary = readCssVar(style, '--primary', DEFAULT_PALETTE.primary);
    const chart1 = readCssVar(style, '--chart-1', DEFAULT_PALETTE.series[0]);
    const chart2 = readCssVar(style, '--chart-2', DEFAULT_PALETTE.series[1]);
    const chart3 = readCssVar(style, '--chart-3', DEFAULT_PALETTE.series[2]);
    const chart4 = readCssVar(style, '--chart-4', DEFAULT_PALETTE.series[3]);
    const chart5 = readCssVar(style, '--chart-5', DEFAULT_PALETTE.series[4]);

    setPalette({
      primary,
      destructive: readCssVar(
        style,
        '--destructive',
        DEFAULT_PALETTE.destructive,
      ),
      foreground: readCssVar(style, '--foreground', DEFAULT_PALETTE.foreground),
      mutedForeground: readCssVar(
        style,
        '--muted-foreground',
        DEFAULT_PALETTE.mutedForeground,
      ),
      border: readCssVar(style, '--border', DEFAULT_PALETTE.border),
      popover: readCssVar(style, '--popover', DEFAULT_PALETTE.popover),
      popoverForeground: readCssVar(
        style,
        '--popover-foreground',
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
  }, [resolvedTheme]);

  return palette;
}
