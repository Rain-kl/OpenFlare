import { expect, it } from 'vitest';

import { buildEdgeDockerInstallCommand } from '@/app/(main)/nodes/components/node-utils';

it('uses the Edge image channel and includes the ports, volume and node credentials', () => {
  const server = 'https://control.example.test';
  const command = buildEdgeDockerInstallCommand(
    server,
    "fixture'token",
    'canary',
  );

  expect(command).toContain('ghcr.io/rain-kl/openflare-edge:canary');
  expect(command).toContain('--cap-add=NET_RAW');
  expect(command).toContain('-p 80:8080 -p 443:8443');
  expect(command).toContain('openflare-edge-data:/var/lib/openflare-edge');
  expect(command).toContain(`OPENFLARE_EDGE_SERVER_URL='${server}'`);
  expect(command).toContain('OPENFLARE_EDGE_TOKEN_TYPE=node');
  expect(command).toContain("OPENFLARE_EDGE_TOKEN='fixture'\\''token'");
  expect(
    buildEdgeDockerInstallCommand(server, 'fixture-token', 'v3.6.1'),
  ).toContain('ghcr.io/rain-kl/openflare-edge:latest');
});
