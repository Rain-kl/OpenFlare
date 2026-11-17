import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { NextIntlClientProvider } from 'next-intl';
import { expect, it, vi } from 'vitest';
import {
  PoWService,
  defaultPoWConfig,
  type PoWPathRules,
} from '@/lib/services/openflare';
import mainZh from '@/messages/zh-CN.json';
import powZh from '@/messages/fragments/pow.zh-CN.json';
import { PolicyEditor } from './policy-editor';

const messages = { ...mainZh, ...powZh };

it.each<PoWPathRules>([
  { mode: 'all' },
  { mode: 'blacklist', blacklist: [{ type: 'exact', value: '/login' }] },
])('reads omitted empty lists from the API: %j', (path_rules) => {
  render(
    <QueryClientProvider client={new QueryClient()}>
      <NextIntlClientProvider locale='zh-CN' messages={messages}>
        <PolicyEditor
          policy={{
            route_id: 17,
            enabled: true,
            config: { ...defaultPoWConfig, path_rules },
          }}
        />
      </NextIntlClientProvider>
    </QueryClientProvider>,
  );
  const addButtons = screen.getAllByRole('button', { name: '添加路径规则' });
  expect(addButtons).toHaveLength(2);
  fireEvent.click(addButtons[0]);
  expect(screen.getAllByLabelText('路径或正则表达式')).toHaveLength(
    (path_rules.blacklist?.length ?? 0) + 1,
  );
});

it('saves independent site settings with path exceptions', async () => {
  const save = vi.spyOn(PoWService, 'save').mockResolvedValue(undefined);
  const client = new QueryClient({
    defaultOptions: { mutations: { retry: false } },
  });
  render(
    <QueryClientProvider client={client}>
      <NextIntlClientProvider locale='zh-CN' messages={messages}>
        <PolicyEditor
          policy={{
            route_id: 17,
            enabled: false,
            config: {
              ...defaultPoWConfig,
              path_rules: {
                mode: 'blacklist',
                blacklist: [{ type: 'prefix', value: '/admin/' }],
                whitelist: [],
              },
            },
          }}
        />
      </NextIntlClientProvider>
    </QueryClientProvider>,
  );
  fireEvent.click(screen.getByRole('switch', { name: '启用 PoW 校验' }));
  fireEvent.change(screen.getByLabelText('计算难度'), {
    target: { value: '3' },
  });
  fireEvent.click(screen.getAllByRole('button', { name: '添加路径规则' })[0]);
  fireEvent.change(screen.getAllByLabelText('路径或正则表达式')[0], {
    target: { value: '/health' },
  });
  fireEvent.click(screen.getByRole('button', { name: '保存' }));
  await waitFor(() =>
    expect(save).toHaveBeenCalledWith({
      route_id: 17,
      enabled: true,
      config: expect.objectContaining({
        difficulty: 3,
        path_rules: {
          mode: 'blacklist',
          blacklist: [{ type: 'prefix', value: '/admin/' }],
          whitelist: [{ type: 'exact', value: '/health' }],
        },
      }),
    }),
  );
  save.mockRestore();
});
