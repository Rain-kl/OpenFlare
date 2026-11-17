import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { NextIntlClientProvider } from 'next-intl';
import { afterEach, expect, it, vi } from 'vitest';

import {
  defaultPoWConfig,
  PoWService,
  type ProxyRouteItem,
} from '@/lib/services/openflare';
import messages from '@/messages/zh-CN.json';
import { PoWSection } from './pow-section';

afterEach(() => vi.restoreAllMocks());

it('loads and saves the current rule when switching between rule details', async () => {
  vi.spyOn(PoWService, 'list').mockResolvedValue([
    {
      route_id: 7,
      enabled: true,
      config: { ...defaultPoWConfig, difficulty: 2 },
    },
    {
      route_id: 9,
      enabled: false,
      config: { ...defaultPoWConfig, difficulty: 5 },
    },
  ]);
  const save = vi.spyOn(PoWService, 'save').mockResolvedValue(undefined);
  const client = new QueryClient();
  const view = (id: number) => (
    <QueryClientProvider client={client}>
      <NextIntlClientProvider locale='zh-CN' messages={messages}>
        <PoWSection route={{ id } as ProxyRouteItem} />
      </NextIntlClientProvider>
    </QueryClientProvider>
  );
  const { rerender } = render(view(9));
  await waitFor(() => expect(screen.getByLabelText('计算难度')).toHaveValue(5));
  expect(
    screen.getByRole('switch', { name: '启用 PoW 校验' }),
  ).not.toBeChecked();
  fireEvent.click(screen.getByRole('button', { name: '保存' }));
  await waitFor(() =>
    expect(save).toHaveBeenCalledWith(
      expect.objectContaining({ route_id: 9, enabled: false }),
    ),
  );
  rerender(view(7));
  await waitFor(() => expect(screen.getByLabelText('计算难度')).toHaveValue(2));
  expect(screen.getByRole('switch', { name: '启用 PoW 校验' })).toBeChecked();
  fireEvent.click(screen.getByRole('button', { name: '保存' }));
  await waitFor(() =>
    expect(save).toHaveBeenLastCalledWith(
      expect.objectContaining({ route_id: 7, enabled: true }),
    ),
  );
  expect(screen.getByRole('link', { name: '挑战页设置' })).toHaveAttribute(
    'href',
    '/responses?tab=pow',
  );
});

it('uses disabled defaults for a rule without a saved policy', async () => {
  vi.spyOn(PoWService, 'list').mockResolvedValue([
    {
      route_id: 7,
      enabled: true,
      config: { ...defaultPoWConfig, difficulty: 2 },
    },
  ]);
  const save = vi.spyOn(PoWService, 'save').mockResolvedValue(undefined);
  render(
    <QueryClientProvider client={new QueryClient()}>
      <NextIntlClientProvider locale='zh-CN' messages={messages}>
        <PoWSection route={{ id: 42 } as ProxyRouteItem} />
      </NextIntlClientProvider>
    </QueryClientProvider>,
  );
  await waitFor(() => expect(screen.getByLabelText('计算难度')).toHaveValue(4));
  expect(
    screen.getByRole('switch', { name: '启用 PoW 校验' }),
  ).not.toBeChecked();
  fireEvent.click(screen.getByRole('button', { name: '保存' }));
  await waitFor(() =>
    expect(save).toHaveBeenCalledWith(
      expect.objectContaining({ route_id: 42, enabled: false }),
    ),
  );
});
