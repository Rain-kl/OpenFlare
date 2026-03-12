import type { ReleaseChannel } from '@/features/update/types';

export interface NodeItem {
  id: number;
  node_id: string;
  name: string;
  ip: string;
  agent_token: string;
  auto_update_enabled: boolean;
  update_requested: boolean;
  update_channel: ReleaseChannel;
  update_tag: string;
  restart_openresty_requested: boolean;
  agent_version: string;
  nginx_version: string;
  openresty_status: 'healthy' | 'unhealthy' | 'unknown';
  openresty_message: string;
  status: 'online' | 'offline' | 'pending';
  current_version: string;
  last_seen_at: string;
  last_error: string;
  latest_apply_result: 'success' | 'failed' | '';
  latest_apply_message: string;
  latest_apply_checksum: string;
  latest_main_config_checksum: string;
  latest_route_config_checksum: string;
  latest_support_file_count: number;
  latest_apply_at?: string | null;
  created_at: string;
  updated_at: string;
}

export interface NodeBootstrapToken {
  discovery_token: string;
}

export interface NodeMutationPayload {
  name: string;
  auto_update_enabled: boolean;
}

export interface NodeAgentReleaseInfo {
  tag_name: string;
  body: string;
  html_url: string;
  published_at: string;
  current_version: string;
  has_update: boolean;
  channel: ReleaseChannel;
  prerelease: boolean;
  update_requested: boolean;
  requested_channel: ReleaseChannel;
  requested_tag: string;
}

export interface NodeAgentUpdatePayload {
  channel?: ReleaseChannel;
  tag_name?: string;
}
