import '@testing-library/jest-dom/vitest';
import { vi } from 'vitest';

const toastMock = vi.fn((msg) => {
  const el = document.createElement('div');
  el.textContent = typeof msg === 'string' ? msg : '';
  document.body.appendChild(el);
  return 'toast-id';
});

vi.mock('sonner', () => ({
  toast: Object.assign(toastMock, {
    success: vi.fn((msg) => {
      const el = document.createElement('div');
      el.textContent = typeof msg === 'string' ? msg : '';
      document.body.appendChild(el);
      return 'toast-id';
    }),
    error: vi.fn((msg) => {
      const el = document.createElement('div');
      el.textContent = typeof msg === 'string' ? msg : '';
      document.body.appendChild(el);
      return 'toast-id';
    }),
  }),
}));
