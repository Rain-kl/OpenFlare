/**
 * OpenFlare 统一 API 响应信封解析。
 * 与后端 internal/common/response.Response 及 axios api-client 约定一致。
 */

export interface ApiEnvelope<T = unknown> {
  error_msg: string;
  data: T | null;
}

export class ApiEnvelopeError extends Error {
  readonly status: number;

  constructor(message: string, status: number) {
    super(message);
    this.name = 'ApiEnvelopeError';
    this.status = status;
  }
}

function hasEnvelopeShape(value: unknown): value is ApiEnvelope<unknown> {
  if (!value || typeof value !== 'object') {
    return false;
  }
  return 'error_msg' in value && 'data' in value;
}

/**
 * 解析 fetch 响应体中的 { error_msg, data } 信封。
 * - HTTP 非 2xx：优先使用 error_msg
 * - HTTP 200 但 error_msg 非空：视为业务失败
 */
export async function readApiEnvelope<T>(
  res: Response,
  fallbackMessage: string,
): Promise<ApiEnvelope<T>> {
  let body: unknown;
  try {
    body = await res.json();
  } catch {
    throw new ApiEnvelopeError(fallbackMessage, res.status);
  }

  if (!hasEnvelopeShape(body)) {
    throw new ApiEnvelopeError(fallbackMessage, res.status);
  }

  const envelope = body as ApiEnvelope<T>;
  if (!res.ok) {
    throw new ApiEnvelopeError(
      envelope.error_msg || fallbackMessage,
      res.status,
    );
  }
  if (envelope.error_msg) {
    throw new ApiEnvelopeError(envelope.error_msg, res.status);
  }

  return envelope;
}

export async function readApiData<T>(
  res: Response,
  fallbackMessage: string,
): Promise<T> {
  const envelope = await readApiEnvelope<T>(res, fallbackMessage);
  if (envelope.data == null) {
    throw new ApiEnvelopeError(fallbackMessage, res.status);
  }
  return envelope.data;
}
