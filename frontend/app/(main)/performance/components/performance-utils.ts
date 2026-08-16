export type PerformanceFields = {
  openresty_default_server_return_status: string;
  openresty_worker_processes: string;
  openresty_worker_connections: string;
  openresty_worker_rlimit_nofile: string;
  openresty_events_use: string;
  openresty_events_multi_accept_enabled: boolean;
  openresty_keepalive_timeout: string;
  openresty_keepalive_requests: string;
  openresty_client_header_timeout: string;
  openresty_client_body_timeout: string;
  openresty_client_max_body_size: string;
  openresty_large_client_header_buffers: string;
  openresty_send_timeout: string;
  openresty_proxy_connect_timeout: string;
  openresty_proxy_send_timeout: string;
  openresty_proxy_read_timeout: string;
  openresty_websocket_enabled: boolean;
  openresty_http3_enabled: boolean;
  openresty_proxy_request_buffering_enabled: boolean;
  openresty_proxy_buffering_enabled: boolean;
  openresty_proxy_buffers: string;
  openresty_proxy_buffer_size: string;
  openresty_proxy_busy_buffers_size: string;
  openresty_gzip_enabled: boolean;
  openresty_gzip_min_length: string;
  openresty_gzip_comp_level: string;
  openresty_cache_enabled: boolean;
  openresty_cache_path: string;
  openresty_cache_levels: string;
  openresty_cache_inactive: string;
  openresty_cache_max_size: string;
  openresty_cache_key_template: string;
  openresty_cache_lock_enabled: boolean;
  openresty_cache_lock_timeout: string;
  openresty_cache_use_stale: string;
  openresty_resolvers: string;
};

export const defaultPerformanceFields: PerformanceFields = {
  openresty_default_server_return_status: '421',
  openresty_worker_processes: 'auto',
  openresty_worker_connections: '4096',
  openresty_worker_rlimit_nofile: '65535',
  openresty_events_use: 'epoll',
  openresty_events_multi_accept_enabled: true,
  openresty_keepalive_timeout: '20',
  openresty_keepalive_requests: '1000',
  openresty_client_header_timeout: '15',
  openresty_client_body_timeout: '15',
  openresty_client_max_body_size: '64m',
  openresty_large_client_header_buffers: '4 16k',
  openresty_send_timeout: '30',
  openresty_proxy_connect_timeout: '3',
  openresty_proxy_send_timeout: '60',
  openresty_proxy_read_timeout: '60',
  openresty_websocket_enabled: true,
  openresty_http3_enabled: true,
  openresty_proxy_request_buffering_enabled: false,
  openresty_proxy_buffering_enabled: true,
  openresty_proxy_buffers: '16 16k',
  openresty_proxy_buffer_size: '8k',
  openresty_proxy_busy_buffers_size: '64k',
  openresty_gzip_enabled: true,
  openresty_gzip_min_length: '1024',
  openresty_gzip_comp_level: '5',
  openresty_cache_enabled: false,
  openresty_cache_path: '',
  openresty_cache_levels: '1:2',
  openresty_cache_inactive: '30m',
  openresty_cache_max_size: '1g',
  openresty_cache_key_template: '$scheme$host$request_uri',
  openresty_cache_lock_enabled: true,
  openresty_cache_lock_timeout: '5s',
  openresty_cache_use_stale:
    'error timeout updating http_500 http_502 http_503 http_504',
  openresty_resolvers: '',
};

export function optionsToMap(options: Array<{ key: string; value: string }>) {
  return options.reduce<Record<string, string>>((acc, option) => {
    acc[option.key] = option.value;
    return acc;
  }, {});
}

export function toBoolean(value: string | undefined, fallback: boolean) {
  if (value === undefined) return fallback;
  return value === 'true';
}

export function mapOptionsToFields(
  optionMap: Record<string, string>,
): PerformanceFields {
  return {
    openresty_default_server_return_status:
      optionMap.openresty_default_server_return_status ?? '421',
    openresty_worker_processes: optionMap.openresty_worker_processes ?? 'auto',
    openresty_worker_connections:
      optionMap.openresty_worker_connections ?? '4096',
    openresty_worker_rlimit_nofile:
      optionMap.openresty_worker_rlimit_nofile ?? '65535',
    openresty_events_use: optionMap.openresty_events_use ?? 'epoll',
    openresty_events_multi_accept_enabled: toBoolean(
      optionMap.openresty_events_multi_accept_enabled,
      true,
    ),
    openresty_keepalive_timeout: optionMap.openresty_keepalive_timeout ?? '20',
    openresty_keepalive_requests:
      optionMap.openresty_keepalive_requests ?? '1000',
    openresty_client_header_timeout:
      optionMap.openresty_client_header_timeout ?? '15',
    openresty_client_body_timeout:
      optionMap.openresty_client_body_timeout ?? '15',
    openresty_client_max_body_size:
      optionMap.openresty_client_max_body_size ?? '64m',
    openresty_large_client_header_buffers:
      optionMap.openresty_large_client_header_buffers ?? '4 16k',
    openresty_send_timeout: optionMap.openresty_send_timeout ?? '30',
    openresty_proxy_connect_timeout:
      optionMap.openresty_proxy_connect_timeout ?? '3',
    openresty_proxy_send_timeout:
      optionMap.openresty_proxy_send_timeout ?? '60',
    openresty_proxy_read_timeout:
      optionMap.openresty_proxy_read_timeout ?? '60',
    openresty_websocket_enabled: toBoolean(
      optionMap.openresty_websocket_enabled,
      true,
    ),
    openresty_http3_enabled: toBoolean(
      optionMap.openresty_http3_enabled,
      false,
    ),
    openresty_proxy_request_buffering_enabled: toBoolean(
      optionMap.openresty_proxy_request_buffering_enabled,
      false,
    ),
    openresty_proxy_buffering_enabled: toBoolean(
      optionMap.openresty_proxy_buffering_enabled,
      true,
    ),
    openresty_proxy_buffers: optionMap.openresty_proxy_buffers ?? '16 16k',
    openresty_proxy_buffer_size: optionMap.openresty_proxy_buffer_size ?? '8k',
    openresty_proxy_busy_buffers_size:
      optionMap.openresty_proxy_busy_buffers_size ?? '64k',
    openresty_gzip_enabled: toBoolean(optionMap.openresty_gzip_enabled, true),
    openresty_gzip_min_length: optionMap.openresty_gzip_min_length ?? '1024',
    openresty_gzip_comp_level: optionMap.openresty_gzip_comp_level ?? '5',
    openresty_cache_enabled: toBoolean(
      optionMap.openresty_cache_enabled,
      false,
    ),
    openresty_cache_path: optionMap.openresty_cache_path ?? '',
    openresty_cache_levels: optionMap.openresty_cache_levels ?? '1:2',
    openresty_cache_inactive: optionMap.openresty_cache_inactive ?? '30m',
    openresty_cache_max_size: optionMap.openresty_cache_max_size ?? '1g',
    openresty_cache_key_template:
      optionMap.openresty_cache_key_template ?? '$scheme$host$request_uri',
    openresty_cache_lock_enabled: toBoolean(
      optionMap.openresty_cache_lock_enabled,
      true,
    ),
    openresty_cache_lock_timeout:
      optionMap.openresty_cache_lock_timeout ?? '5s',
    openresty_cache_use_stale:
      optionMap.openresty_cache_use_stale ??
      'error timeout updating http_500 http_502 http_503 http_504',
    openresty_resolvers: optionMap.openresty_resolvers ?? '',
  };
}

function isPositiveInteger(value: string) {
  const parsed = Number.parseInt(value, 10);
  return !Number.isNaN(parsed) && parsed > 0;
}

function isSizeValue(value: string) {
  return /^\d+[kKmMgG]?$/.test(value.trim());
}

function isProxyBuffersValue(value: string) {
  return /^\d+\s+\d+[kKmMgG]?$/.test(value.trim());
}

function isDurationToken(value: string) {
  return /^\d+[smhdwSMHDW]$/.test(value.trim());
}

function isCacheLevelsValue(value: string) {
  return /^\d{1,2}(?::\d{1,2}){0,2}$/.test(value.trim());
}

type PerformanceMessageT = (key: string) => string;

export function validateRuntimeFields(
  fields: PerformanceFields,
  t: PerformanceMessageT,
) {
  if (
    fields.openresty_worker_processes !== 'auto' &&
    !isPositiveInteger(fields.openresty_worker_processes)
  ) {
    throw new Error(t('errors.workerProcesses'));
  }
  const integers = [
    fields.openresty_worker_connections,
    fields.openresty_worker_rlimit_nofile,
    fields.openresty_keepalive_timeout,
    fields.openresty_keepalive_requests,
    fields.openresty_client_header_timeout,
    fields.openresty_client_body_timeout,
    fields.openresty_send_timeout,
  ];
  if (integers.some((value) => !isPositiveInteger(value))) {
    throw new Error(t('errors.timeouts'));
  }
  const status = Number.parseInt(
    fields.openresty_default_server_return_status,
    10,
  );
  if (Number.isNaN(status) || status < 100 || status > 999) {
    throw new Error(t('errors.statusCode'));
  }
  if (!isSizeValue(fields.openresty_client_max_body_size)) {
    throw new Error(t('errors.bodySize'));
  }
  if (!isProxyBuffersValue(fields.openresty_large_client_header_buffers)) {
    throw new Error(t('errors.headerBuffers'));
  }
}

export function validateProxyFields(
  fields: PerformanceFields,
  t: PerformanceMessageT,
) {
  const timeouts = [
    fields.openresty_proxy_connect_timeout,
    fields.openresty_proxy_send_timeout,
    fields.openresty_proxy_read_timeout,
  ];
  if (timeouts.some((value) => !isPositiveInteger(value))) {
    throw new Error(t('errors.proxyTimeouts'));
  }
  if (!isProxyBuffersValue(fields.openresty_proxy_buffers)) {
    throw new Error(t('errors.proxyBuffers'));
  }
  if (
    !isSizeValue(fields.openresty_proxy_buffer_size) ||
    !isSizeValue(fields.openresty_proxy_busy_buffers_size)
  ) {
    throw new Error(t('errors.bufferSize'));
  }
}

export function validateGzipFields(
  fields: PerformanceFields,
  t: PerformanceMessageT,
) {
  if (!isPositiveInteger(fields.openresty_gzip_min_length)) {
    throw new Error(t('errors.gzipMinLength'));
  }
  const level = Number.parseInt(fields.openresty_gzip_comp_level, 10);
  if (Number.isNaN(level) || level < 1 || level > 9) {
    throw new Error(t('errors.gzipLevel'));
  }
}

export function validateCacheFields(
  fields: PerformanceFields,
  t: PerformanceMessageT,
) {
  if (!fields.openresty_cache_enabled) return;
  if (!fields.openresty_cache_path.trim()) {
    throw new Error(t('errors.cachePath'));
  }
  if (
    !isCacheLevelsValue(fields.openresty_cache_levels) ||
    !isDurationToken(fields.openresty_cache_inactive) ||
    !isSizeValue(fields.openresty_cache_max_size) ||
    !isDurationToken(fields.openresty_cache_lock_timeout)
  ) {
    throw new Error(t('errors.cacheFormat'));
  }
  if (!fields.openresty_cache_key_template.trim()) {
    throw new Error(t('errors.cacheKey'));
  }
}

export function entriesFromKeys(
  fields: PerformanceFields,
  keys: Array<keyof PerformanceFields>,
): Array<{ key: string; value: string }> {
  return keys.map((key) => ({
    key,
    value:
      typeof fields[key] === 'boolean'
        ? String(fields[key])
        : String(fields[key]).trim(),
  }));
}
