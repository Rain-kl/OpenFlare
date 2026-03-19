import type { AuthUser } from '@/types/auth';

export interface OptionItem {
  key: string;
  value: string;
}

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
