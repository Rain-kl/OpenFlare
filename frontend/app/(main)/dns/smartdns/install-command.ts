export type SmartDNSReleaseChannel = 'stable' | 'canary';

export interface SmartDNSCommandOptions {
  controllerUrl: string;
  token: string;
  tokenType: 'discovery' | 'instance';
  instanceId?: string;
  listenAddr?: string;
  publicAddr?: string;
  channel?: SmartDNSReleaseChannel;
}

function quoteShellArgument(value: string) {
  return "'" + value.replaceAll("'", "'\\''") + "'";
}

export function buildSmartDNSInstallCommand(options: SmartDNSCommandOptions) {
  const channel: SmartDNSReleaseChannel =
    options.channel === 'canary' ? 'canary' : 'stable';
  const listen = options.listenAddr?.trim() || '0.0.0.0:53';
  const scriptArgs = [
    ...(channel === 'canary' ? ['  canary \\'] : []),
    `  --controller-url ${quoteShellArgument(options.controllerUrl)} \\`,
    `  --token-type ${options.tokenType} \\`,
    `  --token ${quoteShellArgument(options.token)} \\`,
  ];
  if (options.instanceId?.trim()) {
    scriptArgs.push(
      `  --instance-id ${quoteShellArgument(options.instanceId.trim())} \\`,
    );
  }
  if (options.publicAddr?.trim()) {
    scriptArgs.push(
      `  --public-addr ${quoteShellArgument(options.publicAddr.trim())} \\`,
    );
  }
  scriptArgs.push(`  --listen ${quoteShellArgument(listen)}`);

  return [
    'curl -fsSL https://raw.githubusercontent.com/Rain-kl/OpenFlare/main/scripts/install-openflare-dns.sh -o install-openflare-dns.sh && \\',
    'sudo bash install-openflare-dns.sh \\',
    ...scriptArgs,
  ].join('\n');
}

export function buildSmartDNSDockerCommand(options: SmartDNSCommandOptions) {
  const tag = options.channel === 'canary' ? 'canary' : 'latest';
  const image = `ghcr.io/rain-kl/openflare-dns:${tag}`;
  const lines = [
    `docker pull ${image}`,
    'docker rm -f openflare-dns 2>/dev/null || true',
    'docker run -d --name openflare-dns --restart unless-stopped \\',
    '  -p 53:5353/udp -p 53:5353/tcp \\',
    '  -v openflare-dns-data:/var/lib/openflare-dns \\',
    `  -e OPENFLARE_DNS_CONTROLLER_URL=${quoteShellArgument(options.controllerUrl)} \\`,
    `  -e OPENFLARE_DNS_TOKEN_TYPE=${options.tokenType} \\`,
    `  -e OPENFLARE_DNS_TOKEN=${quoteShellArgument(options.token)} \\`,
  ];
  if (options.instanceId?.trim()) {
    lines.push(
      `  -e OPENFLARE_DNS_INSTANCE_ID=${quoteShellArgument(options.instanceId.trim())} \\`,
    );
  }
  if (options.publicAddr?.trim()) {
    lines.push(
      `  -e OPENFLARE_DNS_PUBLIC_ADDR=${quoteShellArgument(options.publicAddr.trim())} \\`,
    );
  }
  lines.push(`  ${image}`);
  return lines.join('\n');
}
