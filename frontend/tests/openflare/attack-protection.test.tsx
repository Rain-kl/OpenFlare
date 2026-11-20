import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { NextIntlClientProvider } from 'next-intl';
import { afterEach, expect, it, vi } from 'vitest';

import AttackProtectionPage from '@/app/(main)/attack-protection/page';
import { WafService } from '@/lib/services/openflare/waf.service';
import messages from '@/messages/fragments/attack.zh-CN.json';

afterEach(() => vi.restoreAllMocks());

it('shows CRS rules from the page and previews the edited category levels', async () => {
  vi.spyOn(WafService, 'listAttackGroups').mockResolvedValue([]);
  const client = new QueryClient();
  const user = userEvent.setup();
  render(
    <NextIntlClientProvider
      locale='zh-CN'
      timeZone='UTC'
      messages={{ ...messages, runtime: { ui_closeLabel: '关闭' } }}
    >
      <QueryClientProvider client={client}>
        <AttackProtectionPage />
      </QueryClientProvider>
    </NextIntlClientProvider>,
  );

  await user.click(screen.getByRole('button', { name: '检测规则目录' }));
  let dialog = within(screen.getByRole('dialog'));
  await user.type(dialog.getByRole('textbox'), '934110');
  expect(dialog.getByText('934110')).toBeVisible();
  expect(dialog.getByText('服务端请求伪造（SSRF）')).toBeVisible();
  await user.click(dialog.getByRole('button', { name: '关闭' }));

  await user.click(screen.getByRole('button', { name: '新建规则组' }));
  const csrf = screen.getByRole('radiogroup', { name: '跨站请求伪造（CSRF）' });
  await user.click(within(csrf).getByRole('radio', { name: '平衡防护' }));
  const group = csrf.closest('fieldset');
  expect(group).not.toBeNull();
  await user.click(
    within(group!).getByRole('button', { name: '启用 1 条规则' }),
  );
  dialog = within(screen.getByRole('dialog'));
  const base = dialog.getByText('11151').closest('tr');
  const extended = dialog.getByText('11152').closest('tr');
  expect(base).toHaveTextContent('平衡防护');
  expect(extended).toHaveTextContent('禁用');
});
