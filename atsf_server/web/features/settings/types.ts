import type { AuthUser } from '@/types/auth';

export interface OptionItem {
  key: string;
  value: string;
}

export interface LatestReleaseInfo {
  tag_name: string;
  body: string;
  html_url: string;
  published_at: string;
}

export interface BootstrapTokenPayload {
  discovery_token: string;
}

export interface UpdateSelfPayload {
  username: string;
  display_name: string;
  password: string;
}

export interface SettingsProfile extends AuthUser {}
