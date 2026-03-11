import type { AuthUser } from '@/types/auth';

export interface OptionItem {
  key: string;
  value: string;
}

export interface UpdateSelfPayload {
  username: string;
  display_name: string;
  password: string;
}

export type SettingsProfile = AuthUser;
