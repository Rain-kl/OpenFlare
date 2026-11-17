'use client';

import { useEffect, useState } from 'react';
import { useTheme } from 'next-themes';
import { useCustomTheme } from '@/lib/theme/context';

// Convert theme colors (including OKLCH) to hex for both Canvas and ECharts.
export function useMapPalette() {
  const { resolvedTheme } = useTheme();
  const { currentTheme } = useCustomTheme();
  const [palette, setPalette] = useState<{
    background: string;
    surface: string;
    border: string;
    land: string;
    source: string;
    label: string;
  } | null>(null);

  useEffect(() => {
    const context = document.createElement('canvas').getContext('2d', {
      willReadFrequently: true,
    });
    if (!context) return;
    const colors =
      currentTheme?.colors[resolvedTheme === 'dark' ? 'dark' : 'light'];
    const style = getComputedStyle(document.documentElement);
    const color = (key: string) => {
      context.clearRect(0, 0, 1, 1);
      context.fillStyle =
        colors?.[key] || style.getPropertyValue(`--${key}`).trim();
      context.fillRect(0, 0, 1, 1);
      const rgb = context.getImageData(0, 0, 1, 1).data;
      return `#${Array.from(rgb.slice(0, 3), (value) => value.toString(16).padStart(2, '0')).join('')}`;
    };
    setPalette({
      background: color('background'),
      surface: color('muted'),
      border: color('border'),
      land: color('muted-foreground'),
      source: color('primary'),
      label: color('foreground'),
    });
  }, [currentTheme, resolvedTheme]);

  return palette;
}
