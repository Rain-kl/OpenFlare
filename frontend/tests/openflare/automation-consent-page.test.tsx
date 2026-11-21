import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { NextIntlClientProvider } from 'next-intl';
import { afterEach, expect, it, vi } from 'vitest';

import { AutomationConsentPage } from '@/components/auth/automation-consent-page';
import messages from '@/messages/fragments/automation.zh-CN.json';

vi.mock('next/navigation', () => ({
  useSearchParams: () => new URLSearchParams(),
}));

afterEach(() => {
  window.history.replaceState(null, '', '/');
  vi.unstubAllGlobals();
});

function renderConsent(scopes: string[]) {
  window.history.replaceState(null, '', '/automation/authorize#intent=test');
  vi.stubGlobal(
    'fetch',
    vi.fn().mockResolvedValue({
      ok: true,
      json: async () => ({
        client_name: 'OpenFlare CLI',
        redirect_uri: 'http://127.0.0.1/callback',
        scopes,
      }),
    }),
  );

  render(
    <NextIntlClientProvider locale='zh-CN' timeZone='UTC' messages={messages}>
      <QueryClientProvider client={new QueryClient()}>
        <AutomationConsentPage />
      </QueryClientProvider>
    </NextIntlClientProvider>,
  );
}

it('selects only the requested scopes in each enabled permission group', async () => {
  renderConsent(['sites:write', 'connection:read', 'sites:read']);
  const user = userEvent.setup();
  const read = await screen.findByRole('switch', { name: '读取权限' });
  const write = screen.getByRole('switch', { name: '写入权限' });
  const confirm = screen.getByRole('button', { name: '确认授权' });
  const form = confirm.closest('form')!;

  expect(read).not.toBeChecked();
  expect(write).not.toBeChecked();
  expect(confirm).toBeDisabled();
  expect(new FormData(form).get('scope')).toBe('');

  const readDetails = screen.getByRole('region', { name: '读取权限' });
  const writeDetails = screen.getByRole('region', { name: '写入权限' });
  expect(within(readDetails).getAllByRole('listitem')).toHaveLength(2);
  expect(within(writeDetails).getByRole('listitem')).toHaveTextContent(
    '创建和修改站点及域名',
  );

  await user.click(read);
  expect(new FormData(form).get('scope')).toBe('connection:read sites:read');
  expect(confirm).toBeEnabled();

  await user.click(write);
  expect(new FormData(form).get('scope')).toBe(
    'sites:write connection:read sites:read',
  );

  await user.click(read);
  expect(new FormData(form).get('scope')).toBe('sites:write');
  expect(write).toBeChecked();

  await user.click(write);
  expect(confirm).toBeDisabled();
});

it('only displays groups with requested permissions', async () => {
  renderConsent(['nodes:read']);
  expect(await screen.findByRole('switch', { name: '读取权限' })).toBeVisible();
  expect(
    screen.queryByRole('switch', { name: '写入权限' }),
  ).not.toBeInTheDocument();
});
