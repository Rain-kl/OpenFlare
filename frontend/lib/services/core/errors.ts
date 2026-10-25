import { locales } from '@/i18n/config';
import { getRuntimeTranslator, runtimeText } from '@/i18n/runtime';
/**
 * API 错误类型定义
 */

/**
 * API 错误基类
 */
export class ApiErrorBase extends Error {
  constructor(
    message: string,
    public readonly code?: string,
    public readonly statusCode?: number,
    public readonly details?: unknown,
  ) {
    super(message);
    this.name = 'ApiError';
    Object.setPrototypeOf(this, ApiErrorBase.prototype);
  }
}

/**
 * 网络错误
 */
export class NetworkError extends ApiErrorBase {
  constructor(
    message = runtimeText(
      'network_connection_failed_check_your_network_cbf47b07',
    ),
  ) {
    super(message);
    this.name = 'NetworkError';
    Object.setPrototypeOf(this, NetworkError.prototype);
  }
}

/**
 * 超时错误
 */
export class TimeoutError extends ApiErrorBase {
  constructor(
    message = runtimeText('request_timed_out_try_again_later_6c3465f6'),
  ) {
    super(message);
    this.name = 'TimeoutError';
    Object.setPrototypeOf(this, TimeoutError.prototype);
  }
}

/**
 * 未授权错误 (401)
 */
export class UnauthorizedError extends ApiErrorBase {
  constructor(message = runtimeText('unauthorized_sign_in_first_466879f5')) {
    super(message, 'UNAUTHORIZED', 401);
    this.name = 'UnauthorizedError';
    Object.setPrototypeOf(this, UnauthorizedError.prototype);
  }
}

/**
 * 权限不足错误 (403)
 */
export class ForbiddenError extends ApiErrorBase {
  constructor(
    message = runtimeText('insufficient_permissions_5e3c8a26'),
    code = 'FORBIDDEN',
    details?: unknown,
  ) {
    super(message, code, 403, details);
    this.name = 'ForbiddenError';
    Object.setPrototypeOf(this, ForbiddenError.prototype);
  }
}

/**
 * 资源未找到错误 (404)
 */
export class NotFoundError extends ApiErrorBase {
  constructor(
    message = runtimeText('the_requested_resource_was_not_found_aecc0681'),
  ) {
    super(message, 'NOT_FOUND', 404);
    this.name = 'NotFoundError';
    Object.setPrototypeOf(this, NotFoundError.prototype);
  }
}

/**
 * 服务器错误 (5xx)
 */
export class ServerError extends ApiErrorBase {
  constructor(
    message = runtimeText('internal_server_error_try_again_later_4f0dd7e9'),
    statusCode = 500,
  ) {
    super(message, 'SERVER_ERROR', statusCode);
    this.name = 'ServerError';
    Object.setPrototypeOf(this, ServerError.prototype);
  }
}

/**
 * 验证错误 (400)
 */
export class ValidationError extends ApiErrorBase {
  constructor(
    message = runtimeText('request_parameter_validation_failed_60ef7c51'),
    details?: unknown,
  ) {
    super(message, 'VALIDATION_ERROR', 400, details);
    this.name = 'ValidationError';
    Object.setPrototypeOf(this, ValidationError.prototype);
  }
}

/**
 * 检查错误是否为请求取消错误
 * @param error - 错误对象
 * @returns 是否为取消错误
 */
export function isCancelError(error: unknown): boolean {
  return (
    error !== null &&
    typeof error === 'object' &&
    (('__CANCEL__' in error &&
      (error as { __CANCEL__?: boolean }).__CANCEL__ === true) ||
      ('message' in error &&
        locales.some(
          (locale) =>
            (error as { message?: string }).message ===
            getRuntimeTranslator(locale)('request_canceled_5892f9b1'),
        )))
  );
}
