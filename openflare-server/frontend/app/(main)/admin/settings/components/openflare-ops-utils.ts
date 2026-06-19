import type {OptionItem} from "@/lib/services/openflare"

export type OpenFlareOpsFields = {
  AgentHeartbeatInterval: string
  AgentWebsocketUpgradeEnabled: boolean
  NodeOfflineThreshold: string
  AgentUpdateRepo: string
  GeoIPProvider: string
  ServerAddress: string
  UptimeKumaEnabled: boolean
  UptimeKumaUrl: string
  UptimeKumaUsername: string
  UptimeKumaPassword: string
  UptimeKumaMonitorScope: string
  UptimeKumaSelectedSites: string
  UptimeKumaSyncInterval: string
  UptimeKumaInterval: string
  UptimeKumaRetry: string
  UptimeKumaRetryInterval: string
  UptimeKumaTimeout: string
  DatabaseAutoCleanupEnabled: boolean
  DatabaseAutoCleanupRetentionDays: string
}

export const defaultOpenFlareOpsFields: OpenFlareOpsFields = {
  AgentHeartbeatInterval: "10000",
  AgentWebsocketUpgradeEnabled: true,
  NodeOfflineThreshold: "120000",
  AgentUpdateRepo: "Rain-kl/OpenFlare",
  GeoIPProvider: "ipinfo",
  ServerAddress: "",
  UptimeKumaEnabled: false,
  UptimeKumaUrl: "",
  UptimeKumaUsername: "",
  UptimeKumaPassword: "",
  UptimeKumaMonitorScope: "all",
  UptimeKumaSelectedSites: "",
  UptimeKumaSyncInterval: "5",
  UptimeKumaInterval: "60",
  UptimeKumaRetry: "0",
  UptimeKumaRetryInterval: "60",
  UptimeKumaTimeout: "48",
  DatabaseAutoCleanupEnabled: false,
  DatabaseAutoCleanupRetentionDays: "30",
}

export const INSTALLER_SCRIPT_URL =
  "https://raw.githubusercontent.com/Rain-kl/OpenFlare/main/scripts/install-agent.sh"

export function optionsToMap(options: OptionItem[]) {
  return options.reduce<Record<string, string>>((accumulator, option) => {
    accumulator[option.key] = option.value
    return accumulator
  }, {})
}

function toBoolean(value: string | undefined, fallback: boolean) {
  if (value === undefined) return fallback
  return value === "true"
}

export function mapOptionsToOpsFields(
  optionMap: Record<string, string>,
  serverAddress = "",
): OpenFlareOpsFields {
  return {
    AgentHeartbeatInterval: optionMap.AgentHeartbeatInterval ?? "10000",
    AgentWebsocketUpgradeEnabled: toBoolean(optionMap.AgentWebsocketUpgradeEnabled, true),
    NodeOfflineThreshold: optionMap.NodeOfflineThreshold ?? "120000",
    AgentUpdateRepo: optionMap.AgentUpdateRepo ?? "Rain-kl/OpenFlare",
    GeoIPProvider: optionMap.GeoIPProvider ?? "ipinfo",
    ServerAddress: optionMap.ServerAddress || serverAddress,
    UptimeKumaEnabled: toBoolean(optionMap.UptimeKumaEnabled, false),
    UptimeKumaUrl: optionMap.UptimeKumaUrl ?? "",
    UptimeKumaUsername: optionMap.UptimeKumaUsername ?? "",
    UptimeKumaPassword: "",
    UptimeKumaMonitorScope: optionMap.UptimeKumaMonitorScope ?? "all",
    UptimeKumaSelectedSites: optionMap.UptimeKumaSelectedSites ?? "",
    UptimeKumaSyncInterval: optionMap.UptimeKumaSyncInterval ?? "5",
    UptimeKumaInterval: optionMap.UptimeKumaInterval ?? "60",
    UptimeKumaRetry: optionMap.UptimeKumaRetry ?? "0",
    UptimeKumaRetryInterval: optionMap.UptimeKumaRetryInterval ?? "60",
    UptimeKumaTimeout: optionMap.UptimeKumaTimeout ?? "48",
    DatabaseAutoCleanupEnabled: toBoolean(optionMap.DatabaseAutoCleanupEnabled, false),
    DatabaseAutoCleanupRetentionDays: optionMap.DatabaseAutoCleanupRetentionDays ?? "30",
  }
}

export function formatDurationLabel(value: string) {
  const milliseconds = Number.parseInt(value, 10)
  if (Number.isNaN(milliseconds)) return value
  if (milliseconds >= 60000) return `${milliseconds / 60000} 分钟`
  return `${milliseconds / 1000} 秒`
}

export function normalizeServerUrl(value: string) {
  return value.trim().replace(/\/+$/, "")
}

export function getBrowserOrigin() {
  if (typeof window === "undefined") return ""
  return normalizeServerUrl(window.location.origin)
}

export function buildDiscoveryCommand(serverUrl: string, discoveryToken: string) {
  return [
    `curl -fsSL ${INSTALLER_SCRIPT_URL} | bash -s -- \\`,
    `  --server-url ${normalizeServerUrl(serverUrl)} \\`,
    `  --discovery-token ${discoveryToken}`,
  ].join("\n")
}

export function validateAgentFields(fields: OpenFlareOpsFields) {
  const heartbeat = Number.parseInt(fields.AgentHeartbeatInterval, 10)
  const offline = Number.parseInt(fields.NodeOfflineThreshold, 10)
  if (Number.isNaN(heartbeat) || heartbeat < 5000) {
    throw new Error("心跳间隔不能小于 5000 毫秒。")
  }
  if (Number.isNaN(offline) || offline < 10000) {
    throw new Error("离线阈值不能小于 10000 毫秒。")
  }
}

export function validateUptimeKumaFields(fields: OpenFlareOpsFields) {
  const syncInt = Number.parseInt(fields.UptimeKumaSyncInterval, 10)
  const interval = Number.parseInt(fields.UptimeKumaInterval, 10)
  const retry = Number.parseInt(fields.UptimeKumaRetry, 10)
  const retryInt = Number.parseInt(fields.UptimeKumaRetryInterval, 10)
  const timeout = Number.parseInt(fields.UptimeKumaTimeout, 10)

  if (fields.UptimeKumaEnabled) {
    if (!fields.UptimeKumaUrl.trim()) throw new Error("请输入 Uptime Kuma 地址。")
    if (!fields.UptimeKumaUsername.trim()) throw new Error("请输入 Uptime Kuma 用户名。")
  }
  if (Number.isNaN(syncInt) || syncInt <= 0) throw new Error("同步间隔必须为正整数。")
  if (Number.isNaN(interval) || interval <= 0) throw new Error("心跳间隔必须为正整数。")
  if (Number.isNaN(retry) || retry < 0) throw new Error("重试次数必须为非负整数。")
  if (Number.isNaN(retryInt) || retryInt <= 0) throw new Error("心跳重试间隔必须为正整数。")
  if (Number.isNaN(timeout) || timeout <= 0) throw new Error("请求超时必须为正整数。")
}

export function validateDatabaseAutoCleanup(fields: OpenFlareOpsFields) {
  const retentionDays = Number.parseInt(fields.DatabaseAutoCleanupRetentionDays, 10)
  if (Number.isNaN(retentionDays) || retentionDays < 1) {
    throw new Error("自动清理保留天数至少为 1 天。")
  }
}

export function agentOptionEntries(fields: OpenFlareOpsFields): OptionItem[] {
  validateAgentFields(fields)
  return [
    { key: "AgentHeartbeatInterval", value: fields.AgentHeartbeatInterval },
    { key: "AgentWebsocketUpgradeEnabled", value: String(fields.AgentWebsocketUpgradeEnabled) },
    { key: "NodeOfflineThreshold", value: fields.NodeOfflineThreshold },
    { key: "AgentUpdateRepo", value: fields.AgentUpdateRepo.trim() },
    { key: "GeoIPProvider", value: fields.GeoIPProvider },
  ]
}

export function uptimeKumaOptionEntries(fields: OpenFlareOpsFields): OptionItem[] {
  validateUptimeKumaFields(fields)
  return [
    { key: "UptimeKumaEnabled", value: String(fields.UptimeKumaEnabled) },
    { key: "UptimeKumaUrl", value: fields.UptimeKumaUrl.trim() },
    { key: "UptimeKumaUsername", value: fields.UptimeKumaUsername.trim() },
    { key: "UptimeKumaPassword", value: fields.UptimeKumaPassword },
    { key: "UptimeKumaMonitorScope", value: fields.UptimeKumaMonitorScope },
    { key: "UptimeKumaSelectedSites", value: fields.UptimeKumaSelectedSites },
    { key: "UptimeKumaSyncInterval", value: fields.UptimeKumaSyncInterval },
    { key: "UptimeKumaInterval", value: fields.UptimeKumaInterval },
    { key: "UptimeKumaRetry", value: fields.UptimeKumaRetry },
    { key: "UptimeKumaRetryInterval", value: fields.UptimeKumaRetryInterval },
    { key: "UptimeKumaTimeout", value: fields.UptimeKumaTimeout },
  ]
}

export function databaseAutoCleanupEntries(fields: OpenFlareOpsFields): OptionItem[] {
  validateDatabaseAutoCleanup(fields)
  return [
    { key: "DatabaseAutoCleanupEnabled", value: String(fields.DatabaseAutoCleanupEnabled) },
    { key: "DatabaseAutoCleanupRetentionDays", value: fields.DatabaseAutoCleanupRetentionDays },
  ]
}
