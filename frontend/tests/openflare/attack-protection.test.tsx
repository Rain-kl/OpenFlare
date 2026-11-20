import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { NextIntlClientProvider } from 'next-intl';
import { afterEach, expect, it, vi } from 'vitest';

import AttackProtectionPage from '@/app/(main)/attack-protection/page';
import { WafService } from '@/lib/services/openflare/waf.service';
import messages from '@/messages/fragments/attack.zh-CN.json';

afterEach(() => vi.restoreAllMocks());

it('lets editors override individual rules and save them with the category levels', async () => {
  const stored = {
    id: 42,
    name: '平衡',
    levels: { csrf: 'balanced' as const },
    rule_overrides: { 11151: 'observe' as const },
    updated_at: '2026-11-20T08:00:00Z',
  };
  vi.spyOn(WafService, 'listAttackGroups').mockResolvedValue([stored]);
  const save = vi
    .spyOn(WafService, 'saveAttackGroup')
    .mockResolvedValue(stored);
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
  expect(
    dialog.queryByRole('combobox', { name: '规则 934110 的应用状态' }),
  ).not.toBeInTheDocument();
  await user.keyboard('{Escape}');

  await user.click(await screen.findByRole('button', { name: '编辑' }));
  await user.click(screen.getByRole('button', { name: '1 条运行中' }));
  dialog = within(screen.getByRole('dialog'));
  const base = dialog.getByRole('combobox', { name: '规则 11151 的应用状态' });
  const extended = dialog.getByRole('combobox', {
    name: '规则 11152 的应用状态',
  });
  expect(base).toHaveTextContent('记录');
  expect(extended).toHaveTextContent('关闭');

  HTMLElement.prototype.scrollIntoView ??= vi.fn();
  base.focus();
  await user.keyboard('{Enter}');
  await user.click(
    screen.getByRole('option', { name: '沿用类别设置（开启）' }),
  );
  expect(base).toHaveTextContent('开启');

  extended.focus();
  await user.keyboard('{Enter}');
  await user.click(screen.getByRole('option', { name: '开启' }));
  await user.keyboard('{Escape}');
  expect(screen.getByRole('button', { name: '2 条运行中' })).toBeVisible();
  await user.click(screen.getByRole('button', { name: '保存' }));
  expect(save).toHaveBeenCalledWith(
    {
      name: '平衡',
      levels: { csrf: 'balanced' },
      rule_overrides: { 11152: 'enabled' },
    },
    42,
  );
});
