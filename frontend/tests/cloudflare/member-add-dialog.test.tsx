import { fireEvent, render, screen, within } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';

import { MemberAddDialog } from '@/app/(main)/cloudflare/components/member-add-dialog';
import { NextIntlClientProvider } from 'next-intl';
import zhCN from '@/messages/zh-CN.json';
import type { CloudflareAvailableDomain } from '@/lib/services/openflare';

const domains: CloudflareAvailableDomain[] = [
  {
    id: 1,
    zone_id: 10,
    domain: 'example.com',
    zone_domain: 'example.com',
  },
  {
    id: 2,
    zone_id: 10,
    domain: 'www.example.com',
    zone_domain: 'example.com',
  },
  {
    id: 3,
    zone_id: 10,
    domain: 'api.example.com',
    zone_domain: 'example.com',
  },
  {
    id: 4,
    zone_id: 20,
    domain: 'app.other.io',
    zone_domain: 'other.io',
  },
];

describe('MemberAddDialog advanced domain picker', () => {
  it('groups domains under top-level zone roots and supports batch select', () => {
    const onSubmit = vi.fn();
    render(
      <NextIntlClientProvider
        locale='zh-CN'
        messages={zhCN}
        timeZone='Asia/Shanghai'
      >
        <MemberAddDialog
          open
          onOpenChange={vi.fn()}
          domains={domains}
          defaultProxied
          pending={false}
          onSubmit={onSubmit}
        />
      </NextIntlClientProvider>,
    );

    expect(screen.getAllByText('example.com').length).toBeGreaterThan(0);
    expect(screen.getAllByText('other.io').length).toBeGreaterThan(0);
    expect(screen.getByText('www.example.com')).toBeVisible();
    expect(screen.getByText('api.example.com')).toBeVisible();

    fireEvent.click(screen.getByRole('button', { name: '全选可见' }));
    fireEvent.click(screen.getByRole('button', { name: '添加并同步（4）' }));

    expect(onSubmit).toHaveBeenCalledTimes(1);
    const [ids, proxied] = onSubmit.mock.calls[0] as [number[], boolean];
    expect([...ids].sort((a, b) => a - b)).toEqual([1, 2, 3, 4]);
    expect(proxied).toBe(true);
  });

  it('filters by keyword across domain and zone root', () => {
    render(
      <NextIntlClientProvider
        locale='zh-CN'
        messages={zhCN}
        timeZone='Asia/Shanghai'
      >
        <MemberAddDialog
          open
          onOpenChange={vi.fn()}
          domains={domains}
          defaultProxied={false}
          pending={false}
          onSubmit={vi.fn()}
        />
      </NextIntlClientProvider>,
    );

    fireEvent.change(screen.getByPlaceholderText('搜索域名或顶级域…'), {
      target: { value: 'other' },
    });

    expect(screen.queryByText('www.example.com')).not.toBeInTheDocument();
    expect(screen.getByText('app.other.io')).toBeVisible();
    expect(screen.getByText('other.io')).toBeVisible();
  });

  it('selects an entire top-level domain group', () => {
    const onSubmit = vi.fn();
    render(
      <NextIntlClientProvider
        locale='zh-CN'
        messages={zhCN}
        timeZone='Asia/Shanghai'
      >
        <MemberAddDialog
          open
          onOpenChange={vi.fn()}
          domains={domains}
          defaultProxied
          pending={false}
          onSubmit={onSubmit}
        />
      </NextIntlClientProvider>,
    );

    fireEvent.click(
      screen.getByRole('checkbox', { name: '选择顶级域 example.com' }),
    );
    fireEvent.click(screen.getByRole('button', { name: '添加并同步（3）' }));

    expect(onSubmit).toHaveBeenCalledTimes(1);
    const [ids, proxied] = onSubmit.mock.calls[0] as [number[], boolean];
    expect([...ids].sort((a, b) => a - b)).toEqual([1, 2, 3]);
    expect(proxied).toBe(true);
  });

  it('toggles individual domains within a group', () => {
    const onSubmit = vi.fn();
    render(
      <NextIntlClientProvider
        locale='zh-CN'
        messages={zhCN}
        timeZone='Asia/Shanghai'
      >
        <MemberAddDialog
          open
          onOpenChange={vi.fn()}
          domains={domains}
          defaultProxied={false}
          pending={false}
          onSubmit={onSubmit}
        />
      </NextIntlClientProvider>,
    );

    const wwwLabel = screen.getByText('www.example.com').closest('label');
    expect(wwwLabel).toBeTruthy();
    fireEvent.click(within(wwwLabel as HTMLElement).getByRole('checkbox'));

    fireEvent.click(screen.getByRole('button', { name: '添加并同步' }));
    expect(onSubmit).toHaveBeenCalledWith([2], false);
  });
});
