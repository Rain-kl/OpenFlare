import { fireEvent, render, screen } from '@testing-library/react';
import { expect, it, vi } from 'vitest';

import type { WAFIPGroup, WAFRuleNode } from '@/lib/services/openflare';

import { NodeProperties } from './node-properties';

it('hides match and block until UA check is enabled', () => {
  const node: WAFRuleNode = {
    id: 'ua',
    type: 'ua_check',
    position: { x: 0, y: 0 },
    config: {
      require_ua: false,
      browsers: [],
      operating_systems: [],
      match_mode: 'or',
      block_common_bots: false,
      block_abnormal_ua: false,
      block_custom_ua: false,
      custom_ua_patterns: [],
    },
  };
  const onChange = vi.fn();
  const { rerender } = render(
    <NodeProperties node={node} ipGroups={[]} onChange={onChange} />,
  );
  expect(screen.queryByLabelText('屏蔽常见爬虫 UA')).not.toBeInTheDocument();
  expect(screen.queryByLabelText('浏览器')).not.toBeInTheDocument();
  fireEvent.click(screen.getByLabelText('开启 UA 检查'));
  expect(onChange).toHaveBeenCalledWith(
    expect.objectContaining({
      config: expect.objectContaining({ require_ua: true }),
    }),
  );
  rerender(
    <NodeProperties
      node={{ ...node, config: { ...node.config, require_ua: true } }}
      ipGroups={[]}
      onChange={onChange}
    />,
  );
  expect(screen.getByLabelText('屏蔽常见爬虫 UA')).toBeInTheDocument();
  expect(screen.getByLabelText('屏蔽非正常 UA')).toBeInTheDocument();
  expect(screen.getByLabelText('屏蔽自定义 UA')).toBeInTheDocument();
  expect(screen.getByText(/浏览器或操作系统分类为 Bot/)).toBeInTheDocument();
  expect(screen.getByText(/浏览器分类为 Other 或 Unknown/)).toBeInTheDocument();
});

it('edits display name for configurable nodes', () => {
  const node: WAFRuleNode = {
    id: 'match',
    type: 'ip_match',
    position: { x: 0, y: 0 },
    config: { ips: [], cidrs: [], ip_group_ids: [] },
  };
  const onChange = vi.fn();
  render(<NodeProperties node={node} ipGroups={[]} onChange={onChange} />);
  fireEvent.change(screen.getByLabelText('显示名称'), {
    target: { value: '内网放行' },
  });
  expect(onChange).toHaveBeenCalledWith(
    expect.objectContaining({ label: '内网放行' }),
  );
});

it('hides display name for system nodes', () => {
  const node: WAFRuleNode = {
    id: 'start',
    type: 'start',
    position: { x: 0, y: 0 },
    config: {},
  };
  render(<NodeProperties node={node} ipGroups={[]} onChange={vi.fn()} />);
  expect(screen.queryByLabelText('显示名称')).not.toBeInTheDocument();
  expect(screen.getByText('系统节点无需配置。')).toBeInTheDocument();
});

it('edits IP group config through a typed multi-select', async () => {
  const node: WAFRuleNode = {
    id: 'match',
    type: 'ip_match',
    position: { x: 0, y: 0 },
    config: { ips: [], cidrs: [], ip_group_ids: [] },
  };
  const group = { id: 7, name: '办公室出口' } as WAFIPGroup;
  const onChange = vi.fn();
  render(<NodeProperties node={node} ipGroups={[group]} onChange={onChange} />);
  fireEvent.click(screen.getByRole('button', { name: 'IP 组' }));
  fireEvent.click(await screen.findByText('办公室出口'));
  expect(onChange).toHaveBeenCalledWith(
    expect.objectContaining({
      config: expect.objectContaining({ ip_group_ids: [7] }),
    }),
  );
});

it('associates numeric property labels and constrains server ranges', () => {
  const node: WAFRuleNode = {
    id: 'pow',
    type: 'pow',
    position: { x: 0, y: 0 },
    config: {
      algorithm: 'fast',
      difficulty: 4,
      session_ttl: 60,
      challenge_ttl: 30,
    },
  };
  render(<NodeProperties node={node} ipGroups={[]} onChange={vi.fn()} />);
  expect(screen.getByLabelText('难度')).toHaveAttribute('min', '1');
  expect(screen.getByLabelText('难度')).toHaveAttribute('max', '16');
  expect(screen.getByLabelText('会话 TTL（秒）')).toHaveAttribute('min', '60');
});

it('creates any normalized valid geography code', async () => {
  const node: WAFRuleNode = {
    id: 'geo',
    type: 'geo_match',
    position: { x: 0, y: 0 },
    config: { countries: [], regions: [] },
  };
  const onChange = vi.fn();
  render(<NodeProperties node={node} ipGroups={[]} onChange={onChange} />);
  fireEvent.click(screen.getByRole('button', { name: '国家代码' }));
  fireEvent.change(await screen.findByPlaceholderText('搜索名称或代码'), {
    target: { value: 'nz' },
  });
  fireEvent.click(screen.getByRole('button', { name: '添加代码' }));
  expect(onChange).toHaveBeenCalledWith(
    expect.objectContaining({
      config: expect.objectContaining({ countries: ['NZ'] }),
    }),
  );
});

it('shows localized country names together with their codes', async () => {
  const node: WAFRuleNode = {
    id: 'geo',
    type: 'geo_match',
    position: { x: 0, y: 0 },
    config: { countries: [], regions: [] },
  };
  render(<NodeProperties node={node} ipGroups={[]} onChange={vi.fn()} />);
  fireEvent.click(screen.getByRole('button', { name: '国家代码' }));
  expect(await screen.findByText('中国')).toBeInTheDocument();
  expect(screen.getByText('CN')).toBeInTheDocument();
  expect(screen.getByText(/共 249 个国家\/地区/)).toBeInTheDocument();
});
