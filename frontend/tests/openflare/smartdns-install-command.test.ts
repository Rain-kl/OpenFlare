import { expect, it } from 'vitest';

import {
  buildSmartDNSDockerCommand,
  buildSmartDNSInstallCommand,
} from '@/app/(main)/dns/smartdns/install-command';

it('builds Linux install commands for stable and canary channels with shell escaping', () => {
  const stable = buildSmartDNSInstallCommand({
    controllerUrl: 'https://panel.example.test',
    token: "disc'token",
    tokenType: 'discovery',
    channel: 'stable',
  });
  expect(stable).toContain('install-openflare-dns.sh');
  expect(stable).not.toContain('\n  canary \\');
  expect(stable).toContain("--controller-url 'https://panel.example.test'");
  expect(stable).toContain('--token-type discovery');
  expect(stable).toContain("--token 'disc'\\''token'");
  expect(stable).toContain("--listen '0.0.0.0:53'");

  const canary = buildSmartDNSInstallCommand({
    controllerUrl: 'https://panel.example.test',
    token: 'inst-secret',
    tokenType: 'instance',
    instanceId: 'dns-hk01',
    publicAddr: '203.0.113.53',
    listenAddr: '0.0.0.0:5353',
    channel: 'canary',
  });
  expect(canary).toContain('  canary \\');
  expect(canary).toContain('--token-type instance');
  expect(canary).toContain("--instance-id 'dns-hk01'");
  expect(canary).toContain("--public-addr '203.0.113.53'");
  expect(canary).toContain("--listen '0.0.0.0:5353'");
});

it('builds Docker commands with stable or canary image tags and required ports', () => {
  const stable = buildSmartDNSDockerCommand({
    controllerUrl: 'https://panel.example.test',
    token: 'disc-token',
    tokenType: 'discovery',
    channel: 'stable',
  });
  expect(stable).toContain('ghcr.io/rain-kl/openflare-dns:latest');
  expect(stable).toContain('-p 53:5353/udp -p 53:5353/tcp');
  expect(stable).toContain('openflare-dns-data:/var/lib/openflare-dns');
  expect(stable).toContain(
    "OPENFLARE_DNS_CONTROLLER_URL='https://panel.example.test'",
  );
  expect(stable).toContain('OPENFLARE_DNS_TOKEN_TYPE=discovery');
  expect(stable).toContain("OPENFLARE_DNS_TOKEN='disc-token'");

  const canary = buildSmartDNSDockerCommand({
    controllerUrl: 'https://panel.example.test',
    token: "acc'tok",
    tokenType: 'instance',
    instanceId: 'dns-edge',
    channel: 'canary',
  });
  expect(canary).toContain('ghcr.io/rain-kl/openflare-dns:canary');
  expect(canary).toContain('OPENFLARE_DNS_TOKEN_TYPE=instance');
  expect(canary).toContain("OPENFLARE_DNS_TOKEN='acc'\\''tok'");
  expect(canary).toContain("OPENFLARE_DNS_INSTANCE_ID='dns-edge'");
});
