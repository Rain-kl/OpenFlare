import '@testing-library/jest-dom/vitest';

// input-otp（登录 OTP 组件）依赖 ResizeObserver，jsdom 未内置
class ResizeObserverMock {
  observe() {}
  unobserve() {}
  disconnect() {}
}
if (typeof globalThis.ResizeObserver === 'undefined') {
  globalThis.ResizeObserver =
    ResizeObserverMock as unknown as typeof ResizeObserver;
}
