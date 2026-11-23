import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { NextIntlClientProvider } from 'next-intl';
import type { ReactNode } from 'react';
import { afterEach, expect, it, vi } from 'vitest';

import AttackProtectionPage from '@/app/(main)/attack-protection/page';
import AttackProtectionEditorPage from '@/app/(main)/attack-protection/editor/page';
import type { RequestWhitelistEntry } from '@/lib/services/openflare/attack-types';
import { WafService } from '@/lib/services/openflare/waf.service';
import messages from '@/messages/fragments/attack.zh-CN.json';

afterEach(() => vi.restoreAllMocks());

const replace = vi.fn();
let params = new URLSearchParams();
vi.mock('next/navigation', () => ({
  useRouter: () => ({ replace }),
  useSearchParams: () => params,
}));

function fixture(client: QueryClient, page: ReactNode) {
  return (
    <NextIntlClientProvider
      locale='zh-CN'
      timeZone='UTC'
      messages={{
        ...messages,
        runtime: {
          ui_closeLabel: '关闭',
          loading_d04fcbda: '正在加载',
          fetching_data_9ac5e428: '正在获取数据',
        },
      }}
    >
      <QueryClientProvider client={client}>{page}</QueryClientProvider>
    </NextIntlClientProvider>
  );
}

it('lets editors override individual rules and save them with the category levels', async () => {
  const stored = {
    id: 42,
    name: '平衡',
    levels: { csrf: 'balanced' as const },
    rule_overrides: { 11151: 'observe' as const },
    updated_at: '2026-11-20T08:00:00Z',
  };
  vi.spyOn(WafService, 'listAttackGroups').mockResolvedValue([stored]);
  vi.spyOn(WafService, 'listRequestWhitelist').mockResolvedValue([]);
  const save = vi
    .spyOn(WafService, 'saveAttackGroup')
    .mockResolvedValue(stored);
  const client = new QueryClient();
  const user = userEvent.setup();
  const { rerender } = render(fixture(client, <AttackProtectionPage />));

  await user.click(screen.getByRole('button', { name: '检测规则目录' }));
  let dialog = within(screen.getByRole('dialog'));
  await user.click(dialog.getByRole('textbox'));
  await user.paste('934110');
  expect(dialog.getByText('934110')).toBeVisible();
  expect(dialog.getByText('服务端请求伪造（SSRF）')).toBeVisible();
  expect(
    dialog.queryByRole('combobox', { name: '规则 934110 的应用状态' }),
  ).not.toBeInTheDocument();
  await user.keyboard('{Escape}');

  expect(screen.getByRole('link', { name: '新建规则组' })).toHaveAttribute(
    'href',
    '/attack-protection/editor',
  );
  const edit = await screen.findByRole('link', { name: '编辑' });
  expect(edit).toHaveAttribute('href', '/attack-protection/editor?id=42');
  params = new URLSearchParams('id=42');
  rerender(fixture(client, <AttackProtectionEditorPage />));
  expect(screen.getByRole('link', { name: '返回攻击防护' })).toHaveAttribute(
    'href',
    '/attack-protection',
  );
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
  const csrf = screen.getByRole('combobox', { name: '跨站请求伪造（CSRF）' });
  csrf.focus();
  await user.keyboard('{Enter}');
  await user.click(screen.getByRole('option', { name: '必要防护' }));
  expect(screen.getByRole('button', { name: '1 条运行中' })).toBeVisible();
  await user.click(screen.getByRole('button', { name: '保存' }));
  expect(save).toHaveBeenCalledWith(
    {
      name: '平衡',
      levels: { csrf: 'essential' },
      rule_overrides: { 11152: 'enabled' },
    },
    42,
  );
}, 10_000);

it('adds a global domain and path exemption from the attack protection page', async () => {
  vi.spyOn(WafService, 'listAttackGroups').mockResolvedValue([]);
  const saved: RequestWhitelistEntry = {
    id: 3,
    domain: '*.example.test',
    path_match: 'suffix',
    path: '.webp',
  };
  vi.spyOn(WafService, 'listRequestWhitelist')
    .mockResolvedValueOnce([])
    .mockResolvedValue([saved]);
  const save = vi
    .spyOn(WafService, 'saveRequestWhitelistEntry')
    .mockResolvedValue(saved);
  const user = userEvent.setup();
  render(fixture(new QueryClient(), <AttackProtectionPage />));

  await user.click(screen.getByRole('button', { name: '添加白名单' }));
  const dialog = within(screen.getByRole('dialog'));
  await user.type(
    dialog.getByRole('textbox', { name: '匹配域名' }),
    saved.domain,
  );
  const mode = dialog.getByRole('combobox', { name: '路径匹配方式' });
  mode.focus();
  await user.keyboard('{Enter}');
  await user.click(screen.getByRole('option', { name: '后缀' }));
  await user.type(
    dialog.getByRole('textbox', { name: '匹配路径' }),
    saved.path,
  );
  await user.click(dialog.getByRole('button', { name: '保存' }));
  expect(save).toHaveBeenCalledWith(
    { domain: saved.domain, path_match: 'suffix', path: saved.path },
    undefined,
  );
  expect(await screen.findByRole('cell', { name: saved.domain })).toBeVisible();
});
