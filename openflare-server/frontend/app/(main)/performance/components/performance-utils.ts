export type PerformanceFields = {
  OpenRestyDefaultServerReturnStatus: string
  OpenRestyWorkerProcesses: string
  OpenRestyWorkerConnections: string
  OpenRestyWorkerRlimitNofile: string
  OpenRestyEventsUse: string
  OpenRestyEventsMultiAcceptEnabled: boolean
  OpenRestyKeepaliveTimeout: string
  OpenRestyKeepaliveRequests: string
  OpenRestyClientHeaderTimeout: string
  OpenRestyClientBodyTimeout: string
  OpenRestyClientMaxBodySize: string
  OpenRestyLargeClientHeaderBuffers: string
  OpenRestySendTimeout: string
  OpenRestyProxyConnectTimeout: string
  OpenRestyProxySendTimeout: string
  OpenRestyProxyReadTimeout: string
  OpenRestyWebsocketEnabled: boolean
  OpenRestyHTTP3Enabled: boolean
  OpenRestyProxyRequestBufferingEnabled: boolean
  OpenRestyProxyBufferingEnabled: boolean
  OpenRestyProxyBuffers: string
  OpenRestyProxyBufferSize: string
  OpenRestyProxyBusyBuffersSize: string
  OpenRestyGzipEnabled: boolean
  OpenRestyGzipMinLength: string
  OpenRestyGzipCompLevel: string
  OpenRestyCacheEnabled: boolean
  OpenRestyCachePath: string
  OpenRestyCacheLevels: string
  OpenRestyCacheInactive: string
  OpenRestyCacheMaxSize: string
  OpenRestyCacheKeyTemplate: string
  OpenRestyCacheLockEnabled: boolean
  OpenRestyCacheLockTimeout: string
  OpenRestyCacheUseStale: string
  OpenRestyResolvers: string
}

export const defaultPerformanceFields: PerformanceFields = {
  OpenRestyDefaultServerReturnStatus: "421",
  OpenRestyWorkerProcesses: "auto",
  OpenRestyWorkerConnections: "4096",
  OpenRestyWorkerRlimitNofile: "65535",
  OpenRestyEventsUse: "epoll",
  OpenRestyEventsMultiAcceptEnabled: true,
  OpenRestyKeepaliveTimeout: "20",
  OpenRestyKeepaliveRequests: "1000",
  OpenRestyClientHeaderTimeout: "15",
  OpenRestyClientBodyTimeout: "15",
  OpenRestyClientMaxBodySize: "64m",
  OpenRestyLargeClientHeaderBuffers: "4 16k",
  OpenRestySendTimeout: "30",
  OpenRestyProxyConnectTimeout: "3",
  OpenRestyProxySendTimeout: "60",
  OpenRestyProxyReadTimeout: "60",
  OpenRestyWebsocketEnabled: true,
  OpenRestyHTTP3Enabled: true,
  OpenRestyProxyRequestBufferingEnabled: false,
  OpenRestyProxyBufferingEnabled: true,
  OpenRestyProxyBuffers: "16 16k",
  OpenRestyProxyBufferSize: "8k",
  OpenRestyProxyBusyBuffersSize: "64k",
  OpenRestyGzipEnabled: true,
  OpenRestyGzipMinLength: "1024",
  OpenRestyGzipCompLevel: "5",
  OpenRestyCacheEnabled: false,
  OpenRestyCachePath: "",
  OpenRestyCacheLevels: "1:2",
  OpenRestyCacheInactive: "30m",
  OpenRestyCacheMaxSize: "1g",
  OpenRestyCacheKeyTemplate: "$scheme$host$request_uri",
  OpenRestyCacheLockEnabled: true,
  OpenRestyCacheLockTimeout: "5s",
  OpenRestyCacheUseStale:
    "error timeout updating http_500 http_502 http_503 http_504",
  OpenRestyResolvers: "",
}

export function optionsToMap(options: Array<{ key: string; value: string }>) {
  return options.reduce<Record<string, string>>((acc, option) => {
    acc[option.key] = option.value
    return acc
  }, {})
}

export function toBoolean(value: string | undefined, fallback: boolean) {
  if (value === undefined) return fallback
  return value === "true"
}

export function mapOptionsToFields(
  optionMap: Record<string, string>,
): PerformanceFields {
  return {
    OpenRestyDefaultServerReturnStatus:
      optionMap.OpenRestyDefaultServerReturnStatus ?? "421",
    OpenRestyWorkerProcesses: optionMap.OpenRestyWorkerProcesses ?? "auto",
    OpenRestyWorkerConnections: optionMap.OpenRestyWorkerConnections ?? "4096",
    OpenRestyWorkerRlimitNofile:
      optionMap.OpenRestyWorkerRlimitNofile ?? "65535",
    OpenRestyEventsUse: optionMap.OpenRestyEventsUse ?? "epoll",
    OpenRestyEventsMultiAcceptEnabled: toBoolean(
      optionMap.OpenRestyEventsMultiAcceptEnabled,
      true,
    ),
    OpenRestyKeepaliveTimeout: optionMap.OpenRestyKeepaliveTimeout ?? "20",
    OpenRestyKeepaliveRequests: optionMap.OpenRestyKeepaliveRequests ?? "1000",
    OpenRestyClientHeaderTimeout:
      optionMap.OpenRestyClientHeaderTimeout ?? "15",
    OpenRestyClientBodyTimeout: optionMap.OpenRestyClientBodyTimeout ?? "15",
    OpenRestyClientMaxBodySize: optionMap.OpenRestyClientMaxBodySize ?? "64m",
    OpenRestyLargeClientHeaderBuffers:
      optionMap.OpenRestyLargeClientHeaderBuffers ?? "4 16k",
    OpenRestySendTimeout: optionMap.OpenRestySendTimeout ?? "30",
    OpenRestyProxyConnectTimeout: optionMap.OpenRestyProxyConnectTimeout ?? "3",
    OpenRestyProxySendTimeout: optionMap.OpenRestyProxySendTimeout ?? "60",
    OpenRestyProxyReadTimeout: optionMap.OpenRestyProxyReadTimeout ?? "60",
    OpenRestyWebsocketEnabled: toBoolean(
      optionMap.OpenRestyWebsocketEnabled,
      true,
    ),
    OpenRestyHTTP3Enabled: toBoolean(optionMap.OpenRestyHTTP3Enabled, false),
    OpenRestyProxyRequestBufferingEnabled: toBoolean(
      optionMap.OpenRestyProxyRequestBufferingEnabled,
      false,
    ),
    OpenRestyProxyBufferingEnabled: toBoolean(
      optionMap.OpenRestyProxyBufferingEnabled,
      true,
    ),
    OpenRestyProxyBuffers: optionMap.OpenRestyProxyBuffers ?? "16 16k",
    OpenRestyProxyBufferSize: optionMap.OpenRestyProxyBufferSize ?? "8k",
    OpenRestyProxyBusyBuffersSize:
      optionMap.OpenRestyProxyBusyBuffersSize ?? "64k",
    OpenRestyGzipEnabled: toBoolean(optionMap.OpenRestyGzipEnabled, true),
    OpenRestyGzipMinLength: optionMap.OpenRestyGzipMinLength ?? "1024",
    OpenRestyGzipCompLevel: optionMap.OpenRestyGzipCompLevel ?? "5",
    OpenRestyCacheEnabled: toBoolean(optionMap.OpenRestyCacheEnabled, false),
    OpenRestyCachePath: optionMap.OpenRestyCachePath ?? "",
    OpenRestyCacheLevels: optionMap.OpenRestyCacheLevels ?? "1:2",
    OpenRestyCacheInactive: optionMap.OpenRestyCacheInactive ?? "30m",
    OpenRestyCacheMaxSize: optionMap.OpenRestyCacheMaxSize ?? "1g",
    OpenRestyCacheKeyTemplate:
      optionMap.OpenRestyCacheKeyTemplate ?? "$scheme$host$request_uri",
    OpenRestyCacheLockEnabled: toBoolean(
      optionMap.OpenRestyCacheLockEnabled,
      true,
    ),
    OpenRestyCacheLockTimeout: optionMap.OpenRestyCacheLockTimeout ?? "5s",
    OpenRestyCacheUseStale:
      optionMap.OpenRestyCacheUseStale ??
      "error timeout updating http_500 http_502 http_503 http_504",
    OpenRestyResolvers: optionMap.OpenRestyResolvers ?? "",
  }
}

function isPositiveInteger(value: string) {
  const parsed = Number.parseInt(value, 10)
  return !Number.isNaN(parsed) && parsed > 0
}

function isSizeValue(value: string) {
  return /^\d+[kKmMgG]?$/.test(value.trim())
}

function isProxyBuffersValue(value: string) {
  return /^\d+\s+\d+[kKmMgG]?$/.test(value.trim())
}

function isDurationToken(value: string) {
  return /^\d+[smhdwSMHDW]$/.test(value.trim())
}

function isCacheLevelsValue(value: string) {
  return /^\d{1,2}(?::\d{1,2}){0,2}$/.test(value.trim())
}

export function validateRuntimeFields(fields: PerformanceFields) {
  if (
    fields.OpenRestyWorkerProcesses !== "auto" &&
    !isPositiveInteger(fields.OpenRestyWorkerProcesses)
  ) {
    throw new Error("worker_processes 必须为 auto 或大于 0 的整数")
  }
  const integers = [
    fields.OpenRestyWorkerConnections,
    fields.OpenRestyWorkerRlimitNofile,
    fields.OpenRestyKeepaliveTimeout,
    fields.OpenRestyKeepaliveRequests,
    fields.OpenRestyClientHeaderTimeout,
    fields.OpenRestyClientBodyTimeout,
    fields.OpenRestySendTimeout,
  ]
  if (integers.some((value) => !isPositiveInteger(value))) {
    throw new Error("超时与连接参数必须为大于 0 的整数")
  }
  const status = Number.parseInt(fields.OpenRestyDefaultServerReturnStatus, 10)
  if (Number.isNaN(status) || status < 100 || status > 999) {
    throw new Error("空白页面返回状态码必须在 100 到 999 之间")
  }
  if (!isSizeValue(fields.OpenRestyClientMaxBodySize)) {
    throw new Error("client_max_body_size 格式不合法")
  }
  if (!isProxyBuffersValue(fields.OpenRestyLargeClientHeaderBuffers)) {
    throw new Error('large_client_header_buffers 格式必须类似 "4 16k"')
  }
}

export function validateProxyFields(fields: PerformanceFields) {
  const timeouts = [
    fields.OpenRestyProxyConnectTimeout,
    fields.OpenRestyProxySendTimeout,
    fields.OpenRestyProxyReadTimeout,
  ]
  if (timeouts.some((value) => !isPositiveInteger(value))) {
    throw new Error("代理超时参数必须为大于 0 的整数秒")
  }
  if (!isProxyBuffersValue(fields.OpenRestyProxyBuffers)) {
    throw new Error('proxy_buffers 格式必须类似 "16 16k"')
  }
  if (
    !isSizeValue(fields.OpenRestyProxyBufferSize) ||
    !isSizeValue(fields.OpenRestyProxyBusyBuffersSize)
  ) {
    throw new Error("缓冲大小必须为整数或带 k/m/g 单位的值")
  }
}

export function validateGzipFields(fields: PerformanceFields) {
  if (!isPositiveInteger(fields.OpenRestyGzipMinLength)) {
    throw new Error("gzip_min_length 必须为大于 0 的整数")
  }
  const level = Number.parseInt(fields.OpenRestyGzipCompLevel, 10)
  if (Number.isNaN(level) || level < 1 || level > 9) {
    throw new Error("gzip_comp_level 必须在 1 到 9 之间")
  }
}

export function validateCacheFields(fields: PerformanceFields) {
  if (!fields.OpenRestyCacheEnabled) return
  if (!fields.OpenRestyCachePath.trim()) {
    throw new Error("启用缓存时必须填写 proxy_cache_path 目录")
  }
  if (
    !isCacheLevelsValue(fields.OpenRestyCacheLevels) ||
    !isDurationToken(fields.OpenRestyCacheInactive) ||
    !isSizeValue(fields.OpenRestyCacheMaxSize) ||
    !isDurationToken(fields.OpenRestyCacheLockTimeout)
  ) {
    throw new Error("缓存 levels、inactive、max_size 或 lock_timeout 格式不合法")
  }
  if (!fields.OpenRestyCacheKeyTemplate.trim()) {
    throw new Error("启用缓存时必须填写缓存 Key 模板")
  }
}

export function entriesFromKeys(
  fields: PerformanceFields,
  keys: Array<keyof PerformanceFields>,
): Array<{ key: string; value: string }> {
  return keys.map((key) => ({
    key,
    value:
      typeof fields[key] === "boolean"
        ? String(fields[key])
        : String(fields[key]).trim(),
  }))
}