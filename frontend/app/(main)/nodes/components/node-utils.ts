import type {
  ApplyResult,
  NodeItem,
  NodeStatus,
  OpenrestyStatus,
} from '@/lib/services/openflare';

export const WS_CONNECTED_LAST_SEEN = '__OPENFLARE_WS_CONNECTED__';
export const FLARED_WS_CONNECTED_LAST_SEEN =
  '__OPENFLARE_FLARED_WS_CONNECTED__';

export type StatusTone = 'success' | 'warning' | 'danger' | 'info';

export function isWSConnectedLastSeen(value: string | null | undefined) {
  return (
    value === WS_CONNECTED_LAST_SEEN || value === FLARED_WS_CONNECTED_LAST_SEEN
  );
}

export function isMeaningfulTime(
  value: string | null | undefined,
): value is string {
  return (
    Boolean(value) &&
    !isWSConnectedLastSeen(value) &&
    !String(value).startsWith('0001-01-01')
  );
}

export function formatRelativeTime(value: string) {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }

  const diffMs = Date.now() - date.getTime();
  const diffMinutes = Math.floor(diffMs / 60_000);
  if (diffMinutes < 1) return '刚刚';
  if (diffMinutes < 60) return `${diffMinutes} 分钟前`;

  const diffHours = Math.floor(diffMinutes / 60);
  if (diffHours < 24) return `${diffHours} 小时前`;

  const diffDays = Math.floor(diffHours / 24);
  if (diffDays < 30) return `${diffDays} 天前`;

  return `${Math.floor(diffDays / 30)} 个月前`;
}

export function getNodeStatusTone(status: NodeStatus): StatusTone {
  if (status === 'online') return 'success';
  if (status === 'pending') return 'warning';
  return 'danger';
}

export function getNodeStatusLabel(status: NodeStatus) {
  if (status === 'online') return '在线';
  if (status === 'pending') return '待接入';
  return '离线';
}

export function getApplyTone(result: ApplyResult): StatusTone {
  if (result === 'success') return 'success';
  if (result === 'warning') return 'warning';
  if (result === 'failed') return 'danger';
  return 'warning';
}

export function getApplyLabel(result: ApplyResult) {
  if (result === 'success') return '成功';
  if (result === 'warning') return '警告';
  if (result === 'failed') return '失败';
  return '暂无';
}

export function getOpenrestyStatusTone(status: OpenrestyStatus): StatusTone {
  if (status === 'healthy') return 'success';
  if (status === 'unhealthy') return 'danger';
  return 'warning';
}

export function getOpenrestyStatusLabel(status: OpenrestyStatus) {
  if (status === 'healthy') return '健康';
  if (status === 'unhealthy') return '异常';
  return '未知';
}

export function getRelayStatusTone(
  status: string | null | undefined,
): StatusTone {
  if (status === 'healthy') return 'success';
  if (status === 'unhealthy') return 'danger';
  return 'warning';
}

export function getRelayStatusLabel(status: string | null | undefined) {
  if (status === 'healthy') return '健康';
  if (status === 'unhealthy') return '异常';
  return '未知';
}

export function getNodeTypeLabel(nodeType: NodeItem['node_type']) {
  if (nodeType === 'tunnel_relay') return 'Relay';
  if (nodeType === 'tunnel_client') return 'Tunnel';
  return 'Edge';
}

export function getErrorMessage(error: unknown) {
  return error instanceof Error ? error.message : '请求失败，请稍后重试。';
}

export function getServerUrl(value: string) {
  return value.trim().replace(/\/+$/, '');
}

const relayInstallerScriptUrl =
  'https://raw.githubusercontent.com/Rain-kl/OpenFlare/main/scripts/install-relay.sh';

const flaredInstallerScriptUrl =
  'https://raw.githubusercontent.com/Rain-kl/OpenFlare/main/scripts/install-flared.sh';

export function getImageTag(version?: string): string {
  if (!version) {
    return 'latest';
  }
  const v = version.toLowerCase();
  if (
    v === 'dev' ||
    v.includes('alpha') ||
    v.includes('beta') ||
    v.includes('rc')
  ) {
    return 'beta';
  }
  if (v.startsWith('v')) {
    return version;
  }
  return 'latest';
}

export function buildEdgeDockerInstallCommand(
  serverUrl: string,
  agentToken: string,
  version?: string,
) {
  const tag = getImageTag(version);
  const image = `ghcr.io/rain-kl/openflare-agent:${tag}`;

  return [
    `docker pull ${image}`,
    `docker rm -f openflare-agent 2>/dev/null || true`,
    `docker run -d --name openflare-agent --restart unless-stopped \\`,
    `  -p 80:80 -p 443:443/tcp -p 443:443/udp \\`,
    `  -v openflare-agent-pages:/data/var/lib/openflare/pages \\`,
    `  -e OPENFLARE_SERVER_URL=${serverUrl} \\`,
    `  -e OPENFLARE_AGENT_TOKEN=${agentToken} \\`,
    `  ${image}`,
  ].join('\n');
}

export function buildRelayInstallCommand(
  serverUrl: string,
  discoveryToken: string,
) {
  return [
    `curl -fsSL ${relayInstallerScriptUrl} | bash -s -- \\`,
    `  --server-url ${serverUrl} \\`,
    `  --discovery-token ${discoveryToken}`,
  ].join('\n');
}

export function buildRelayDockerInstallCommand(
  serverUrl: string,
  discoveryToken: string,
  version?: string,
) {
  const tag = getImageTag(version);
  const image = `ghcr.io/rain-kl/openflare-relay:${tag}`;

  return [
    `docker pull ${image}`,
    `docker rm -f openflare-relay 2>/dev/null || true`,
    `docker run -d --name openflare-relay --net host --restart unless-stopped \\`,
    `  -e OPENFLARE_SERVER_URL=${serverUrl} \\`,
    `  -e OPENFLARE_DISCOVERY_TOKEN=${discoveryToken} \\`,
    `  ${image}`,
  ].join('\n');
}

export function buildTunnelInstallCommand(
  serverUrl: string,
  tunnelToken: string,
) {
  return [
    `curl -fsSL ${flaredInstallerScriptUrl} | bash -s -- \\`,
    `  --server-url ${serverUrl} \\`,
    `  --tunnel-token ${tunnelToken}`,
  ].join('\n');
}

export function buildTunnelDockerInstallCommand(
  serverUrl: string,
  tunnelToken: string,
  version?: string,
) {
  const tag = getImageTag(version);
  const image = `ghcr.io/rain-kl/openflared:${tag}`;

  return [
    `docker pull ${image}`,
    `docker rm -f openflared 2>/dev/null || true`,
    `docker run -d --name openflared --restart unless-stopped \\`,
    `  -e OPENFLARE_SERVER_URL=${serverUrl} \\`,
    `  -e OPENFLARE_TUNNEL_TOKEN=${tunnelToken} \\`,
    `  ${image}`,
  ].join('\n');
}

export function formatBytes(bytes?: number | null, decimals = 1) {
  if (bytes === undefined || bytes === null || !Number.isFinite(bytes)) {
    return '—';
  }
  if (bytes <= 0) {
    return '0 B';
  }

  const units = ['B', 'KB', 'MB', 'GB', 'TB'];
  const index = Math.min(
    Math.floor(Math.log(bytes) / Math.log(1024)),
    units.length - 1,
  );
  const value = bytes / 1024 ** index;
  return `${value.toFixed(decimals)} ${units[index]}`;
}

export function formatPercent(value?: number | null) {
  if (value === undefined || value === null || !Number.isFinite(value)) {
    return '—';
  }
  return `${value.toFixed(1)}%`;
}

export function formatMetricCount(value?: number | null) {
  if (value === undefined || value === null || !Number.isFinite(value)) {
    return '—';
  }
  return value.toLocaleString('zh-CN');
}

export function formatBytesPerSecond(value?: number | null, windowSeconds = 1) {
  if (value === undefined || value === null || !Number.isFinite(value)) {
    return '—';
  }
  if (windowSeconds <= 0) {
    return '—';
  }
  return `${formatBytes(value / windowSeconds)}/s`;
}

export function formatUsageRatio(used?: number | null, total?: number | null) {
  if (!used || !total || total <= 0) {
    return null;
  }
  return Math.max(0, Math.min(100, (used / total) * 100));
}

export function formatUptime(seconds?: number | null) {
  if (!seconds || seconds <= 0) {
    return '—';
  }

  const days = Math.floor(seconds / 86400);
  const hours = Math.floor((seconds % 86400) / 3600);
  const minutes = Math.floor((seconds % 3600) / 60);

  if (days > 0) {
    return `${days} 天 ${hours} 小时`;
  }
  if (hours > 0) {
    return `${hours} 小时 ${minutes} 分钟`;
  }
  return `${minutes} 分钟`;
}

export function getHealthEventTone(event: {
  status: string;
  severity: string;
}): StatusTone {
  if (event.status === 'resolved') {
    return 'success';
  }
  if (event.severity === 'critical') {
    return 'danger';
  }
  if (event.severity === 'warning') {
    return 'warning';
  }
  return 'info';
}

export function getHealthEventLabel(event: { event_type: string }) {
  return event.event_type.replaceAll('_', ' ');
}

export function getFlaredStatusLabel(node: NodeItem) {
  if (isWSConnectedLastSeen(node.last_seen_at)) {
    return 'WS 已连接';
  }
  if (node.status === 'online') {
    return '运行中';
  }
  if (node.status === 'pending') {
    return '待接入';
  }
  return '离线';
}

export function getFlaredStatusTone(node: NodeItem): StatusTone {
  if (isWSConnectedLastSeen(node.last_seen_at) || node.status === 'online') {
    return 'success';
  }
  if (node.status === 'pending') {
    return 'warning';
  }
  return 'danger';
}
