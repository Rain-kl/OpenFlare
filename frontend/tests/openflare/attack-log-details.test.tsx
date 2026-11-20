import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen } from '@testing-library/react';
import { NextIntlClientProvider } from 'next-intl';
import { afterEach, expect, it, vi } from 'vitest';

import { AttackLogDetails } from '@/app/(main)/attack-logs/components/attack-log-details';
import type { AttackLog } from '@/lib/services/openflare/attack-types';
import { WafService } from '@/lib/services/openflare/waf.service';
import messages from '@/messages/fragments/attack.zh-CN.json';

afterEach(() => vi.restoreAllMocks());

it('shows the HTTP request preview as text and handles historical records', async () => {
  const log: AttackLog = {
    id: '128401926193680388',
    node_id: 'edge-a',
    logged_at: '2026-11-20T08:00:00Z',
    remote_addr: '192.0.2.1',
    region: '',
    host: 'app.example.test',
    path: '/',
    status_code: 403,
    action: 'blocked',
    reason: 'block',
    method: 'POST',
    https: true,
    flow_id: 1,
    flow_name: 'Default',
    flow_node_id: 'attack-protection',
  };
  const message =
    'POST /?view=summary HTTP/1.1\r\nHost: app.example.test\r\n\r\n<b>Example</b>';
  const getRequest = vi
    .spyOn(WafService, 'getAttackLogRequest')
    .mockResolvedValueOnce({
      request: { message, truncated: true, body_omitted: false },
    })
    .mockResolvedValueOnce({ request: null });
  const client = new QueryClient();
  const details = (entry: AttackLog) => (
    <NextIntlClientProvider
      locale='zh-CN'
      timeZone='UTC'
      messages={{ ...messages, runtime: { ui_closeLabel: '关闭' } }}
    >
      <QueryClientProvider client={client}>
        <AttackLogDetails log={entry} onClose={vi.fn()} />
      </QueryClientProvider>
    </NextIntlClientProvider>
  );
  const { rerender } = render(details(log));

  expect(screen.getByRole('heading', { name: '请求报文' })).toBeVisible();
  expect(await screen.findByText('报文超过 8 KiB，已截断。')).toBeVisible();
  const code = screen.getByRole('code');
  expect(code.textContent).toBe(message);
  expect(code.childElementCount).toBe(0);
  expect(getRequest).toHaveBeenCalledWith(log);

  rerender(details({ ...log, id: '128401926193680389' }));
  expect(await screen.findByText('该记录没有采集请求报文。')).toBeVisible();
  expect(screen.queryByRole('code')).not.toBeInTheDocument();
});
