import { render } from '@testing-library/react';
import { afterEach, expect, it, vi } from 'vitest';

import { UnsavedChanges } from './unsaved-changes';

afterEach(() => vi.restoreAllMocks());

it('blocks same-origin application links when dirty and confirmation is declined', () => {
  vi.spyOn(window, 'confirm').mockReturnValue(false);
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
  container.querySelector('a')!.dispatchEvent(event);
  expect(window.confirm).toHaveBeenCalledOnce();
  expect(event.defaultPrevented).toBe(true);
});

it('does not block application links without changes', () => {
  const confirm = vi.spyOn(window, 'confirm');
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
  expect(confirm).not.toHaveBeenCalled();
});

it('restores declined Back and Forward transitions by indexed delta', () => {
  vi.spyOn(window, 'confirm').mockReturnValue(false);
  const go = vi.spyOn(history, 'go').mockImplementation(() => undefined);
  history.replaceState({ __wafEditorIndex: 4 }, '');
  render(<UnsavedChanges dirty />);
  window.dispatchEvent(
    new PopStateEvent('popstate', { state: { __wafEditorIndex: 3 } }),
  );
  expect(go).toHaveBeenLastCalledWith(1);
  window.dispatchEvent(
    new PopStateEvent('popstate', { state: { __wafEditorIndex: 4 } }),
  );
  window.dispatchEvent(
    new PopStateEvent('popstate', { state: { __wafEditorIndex: 6 } }),
  );
  expect(go).toHaveBeenLastCalledWith(-2);
});

it('prompts and restores the current URL for an unknown unindexed history entry', () => {
  vi.spyOn(window, 'confirm').mockReturnValue(false);
  history.replaceState({ __wafEditorIndex: 4 }, '', '/waf/rules/editor?id=9');
  const push = vi.spyOn(history, 'pushState');
  render(<UnsavedChanges dirty />);
  window.dispatchEvent(
    new PopStateEvent('popstate', { state: { legacy: true } }),
  );
  expect(window.confirm).toHaveBeenCalledOnce();
  expect(push).toHaveBeenCalledWith(
    expect.objectContaining({ __wafEditorIndex: 4 }),
    '',
    '/waf/rules/editor?id=9',
  );
});
