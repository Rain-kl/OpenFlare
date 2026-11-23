import type { TunnelInput } from '@/lib/services/openflare';

export type TunnelStatusTone = 'success' | 'warning' | 'danger' | 'info';

export function getTunnelStatusTone(status: string): TunnelStatusTone {
  switch (status) {
    case 'online':
    case 'healthy':
    case 'active':
      return 'success';
    case 'pending':
    case 'connecting':
    case 'degraded':
    case 'draining':
      return 'warning';
    case 'quarantined':
    case 'revoked':
    case 'offline':
    case 'stale':
    case 'unhealthy':
      return 'danger';
    default:
      return 'info';
  }
}

export function getErrorMessage(error: unknown, fallback: string) {
  return error instanceof Error ? error.message : fallback;
}

export function splitCommaOrLineList(raw: string): string[] {
  return raw
    .split(/[\r\n,]+/)
    .map((item) => item.trim())
    .filter(Boolean);
}

export function normalizeServerUrl(value: string) {
  return value.trim().replace(/\/+$/, '');
}

function quoteShellArg(value: string) {
  return "'" + value.replaceAll("'", "'\\''") + "'";
}

export function buildRelayInstallScriptCommand(
  serverUrl: string,
  bootstrapToken: string,
) {
  const tokenArg = bootstrapToken.startsWith('oftok1_')
    ? `--token ${quoteShellArg(bootstrapToken)}`
    : `--server-url ${quoteShellArg(normalizeServerUrl(serverUrl))} --token ${quoteShellArg(bootstrapToken)}`;
  return [
    'curl -fsSL https://raw.githubusercontent.com/Rain-kl/OpenFlare/main/scripts/install-openflare-relay.sh -o install-openflare-relay.sh && \\',
    `sudo bash install-openflare-relay.sh ${tokenArg}`,
  ].join('\n');
}

export function buildRelayDockerCommand(
  serverUrl: string,
  bootstrapToken: string,
) {
  return [
    'docker pull ghcr.io/rain-kl/openflare-relay:latest',
    'docker rm -f openflare-relay 2>/dev/null || true',
    'docker run -d --name openflare-relay --net host --restart unless-stopped \\',
    '  -v openflare-relay-data:/var/lib/openflare-relay \\',
    `  -e OPENFLARE_SERVER_URL=${quoteShellArg(normalizeServerUrl(serverUrl))} \\`,
    `  -e OPENFLARE_RELAY_TOKEN=${quoteShellArg(bootstrapToken)} \\`,
    '  ghcr.io/rain-kl/openflare-relay:latest',
  ].join('\n');
}

export function buildConnectorInstallScriptCommand(
  serverUrl: string,
  bootstrapToken: string,
) {
  const tokenArg = bootstrapToken.startsWith('oftok1_')
    ? `--token ${quoteShellArg(bootstrapToken)}`
    : `--server-url ${quoteShellArg(normalizeServerUrl(serverUrl))} --token ${quoteShellArg(bootstrapToken)}`;
  return [
    'curl -fsSL https://raw.githubusercontent.com/Rain-kl/OpenFlare/main/scripts/install-openflared.sh -o install-openflared.sh && \\',
    `sudo bash install-openflared.sh ${tokenArg}`,
  ].join('\n');
}

export function buildConnectorDockerCommand(
  serverUrl: string,
  bootstrapToken: string,
) {
  return [
    'docker pull ghcr.io/rain-kl/openflared:latest',
    'docker rm -f openflared 2>/dev/null || true',
    'docker run -d --name openflared --restart unless-stopped \\',
    '  -v openflared-data:/var/lib/openflared \\',
    `  -e OPENFLARE_SERVER_URL=${quoteShellArg(normalizeServerUrl(serverUrl))} \\`,
    `  -e OPENFLARE_TUNNEL_TOKEN=${quoteShellArg(bootstrapToken)} \\`,
    '  ghcr.io/rain-kl/openflared:latest',
  ].join('\n');
}

export function buildDefaultTunnelInput(): TunnelInput {
  return {
    name: '',
    description: '',
    ha_relay_count: 2,
    preferred_transport: 'auto',
    fallback_transports: ['noise', 'websocket', 'tcp'],
    allowed_target_cidrs: [],
    blocked_target_cidrs: ['169.254.169.254/32'],
    max_connectors: 4,
    enabled: true,
    auto_update_enabled: false,
  };
}
