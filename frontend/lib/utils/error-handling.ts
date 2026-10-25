import { runtimeText } from '@/i18n/runtime';
import { isCancelError } from '@/lib/services/core';
import { toast } from 'sonner';

/**
 * 错误处理选项
 */
interface HandleContextErrorOptions {
  /** 是否显示 toast 提示 */
  showToast?: boolean;
  /** 是否在控制台记录错误 */
  logError?: boolean;
  logMessage?: string;
}

/**
 * 统一处理 Context 中的错误
 *
 * @param error - 捕获的错误对象
 * @param defaultMessage - 默认错误消息
 * @param options - 错误处理选项
 * @returns 标准化的 Error 对象
 *
 * @example
 * ```typescript
 * try {
 *   await fetchData()
 * } catch (error) {
 *   const errorObject = handleContextError(error, '加载数据失败', {
 *     showToast: true,
 *     logError: true
 *   })
 *   setError(errorObject)
 * }
 * ```
 */
export function handleContextError(
  error: unknown,
  defaultMessage: string,
  options: HandleContextErrorOptions = {},
): Error {
  const {
    showToast = false,
    logError = true,
    logMessage = 'Context operation failed',
  } = options;

  // 取消的请求不算错误
  if (isCancelError(error)) {
    return new Error(runtimeText('request_canceled_282c4cfb'));
  }

  const errorMessage = error instanceof Error ? error.message : defaultMessage;
  const errorObject =
    error instanceof Error ? error : new Error(defaultMessage);

  if (logError) {
    console.error(logMessage, error);
  }

  if (showToast) {
    toast.error(defaultMessage, {
      description: errorMessage,
    });
  }

  return errorObject;
}
