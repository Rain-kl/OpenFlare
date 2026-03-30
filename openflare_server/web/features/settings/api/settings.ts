import { apiRequest } from '@/lib/api/client';

import type {
  BootstrapTokenPayload,
  DatabaseCleanupPayload,
  DatabaseCleanupResult,
  GeoIPLookupResult,
  OptionBatchPayload,
  OptionItem,
  SettingsProfile,
  UpdateSelfPayload,
} from '@/features/settings/types';

export function getOptions() {
  return apiRequest<OptionItem[]>('/option/');
}

export function updateOption(key: string, value: string) {
  return apiRequest<void>('/option/update', {
    method: 'POST',
    body: JSON.stringify({ key, value }),
  });
}

export function updateOptions(options: OptionBatchPayload['options']) {
  return apiRequest<void>('/option/update-batch', {
    method: 'POST',
    body: JSON.stringify({ options }),
  });
}

export function lookupGeoIP(provider: string, ip: string) {
  return apiRequest<GeoIPLookupResult>('/option/geoip/lookup', {
    method: 'POST',
    body: JSON.stringify({ provider, ip }),
  });
}

export function cleanupDatabaseObservability(payload: DatabaseCleanupPayload) {
  return apiRequest<DatabaseCleanupResult>('/option/database/cleanup', {
    method: 'POST',
    body: JSON.stringify(payload),
  });
}

export function getBootstrapToken() {
  return apiRequest<BootstrapTokenPayload>('/nodes/bootstrap-token');
}

export function rotateBootstrapToken() {
  return apiRequest<BootstrapTokenPayload>('/nodes/bootstrap-token/rotate', {
    method: 'POST',
  });
}

export function getSettingsProfile() {
  return apiRequest<SettingsProfile>('/user/self');
}

export function updateSelf(payload: UpdateSelfPayload) {
  return apiRequest<void>('/user/self/update', {
    method: 'POST',
    body: JSON.stringify(payload),
  });
}

export function generateAccessToken() {
  return apiRequest<string>('/user/token');
}

export function bindWeChat(code: string) {
  return apiRequest<void>(
    `/oauth/wechat/bind?code=${encodeURIComponent(code)}`,
  );
}

export function bindEmail(email: string, code: string) {
  const searchParams = new URLSearchParams({ email, code });
  return apiRequest<void>(`/oauth/email/bind?${searchParams.toString()}`);
}

export function getAboutContent() {
  return apiRequest<string>('/about');
}
