import { describe, expect, it } from 'vitest';

import type { NodeItem } from '@/features/nodes/types';
import { isNodeAbnormal } from '@/features/nodes/utils';

function buildNode(overrides: Partial<NodeItem> = {}): NodeItem {
  return {
    id: 1,
    node_id: 'node-1',
    node_type: 'edge_node',
    name: 'node-1',
    ip: '127.0.0.1',
    ip_manual_override: false,
    relay_bind_port: 7000,
    relay_vhost_http_port: 8080,
    relay_client_access_addr: '',
    relay_agent_access_addr: '',
    relay_client_proxy_url: '',
    relay_auth_token: '',
    relay_status: 'healthy',
    relay_frps_connections: 0,
    relay_frps_proxy_count: 0,
    geo_name: '',
    geo_latitude: null,
    geo_longitude: null,
    geo_manual_override: false,
    access_token: '',
    auto_update_enabled: false,
    update_requested: false,
    update_channel: 'stable',
    update_tag: '',
    restart_openresty_requested: false,
    version: '1.0.0',
    ext_version: '',
    openresty_status: 'healthy',
    openresty_message: '',
    status: 'online',
    current_version: '20260601-001',
    last_seen_at: '',
    last_error: '',
    latest_apply_result: '',
    latest_apply_message: '',
    latest_apply_checksum: '',
    latest_main_config_checksum: '',
    latest_route_config_checksum: '',
    latest_support_file_count: 0,
    latest_apply_at: null,
    created_at: '',
    updated_at: '',
    ...overrides,
  };
}

describe('isNodeAbnormal', () => {
  it('treats offline nodes as abnormal', () => {
    expect(
      isNodeAbnormal(buildNode({ status: 'offline' }), '20260601-001'),
    ).toBe(true);
  });

  it('treats unhealthy edge nodes as abnormal', () => {
    expect(
      isNodeAbnormal(
        buildNode({ node_type: 'edge_node', openresty_status: 'unhealthy' }),
        '20260601-001',
      ),
    ).toBe(true);
  });

  it('treats unhealthy relay nodes as abnormal', () => {
    expect(
      isNodeAbnormal(
        buildNode({ node_type: 'tunnel_relay', relay_status: 'unhealthy' }),
        '20260601-001',
      ),
    ).toBe(true);
  });

  it('treats nodes behind the active version as abnormal', () => {
    expect(
      isNodeAbnormal(
        buildNode({
          node_type: 'tunnel_client',
          current_version: '20260531-001',
        }),
        '20260601-001',
      ),
    ).toBe(true);
  });

  it('ignores version lag when there is no active version', () => {
    expect(
      isNodeAbnormal(
        buildNode({
          node_type: 'tunnel_client',
          current_version: '20260531-001',
        }),
      ),
    ).toBe(false);
  });

  it('keeps healthy synced nodes out of the abnormal filter', () => {
    expect(isNodeAbnormal(buildNode(), '20260601-001')).toBe(false);
  });
});
