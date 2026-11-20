import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { fireEvent, render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { NextIntlClientProvider } from 'next-intl';
import { afterEach, expect, it, vi } from 'vitest';

import { AttackLogFilters } from '@/app/(main)/attack-logs/components/attack-log-filters';
import { NodeService } from '@/lib/services/openflare';
import messages from '@/messages/fragments/attack.zh-CN.json';

afterEach(() => vi.restoreAllMocks());

it('keeps the time range when collapsed and opens the fields on invalid input', async () => {
  vi.spyOn(NodeService, 'listNodes').mockResolvedValue([]);
  const apply = vi.fn();
  const user = userEvent.setup();
  const initialSince = 1_795_164_000;
  const initialUntil = 1_795_167_600;

  render(
    <NextIntlClientProvider locale='zh-CN' timeZone='UTC' messages={messages}>
      <QueryClientProvider client={new QueryClient()}>
        <AttackLogFilters
          filter={{
            action: 'blocked',
            since: initialSince,
            until: initialUntil,
            node_id: 'edge-a',
          }}
          apply={apply}
          onRefresh={vi.fn()}
          interval={0}
          setInterval={vi.fn()}
          fetching={false}
        />
      </QueryClientProvider>
    </NextIntlClientProvider>,
  );

  const more = screen.getByRole('button', { name: /^更多筛选\s*2$/ });
  const submit = screen.getByRole('button', { name: '应用筛选' });
  const since = screen.getByLabelText('开始时间');
  const until = screen.getByLabelText('结束时间');

  expect(more).toHaveAttribute('aria-expanded', 'false');
  expect(since).not.toBeVisible();
  await user.click(submit);
  expect(apply).toHaveBeenLastCalledWith(
    expect.objectContaining({
      since: initialSince,
      until: initialUntil,
      node_id: 'edge-a',
    }),
  );

  await user.click(more);
  expect(since).toBeVisible();
  fireEvent.change(since, { target: { value: '2026-11-20T08:00' } });
  fireEvent.change(until, { target: { value: '2026-11-20T09:00' } });
  await user.click(more);
  await user.click(submit);
  expect(apply).toHaveBeenLastCalledWith(
    expect.objectContaining({
      since: new Date('2026-11-20T08:00').getTime() / 1000,
      until: new Date('2026-11-20T09:00').getTime() / 1000,
    }),
  );

  await user.click(more);
  fireEvent.change(until, { target: { value: '2026-11-20T07:00' } });
  await user.click(more);
  await user.click(submit);
  expect(apply).toHaveBeenCalledTimes(2);
  expect(more).toHaveAttribute('aria-expanded', 'true');
  expect(screen.getByText('结束时间必须晚于开始时间。')).toBeVisible();
});
