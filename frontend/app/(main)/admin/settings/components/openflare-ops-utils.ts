import type { OptionItem } from '@/lib/services/openflare';

export type OpenFlareOpsFields = {
  agent_heartbeat_interval: string;
  agent_websocket_upgrade_enabled: boolean;
  node_offline_threshold: string;
  agent_update_repo: string;
  geoip_provider: string;
  server_address: string;
  uptime_kuma_enabled: boolean;
  uptime_kuma_url: string;
  uptime_kuma_username: string;
  uptime_kuma_password: string;
  uptime_kuma_monitor_scope: string;
  uptime_kuma_selected_sites: string;
  uptime_kuma_sync_interval: string;
  uptime_kuma_interval: string;
  uptime_kuma_retry: string;
  uptime_kuma_retry_interval: string;
  uptime_kuma_timeout: string;
  database_auto_cleanup_enabled: boolean;
  database_auto_cleanup_retention_days: string;
  pages_max_package_size_mb: string;
  pages_max_history_count: string;
};

export const defaultOpenFlareOpsFields: OpenFlareOpsFields = {
  agent_heartbeat_interval: '10000',
  agent_websocket_upgrade_enabled: true,
  node_offline_threshold: '120000',
  agent_update_repo: 'Rain-kl/OpenFlare',
  geoip_provider: 'ipinfo',
  server_address: '',
  uptime_kuma_enabled: false,
  uptime_kuma_url: '',
  uptime_kuma_username: '',
  uptime_kuma_password: '',
  uptime_kuma_monitor_scope: 'all',
  uptime_kuma_selected_sites: '',
  uptime_kuma_sync_interval: '5',
  uptime_kuma_interval: '60',
  uptime_kuma_retry: '0',
  uptime_kuma_retry_interval: '60',
  uptime_kuma_timeout: '48',
  database_auto_cleanup_enabled: false,
  database_auto_cleanup_retention_days: '30',
  pages_max_package_size_mb: '100',
  pages_max_history_count: '20',
};

export const INSTALLER_SCRIPT_URL =
  'https://raw.githubusercontent.com/Rain-kl/OpenFlare/main/scripts/install-agent.sh';

export function optionsToMap(options: OptionItem[]) {
  return options.reduce<Record<string, string>>((accumulator, option) => {
    accumulator[option.key] = option.value;
    return accumulator;
  }, {});
}

function toBoolean(value: string | undefined, fallback: boolean) {
  if (value === undefined) return fallback;
  return value === 'true';
}

export function mapOptionsToOpsFields(
  optionMap: Record<string, string>,
  serverAddress = '',
): OpenFlareOpsFields {
  return {
    agent_heartbeat_interval: optionMap.agent_heartbeat_interval ?? '10000',
    agent_websocket_upgrade_enabled: toBoolean(
      optionMap.agent_websocket_upgrade_enabled,
      true,
    ),
    node_offline_threshold: optionMap.node_offline_threshold ?? '120000',
    agent_update_repo: optionMap.agent_update_repo ?? 'Rain-kl/OpenFlare',
    geoip_provider: optionMap.geoip_provider ?? 'ipinfo',
    server_address: optionMap.server_address || serverAddress,
    uptime_kuma_enabled: toBoolean(optionMap.uptime_kuma_enabled, false),
    uptime_kuma_url: optionMap.uptime_kuma_url ?? '',
    uptime_kuma_username: optionMap.uptime_kuma_username ?? '',
    uptime_kuma_password: '',
    uptime_kuma_monitor_scope: optionMap.uptime_kuma_monitor_scope ?? 'all',
    uptime_kuma_selected_sites: optionMap.uptime_kuma_selected_sites ?? '',
    uptime_kuma_sync_interval: optionMap.uptime_kuma_sync_interval ?? '5',
    uptime_kuma_interval: optionMap.uptime_kuma_interval ?? '60',
    uptime_kuma_retry: optionMap.uptime_kuma_retry ?? '0',
    uptime_kuma_retry_interval: optionMap.uptime_kuma_retry_interval ?? '60',
    uptime_kuma_timeout: optionMap.uptime_kuma_timeout ?? '48',
    database_auto_cleanup_enabled: toBoolean(
      optionMap.database_auto_cleanup_enabled,
      false,
    ),
    database_auto_cleanup_retention_days:
      optionMap.database_auto_cleanup_retention_days ?? '30',
    pages_max_package_size_mb: optionMap.pages_max_package_size_mb ?? '100',
    pages_max_history_count: optionMap.pages_max_history_count ?? '20',
  };
}

export function formatDurationLabel(value: string) {
  const milliseconds = Number.parseInt(value, 10);
  if (Number.isNaN(milliseconds)) return value;
  if (milliseconds >= 60000) return `${milliseconds / 60000} 分钟`;
  return `${milliseconds / 1000} 秒`;
}

export function normalizeServerUrl(value: string) {
  return value.trim().replace(/\/+$/, '');
}

export function getBrowserOrigin() {
  if (typeof window === 'undefined') return '';
  return normalizeServerUrl(window.location.origin);
}

export function buildDiscoveryCommand(
  serverUrl: string,
  discoveryToken: string,
) {
  return [
    `curl -fsSL ${INSTALLER_SCRIPT_URL} | bash -s -- \\`,
    `  --server-url ${normalizeServerUrl(serverUrl)} \\`,
    `  --discovery-token ${discoveryToken}`,
  ].join('\n');
}

export function validateAgentFields(fields: OpenFlareOpsFields) {
  const heartbeat = Number.parseInt(fields.agent_heartbeat_interval, 10);
  const offline = Number.parseInt(fields.node_offline_threshold, 10);
  if (Number.isNaN(heartbeat) || heartbeat < 5000) {
    throw new Error('心跳间隔不能小于 5000 毫秒。');
  }
  if (Number.isNaN(offline) || offline < 10000) {
    throw new Error('离线阈值不能小于 10000 毫秒。');
  }
}

export function validateUptimeKumaFields(fields: OpenFlareOpsFields) {
  const syncInt = Number.parseInt(fields.uptime_kuma_sync_interval, 10);
  const interval = Number.parseInt(fields.uptime_kuma_interval, 10);
  const retry = Number.parseInt(fields.uptime_kuma_retry, 10);
  const retryInt = Number.parseInt(fields.uptime_kuma_retry_interval, 10);
  const timeout = Number.parseInt(fields.uptime_kuma_timeout, 10);

  if (fields.uptime_kuma_enabled) {
    if (!fields.uptime_kuma_url.trim())
      throw new Error('请输入 Uptime Kuma 地址。');
    if (!fields.uptime_kuma_username.trim())
      throw new Error('请输入 Uptime Kuma 用户名。');
  }
  if (Number.isNaN(syncInt) || syncInt <= 0)
    throw new Error('同步间隔必须为正整数。');
  if (Number.isNaN(interval) || interval <= 0)
    throw new Error('心跳间隔必须为正整数。');
  if (Number.isNaN(retry) || retry < 0)
    throw new Error('重试次数必须为非负整数。');
  if (Number.isNaN(retryInt) || retryInt <= 0)
    throw new Error('心跳重试间隔必须为正整数。');
  if (Number.isNaN(timeout) || timeout <= 0)
    throw new Error('请求超时必须为正整数。');
}

export function validateDatabaseAutoCleanup(fields: OpenFlareOpsFields) {
  const retentionDays = Number.parseInt(
    fields.database_auto_cleanup_retention_days,
    10,
  );
  if (Number.isNaN(retentionDays) || retentionDays < 1) {
    throw new Error('自动清理保留天数至少为 1 天。');
  }
}

export function agentOptionEntries(fields: OpenFlareOpsFields): OptionItem[] {
  validateAgentFields(fields);
  return [
    { key: 'agent_heartbeat_interval', value: fields.agent_heartbeat_interval },
    {
      key: 'agent_websocket_upgrade_enabled',
      value: String(fields.agent_websocket_upgrade_enabled),
    },
    { key: 'node_offline_threshold', value: fields.node_offline_threshold },
    { key: 'agent_update_repo', value: fields.agent_update_repo.trim() },
    { key: 'geoip_provider', value: fields.geoip_provider },
  ];
}

export function uptimeKumaOptionEntries(
  fields: OpenFlareOpsFields,
): OptionItem[] {
  validateUptimeKumaFields(fields);
  return [
    { key: 'uptime_kuma_enabled', value: String(fields.uptime_kuma_enabled) },
    { key: 'uptime_kuma_url', value: fields.uptime_kuma_url.trim() },
    { key: 'uptime_kuma_username', value: fields.uptime_kuma_username.trim() },
    { key: 'uptime_kuma_password', value: fields.uptime_kuma_password },
    {
      key: 'uptime_kuma_monitor_scope',
      value: fields.uptime_kuma_monitor_scope,
    },
    {
      key: 'uptime_kuma_selected_sites',
      value: fields.uptime_kuma_selected_sites,
    },
    {
      key: 'uptime_kuma_sync_interval',
      value: fields.uptime_kuma_sync_interval,
    },
    { key: 'uptime_kuma_interval', value: fields.uptime_kuma_interval },
    { key: 'uptime_kuma_retry', value: fields.uptime_kuma_retry },
    {
      key: 'uptime_kuma_retry_interval',
      value: fields.uptime_kuma_retry_interval,
    },
    { key: 'uptime_kuma_timeout', value: fields.uptime_kuma_timeout },
  ];
}

export function databaseAutoCleanupEntries(
  fields: OpenFlareOpsFields,
): OptionItem[] {
  validateDatabaseAutoCleanup(fields);
  return [
    {
      key: 'database_auto_cleanup_enabled',
      value: String(fields.database_auto_cleanup_enabled),
    },
    {
      key: 'database_auto_cleanup_retention_days',
      value: fields.database_auto_cleanup_retention_days,
    },
  ];
}

export function validatePagesFields(fields: OpenFlareOpsFields) {
  const packageSize = Number.parseInt(fields.pages_max_package_size_mb, 10);
  const historyCount = Number.parseInt(fields.pages_max_history_count, 10);
  if (Number.isNaN(packageSize) || packageSize < 1 || packageSize > 2048) {
    throw new Error('Pages 部署包大小上限必须为 1～2048 MiB。');
  }
  if (Number.isNaN(historyCount) || historyCount < 0) {
    throw new Error(
      'Pages 历史保留数必须为大于等于 0 的整数（0 表示不限制）。',
    );
  }
}

export function pagesOptionEntries(fields: OpenFlareOpsFields): OptionItem[] {
  validatePagesFields(fields);
  return [
    {
      key: 'pages_max_package_size_mb',
      value: String(Number.parseInt(fields.pages_max_package_size_mb, 10)),
    },
    {
      key: 'pages_max_history_count',
      value: String(Number.parseInt(fields.pages_max_history_count, 10)),
    },
  ];
}
