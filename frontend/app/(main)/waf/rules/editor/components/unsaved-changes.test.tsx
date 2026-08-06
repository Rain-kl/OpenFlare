import { act, render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, expect, it, vi } from 'vitest';

import { UnsavedChanges } from './unsaved-changes';

afterEach(() => vi.restoreAllMocks());

it('blocks same-origin application links when dirty and confirmation is declined', async () => {
  const user = userEvent.setup();
  const { container } = render(
    <>
      <UnsavedChanges dirty />
      <a href='/waf'>WAF</a>
    </>,
  );
  const event = new MouseEvent('click', {
    bubbles: true,
    cancelable: true,
    button: 0,
  });
  act(() => {
    container.querySelector('a')!.dispatchEvent(event);
  });
  expect(event.defaultPrevented).toBe(true);
  expect(
    await screen.findByText('存在未保存的更改，确定离开吗？'),
  ).toBeTruthy();
  await user.click(screen.getByRole('button', { name: '取消' }));
  expect(screen.queryByText('存在未保存的更改，确定离开吗？')).toBeNull();
});

it('does not block application links without changes', () => {
  const { getByRole } = render(
    <>
      <UnsavedChanges dirty={false} />
      <a href='/waf'>WAF</a>
    </>,
  );
  const event = new MouseEvent('click', {
    bubbles: true,
    cancelable: true,
    button: 0,
  });
  event.preventDefault();
  getByRole('link').dispatchEvent(event);
  expect(screen.queryByText('存在未保存的更改，确定离开吗？')).toBeNull();
});

it('restores declined Back and Forward transitions by indexed delta', async () => {
  const user = userEvent.setup();
  const go = vi.spyOn(history, 'go').mockImplementation(() => undefined);
  history.replaceState({ __wafEditorIndex: 4 }, '');
  render(<UnsavedChanges dirty />);

  act(() => {
    window.dispatchEvent(
      new PopStateEvent('popstate', { state: { __wafEditorIndex: 3 } }),
    );
  });
  expect(go).toHaveBeenLastCalledWith(1);
  expect(await screen.findByText('存在未保存的更改，确定离开吗？')).toBeTruthy();
  await user.click(screen.getByRole('button', { name: '取消' }));

  act(() => {
    window.dispatchEvent(
      new PopStateEvent('popstate', { state: { __wafEditorIndex: 4 } }),
    );
  });
  act(() => {
    window.dispatchEvent(
      new PopStateEvent('popstate', { state: { __wafEditorIndex: 6 } }),
    );
  });
  expect(go).toHaveBeenLastCalledWith(-2);
});

it('prompts and restores the current URL for an unknown unindexed history entry', async () => {
  history.replaceState({ __wafEditorIndex: 4 }, '', '/waf/rules/editor?id=9');
  const push = vi.spyOn(history, 'pushState');
  render(<UnsavedChanges dirty />);

  act(() => {
    window.dispatchEvent(
      new PopStateEvent('popstate', { state: { legacy: true } }),
    );
  });

  expect(await screen.findByText('存在未保存的更改，确定离开吗？')).toBeTruthy();
  expect(push).toHaveBeenCalledWith(
    expect.objectContaining({ __wafEditorIndex: 4 }),
    '',
    '/waf/rules/editor?id=9',
  );
});
