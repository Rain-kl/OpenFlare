import { runtimeText } from '@/i18n/runtime';
import type { WAFIPGroup, WAFIPGroupPayload } from '@/lib/services/openflare';

export function getErrorMessage(error: unknown, fallback?: string) {
  return error instanceof Error
    ? error.message
    : (fallback ?? runtimeText('operation_failed_0c3b4cf7'));
}

export function listToText(items: string[] | undefined) {
  return (items ?? []).join('\n');
}

export function parseTextareaList(text: string) {
  return text
    .split(/\r?\n/)
    .map((item) => item.trim())
    .filter(Boolean);
}

export function parseAutomaticConfig(
  text: string,
  invalidMessage = runtimeText(
    'automatic_configuration_must_be_a_json_object_90ba17a4',
  ),
): Record<string, unknown> {
  const parsed = JSON.parse(text || '{}') as unknown;
  if (!parsed || Array.isArray(parsed) || typeof parsed !== 'object') {
    throw new Error(invalidMessage);
  }
  return parsed as Record<string, unknown>;
}

export const automaticPresetRules = [
  {
    get name() {
      return runtimeText('high_frequency_404_scan_from_one_ip_f7d27ffb');
    },
    labelKey: 'presets.scan404' as const,
    expr: 'request_count > 100 && StatusRatio(404) >= 0.8',
  },
  {
    get name() {
      return runtimeText('abnormal_direct_access_from_one_ip_732dbeee');
    },
    labelKey: 'presets.directAccess' as const,
    expr: 'ip_host_count > 50 && ip_host_ratio > 0.5',
  },
];

export type IPGroupViewEntry = {
  ip: string;
  capturedAt?: string;
  banRemaining?: string;
};

export function buildIPGroupPayloadFromGroup(
  group: WAFIPGroup,
  ipList: string[],
): WAFIPGroupPayload {
  return {
    name: group.name,
    type: group.type,
    enabled: group.enabled,
    ip_list: ipList,
    auto_config: group.auto_config ?? {},
    subscription_url: group.subscription_url ?? '',
    subscription_format: group.subscription_format ?? 'text',
    subscription_mapping_rule: group.subscription_mapping_rule ?? '',
    sync_interval_minutes: group.sync_interval_minutes || 1440,
  };
}

type BanRemainingTranslator = (
  key: 'ban.permanent' | 'ban.expired' | 'ban.minutesLater' | 'ban.hoursLater',
  values?: { count: number },
) => string;

export function formatIPGroupBanRemaining(
  capturedAt: string,
  ttlSeconds: number,
  now?: Date,
  t?: BanRemainingTranslator,
): string {
  const reference = now ?? new Date();
  if (ttlSeconds <= 0) {
    return t ? t('ban.permanent') : runtimeText('permanent_3e71ccc8');
  }
  const capturedDate = new Date(capturedAt);
  if (Number.isNaN(capturedDate.getTime())) {
    return '—';
  }
  const expireDate = new Date(capturedDate.getTime() + ttlSeconds * 1000);
  if (expireDate.getTime() <= reference.getTime()) {
    return t ? t('ban.expired') : runtimeText('expired_2fe0e333');
  }
  const diffMins = Math.round(
    (expireDate.getTime() - reference.getTime()) / (60 * 1000),
  );
  if (diffMins < 60) {
    return t
      ? t('ban.minutesLater', { count: diffMins })
      : runtimeText('in_value0_minutes_fd2a5b53', { value0: diffMins });
  }
  const hours = Math.round(diffMins / 60);
  return t
    ? t('ban.hoursLater', { count: hours })
    : runtimeText('in_value0_hours_5ca37144', { value0: hours });
}

export function getIPGroupViewEntries(
  group: WAFIPGroup,
  t?: BanRemainingTranslator,
): IPGroupViewEntry[] {
  if (group.type === 'automatic' && group.ext_ips && group.ext_ips.length > 0) {
    const ttl =
      typeof group.auto_config?.ttl === 'number' ? group.auto_config.ttl : -1;
    return group.ext_ips.map((item) => ({
      ip: item.ip,
      capturedAt: item.captured_at,
      banRemaining: formatIPGroupBanRemaining(
        item.captured_at,
        ttl,
        undefined,
        t,
      ),
    }));
  }
  return (group.ip_list ?? []).map((ip) => ({ ip }));
}
