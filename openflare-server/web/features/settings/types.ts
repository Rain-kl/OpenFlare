import type { AuthUser } from '@/types/auth';

export interface OptionItem {
  key: string;
  value: string;
}

export interface OptionBatchPayload {
  options: OptionItem[];
}

export type AuthSourceType = 'github' | 'oidc';

export interface AuthSource {
  id: number;
  name: string;
  type: AuthSourceType;
  display_name: string;
  is_active: boolean;
  client_id: string;
  client_secret?: string;
  client_secret_configured?: boolean;
  openid_discovery_url: string;
  scopes: string;
  icon_url: string;
}

export interface ExternalAccountBinding {
  id: number;
  auth_source_id: number;
  auth_source_name: string;
  auth_source_type: AuthSourceType;
  auth_source_label: string;
  external_username: string;
  email: string;
  created_at: string;
}

export type AuthSourcePayload = Omit<
  AuthSource,
  'id' | 'client_secret_configured'
> & {
  id?: number;
  client_secret: string;
};

export interface BootstrapTokenPayload {
  discovery_token: string;
}

export interface GeoIPLookupResult {
  provider: string;
  ip: string;
  iso_code: string;
  name: string;
  latitude?: number | null;
  longitude?: number | null;
}

export type DatabaseCleanupTarget =
  | 'node_access_logs'
  | 'node_metric_snapshots'
  | 'node_request_reports';

export interface DatabaseCleanupPayload {
  target: DatabaseCleanupTarget;
  retention_days?: number;
}

export interface DatabaseCleanupResult {
  target: DatabaseCleanupTarget;
  target_label: string;
  deleted_count: number;
  delete_all: boolean;
  retention_days?: number;
  cutoff?: string;
}

export interface UpdateSelfPayload {
  username: string;
  display_name: string;
  password: string;
}

export type SettingsProfile = AuthUser;
