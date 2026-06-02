import type { NodeItem } from '@/features/nodes/types';

export const WS_CONNECTED_LAST_SEEN = '__OPENFLARE_WS_CONNECTED__';
export const FLARED_WS_CONNECTED_LAST_SEEN = '__OPENFLARE_FLARED_WS_CONNECTED__';

export function isWSConnectedLastSeen(value: string | null | undefined) {
  return value === WS_CONNECTED_LAST_SEEN || value === FLARED_WS_CONNECTED_LAST_SEEN;
}

export function isMeaningfulTime(value: string | null | undefined) {
  return (
    Boolean(value) &&
    !isWSConnectedLastSeen(value) &&
    !String(value).startsWith('0001-01-01')
  );
}

export function getNodeStatusVariant(status: NodeItem['status']) {
  if (status === 'online') {
    return 'success';
  }

  if (status === 'pending') {
    return 'warning';
  }

  return 'danger';
}

export function getNodeStatusLabel(status: NodeItem['status']) {
  if (status === 'online') {
    return '在线';
  }

  if (status === 'pending') {
    return '待接入';
  }

  return '离线';
}

export function getApplyVariant(result: NodeItem['latest_apply_result']) {
  if (result === 'success') {
    return 'success';
  }

  if (result === 'warning') {
    return 'warning';
  }

  if (result === 'failed') {
    return 'danger';
  }

  return 'warning';
}

export function getApplyLabel(result: NodeItem['latest_apply_result']) {
  if (result === 'success') {
    return '成功';
  }

  if (result === 'warning') {
    return '警告';
  }

  if (result === 'failed') {
    return '失败';
  }

  return '暂无';
}

export function getUpdateMode(node: NodeItem) {
  if (node.update_requested) {
    if (node.update_channel === 'preview') {
      return { label: '等待预览更新', variant: 'warning' as const };
    }

    return { label: '等待更新', variant: 'warning' as const };
  }

  if (node.auto_update_enabled) {
    return { label: '自动', variant: 'success' as const };
  }

  return { label: '手动', variant: 'info' as const };
}

export function getOpenrestyStatusVariant(
  status: NodeItem['openresty_status'],
) {
  if (status === 'healthy') {
    return 'success';
  }

  if (status === 'unhealthy') {
    return 'danger';
  }

  return 'warning';
}

export function getOpenrestyStatusLabel(status: NodeItem['openresty_status']) {
  if (status === 'healthy') {
    return '健康';
  }

  if (status === 'unhealthy') {
    return '异常';
  }

  return '未知';
}

export function isNodeAbnormal(node: NodeItem, activeVersion?: string) {
  if (node.status === 'offline') {
    return true;
  }

  if (activeVersion && node.current_version !== activeVersion) {
    return true;
  }

  if (node.node_type === 'tunnel_relay') {
    return node.relay_status === 'unhealthy';
  }

  if (node.node_type === 'edge_node') {
    return node.openresty_status === 'unhealthy';
  }

  return false;
}

function parseVersionParts(version: string) {
  const normalized = version.trim().replace(/^v/i, '');
  if (!normalized || normalized.toLowerCase() === 'unknown') {
    return null;
  }

  return normalized.split('.').map((segment) => {
    const matched = segment.trim().match(/^\d+/);
    return matched ? Number.parseInt(matched[0], 10) : 0;
  });
}

function isOlderVersion(current: string, target: string) {
  const currentParts = parseVersionParts(current);
  const targetParts = parseVersionParts(target);
  if (!currentParts || !targetParts) {
    return false;
  }

  const maxLength = Math.max(currentParts.length, targetParts.length);
  for (let index = 0; index < maxLength; index += 1) {
    const currentPart = currentParts[index] ?? 0;
    const targetPart = targetParts[index] ?? 0;
    if (currentPart < targetPart) {
      return true;
    }
    if (currentPart > targetPart) {
      return false;
    }
  }

  return false;
}

export function shouldShowManualUpdate(
  agentVersion: string,
  serverVersion: string,
) {
  const normalizedServerVersion = serverVersion.trim();
  const normalizedAgentVersion = agentVersion.trim();

  if (
    !normalizedServerVersion ||
    normalizedServerVersion.toLowerCase() === 'dev' ||
    !normalizedAgentVersion ||
    normalizedAgentVersion.toLowerCase() === 'unknown'
  ) {
    return false;
  }

  return isOlderVersion(normalizedAgentVersion, normalizedServerVersion);
}

export function getServerUrl(value: string) {
  return value.trim().replace(/\/+$/, '');
}

const installerScriptUrl =
  'https://raw.githubusercontent.com/Rain-kl/OpenFlare/main/scripts/install-agent.sh';

export function buildNodeInstallCommand(serverUrl: string, agentToken: string) {
  return [
    `curl -fsSL ${installerScriptUrl} | bash -s -- \\`,
    `  --server-url ${serverUrl} \\`,
    `  --agent-token ${agentToken}`,
  ].join('\n');
}

export function buildNodeDockerInstallCommand(
  serverUrl: string,
  agentToken: string,
) {
  const image = 'ghcr.io/rain-kl/openflare-agent:latest';

  return [
    `docker pull ${image}`,
    `docker rm -f openflare-agent 2>/dev/null || true`,
    `docker run -d --name openflare-agent --restart unless-stopped \\`,
    `  -p 80:80 -p 443:443/tcp -p 443:443/udp \\`,
    `  -e OPENFLARE_SERVER_URL=${serverUrl} \\`,
    `  -e OPENFLARE_AGENT_TOKEN=${agentToken} \\`,
    `  ${image}`,
  ].join('\n');
}

export function getRelayStatusVariant(status: string | null | undefined) {
  if (status === 'healthy') {
    return 'success';
  }
  if (status === 'unhealthy') {
    return 'danger';
  }
  return 'warning';
}

export function getRelayStatusLabel(status: string | null | undefined) {
  if (status === 'healthy') {
    return '健康';
  }
  if (status === 'unhealthy') {
    return '异常';
  }
  return '未知';
}

const relayInstallerScriptUrl =
  'https://raw.githubusercontent.com/Rain-kl/OpenFlare/main/scripts/install-relay.sh';

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
) {
  const image = 'ghcr.io/rain-kl/openflare-relay:latest';

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
  const flaredInstallerScriptUrl =
    'https://raw.githubusercontent.com/Rain-kl/OpenFlare/main/scripts/install-flared.sh';
  return [
    `curl -fsSL ${flaredInstallerScriptUrl} | bash -s -- \\`,
    `  --server-url ${serverUrl} \\`,
    `  --tunnel-token ${tunnelToken}`,
  ].join('\n');
}

export function buildTunnelDockerInstallCommand(
  serverUrl: string,
  tunnelToken: string,
) {
  const image = 'ghcr.io/rain-kl/openflared:latest';

  return [
    `docker pull ${image}`,
    `docker rm -f openflared 2>/dev/null || true`,
    `docker run -d --name openflared --restart unless-stopped \\`,
    `  -e OPENFLARE_SERVER_URL=${serverUrl} \\`,
    `  -e OPENFLARE_TUNNEL_TOKEN=${tunnelToken} \\`,
    `  ${image}`,
  ].join('\n');
}
