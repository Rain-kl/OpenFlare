import * as React from 'react';
import { fireEvent, render, screen } from '@testing-library/react';
import { NextIntlClientProvider } from 'next-intl';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import zhCN from '@/messages/zh-CN.json';

import type { CountryHeat } from './country-heat';
import { DottedGlobe } from './dotted-globe';

vi.mock('next/link', () => ({
  default: React.forwardRef<
    HTMLAnchorElement,
    React.AnchorHTMLAttributes<HTMLAnchorElement> & { href: string }
  >(function MockLink({ children, href, ...props }, ref) {
    return (
      <a ref={ref} href={href} {...props}>
        {children}
      </a>
    );
  }),
}));

vi.mock('./use-map-palette', () => ({
  useMapPalette: () => ({
    background: '#ffffff',
    surface: '#f4f4f5',
    border: '#e4e4e7',
    land: '#a1a1aa',
    source: '#c2410c',
    label: '#18181b',
  }),
}));

function createCanvasContextMock() {
  return {
    arc: vi.fn(),
    beginPath: vi.fn(),
    clearRect: vi.fn(),
    createRadialGradient: vi.fn(() => ({ addColorStop: vi.fn() })),
    fill: vi.fn(),
    lineTo: vi.fn(),
    moveTo: vi.fn(),
    restore: vi.fn(),
    rotate: vi.fn(),
    save: vi.fn(),
    setLineDash: vi.fn(),
    setTransform: vi.fn(),
    stroke: vi.fn(),
    translate: vi.fn(),
    fillStyle: '',
    globalAlpha: 1,
    lineWidth: 1,
    strokeStyle: '',
  };
}

function renderGlobe(nodes = defaultNodes) {
  const heat = new Map<string, CountryHeat>([
    [
      'China',
      {
        name: 'China',
        value: 100,
        intensity: 1,
        hasBoundary: true,
        coordinate: [105, 35],
      },
    ],
  ]);

  return render(
    <NextIntlClientProvider
      locale='zh-CN'
      messages={zhCN}
      timeZone='Asia/Shanghai'
    >
      <DottedGlobe nodes={nodes} countryHeat={heat} />
    </NextIntlClientProvider>,
  );
}

const defaultNodes = [
  {
    id: 1,
    name: 'edge-cn',
    coordinates: [105, 35] as [number, number],
    tone: 'healthy' as const,
    detail: 'China · 100 req',
  },
];

describe('DottedGlobe', () => {
  let rafQueue: Array<{ id: number; cb: FrameRequestCallback }> = [];
  let nextRafId = 1;
  let reducedMotion = false;
  let nowValue = 1000;

  beforeEach(() => {
    rafQueue = [];
    nextRafId = 1;
    reducedMotion = false;
    nowValue = 1000;

    vi.spyOn(performance, 'now').mockImplementation(() => nowValue);
    vi.spyOn(window, 'requestAnimationFrame').mockImplementation((cb) => {
      const id = nextRafId++;
      rafQueue.push({ id, cb });
      return id;
    });
    vi.spyOn(window, 'cancelAnimationFrame').mockImplementation((id) => {
      rafQueue = rafQueue.filter((item) => item.id !== id);
    });
    vi.spyOn(HTMLCanvasElement.prototype, 'getContext').mockImplementation(
      () =>
        createCanvasContextMock() as unknown as ReturnType<
          HTMLCanvasElement['getContext']
        >,
    );
    Object.defineProperty(HTMLCanvasElement.prototype, 'clientWidth', {
      configurable: true,
      get: () => 400,
    });
    Object.defineProperty(window, 'matchMedia', {
      configurable: true,
      writable: true,
      value: vi.fn().mockImplementation((query: string) => ({
        matches:
          query === '(prefers-reduced-motion: reduce)' ? reducedMotion : false,
        media: query,
        addEventListener: vi.fn(),
        removeEventListener: vi.fn(),
      })),
    });
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  function stepFrames(steps: number, dtMs = 100) {
    for (let i = 0; i < steps; i++) {
      nowValue += dtMs;
      const pending = [...rafQueue];
      rafQueue = [];
      for (const item of pending) {
        item.cb(nowValue);
      }
    }
  }

  it('slowly auto-rotates the globe and pauses when hovering a node marker', () => {
    renderGlobe();

    const nodeLink = screen.getByRole('link', { name: /edge-cn/ });
    const initialLeft = Number.parseFloat(nodeLink.style.left);

    stepFrames(10, 100);
    const rotatedLeft = Number.parseFloat(nodeLink.style.left);
    expect(rotatedLeft).toBeGreaterThan(initialLeft);

    fireEvent.pointerEnter(nodeLink);
    stepFrames(10, 100);
    expect(Number.parseFloat(nodeLink.style.left)).toBeCloseTo(rotatedLeft, 4);

    fireEvent.pointerLeave(nodeLink);
    stepFrames(12, 100);
    expect(Number.parseFloat(nodeLink.style.left)).toBeGreaterThan(rotatedLeft);
  });

  it('does not schedule continuous rotation when prefers-reduced-motion is enabled', () => {
    reducedMotion = true;
    renderGlobe();

    const nodeLink = screen.getByRole('link', { name: /edge-cn/ });
    const initialLeft = nodeLink.style.left;
    expect(rafQueue).toHaveLength(0);
    expect(nodeLink.style.left).toBe(initialLeft);
  });
});
