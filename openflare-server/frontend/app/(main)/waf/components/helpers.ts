import type {
  ProxyRoutePoWConfig,
  WAFIPGroup,
  WAFIPGroupPayload,
  WAFRuleGroup,
  WAFRuleGroupPayload,
} from '@/lib/services/openflare';

export type WAFTab = 'basic' | 'lists' | 'pow' | 'block';
export type RuleListType = 'whitelist' | 'blacklist';
export type RuleDimension = 'ip' | 'ip_group' | 'country';
export type ListFieldKey =
  | 'ip_whitelist'
  | 'ip_blacklist'
  | 'ip_whitelist_group_ids'
  | 'ip_blacklist_group_ids'
  | 'country_whitelist'
  | 'country_blacklist';

export type CountryOption = {
  code: string;
  zhName: string;
  label: string;
  searchText: string;
};

export type RuleModalState = {
  open: boolean;
  listType: RuleListType;
  dimension: RuleDimension;
  ipValue: string;
  ipGroupIDs: number[];
  countryValues: string[];
};

export type RuleListRenderable = Pick<
  WAFRuleGroupPayload,
  | 'ip_whitelist'
  | 'ip_blacklist'
  | 'ip_whitelist_group_ids'
  | 'ip_blacklist_group_ids'
  | 'country_whitelist'
  | 'country_blacklist'
  | 'region_whitelist'
  | 'region_blacklist'
>;

export const defaultPowConfig: ProxyRoutePoWConfig = {
  difficulty: 4,
  algorithm: 'fast',
  session_ttl: 600,
  challenge_ttl: 300,
  whitelist: {
    ips: [],
    ip_cidrs: [],
    paths: [],
    path_regexes: [],
    user_agents: [],
  },
  blacklist: {
    ips: [],
    ip_cidrs: [],
    paths: [],
    path_regexes: [],
    user_agents: [],
  },
};

export const emptyRuleGroupDraft: WAFRuleGroupPayload = {
  name: '',
  enabled: true,
  block_status_code: 418,
  block_response_body: '',
  ip_whitelist: [],
  ip_blacklist: [],
  ip_whitelist_group_ids: [],
  ip_blacklist_group_ids: [],
  country_whitelist: [],
  country_blacklist: [],
  region_whitelist: [],
  region_blacklist: [],
  pow_enabled: false,
  pow_config: defaultPowConfig,
  remark: '',
};

export const defaultRuleModalState: RuleModalState = {
  open: false,
  listType: 'blacklist',
  dimension: 'ip',
  ipValue: '',
  ipGroupIDs: [],
  countryValues: [],
};

export const wafTabItems: Array<{ id: WAFTab; label: string }> = [
  { id: 'basic', label: '基本信息' },
  { id: 'lists', label: '黑白名单' },
  { id: 'pow', label: 'PoW' },
  { id: 'block', label: '拦截返回' },
];

export function getErrorMessage(error: unknown) {
  return error instanceof Error ? error.message : '操作失败';
}

export function textToList(text: string) {
  return text
    .split(/[\n,，\s]+/)
    .map((item) => item.trim())
    .filter(Boolean);
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

export function normalizeItems(items: string[]) {
  return Array.from(
    new Set(items.map((item) => item.trim()).filter(Boolean)),
  ).sort((left, right) => left.localeCompare(right));
}

export function buildRuleGroupDraft(group: WAFRuleGroup | null): WAFRuleGroupPayload {
  if (!group) {
    return { ...emptyRuleGroupDraft };
  }
  return {
    name: group.name,
    enabled: group.enabled,
    block_status_code: group.block_status_code || 418,
    block_response_body: group.block_response_body ?? '',
    ip_whitelist: group.ip_whitelist ?? [],
    ip_blacklist: group.ip_blacklist ?? [],
    ip_whitelist_group_ids: group.ip_whitelist_group_ids ?? [],
    ip_blacklist_group_ids: group.ip_blacklist_group_ids ?? [],
    country_whitelist: group.country_whitelist ?? [],
    country_blacklist: group.country_blacklist ?? [],
    region_whitelist: group.region_whitelist ?? [],
    region_blacklist: group.region_blacklist ?? [],
    pow_enabled: group.pow_enabled ?? false,
    pow_config: group.pow_config ?? defaultPowConfig,
    remark: group.remark ?? '',
  };
}

export function countRuleEntries(group: RuleListRenderable) {
  return (
    group.ip_whitelist.length +
    group.ip_blacklist.length +
    group.ip_whitelist_group_ids.length +
    group.ip_blacklist_group_ids.length +
    group.country_whitelist.length +
    group.country_blacklist.length +
    group.region_whitelist.length +
    group.region_blacklist.length
  );
}

export function buildCountryOptions() {
  const zhDisplayNames = new Intl.DisplayNames(['zh-CN'], { type: 'region' });
  const enDisplayNames = new Intl.DisplayNames(['en'], { type: 'region' });
  const options: CountryOption[] = [];

  for (let first = 65; first <= 90; first += 1) {
    for (let second = 65; second <= 90; second += 1) {
      const code = String.fromCharCode(first, second);
      const zhName = zhDisplayNames.of(code);
      const enName = enDisplayNames.of(code);

      if (
        !zhName ||
        zhName === code ||
        /未知/.test(zhName) ||
        !enName ||
        enName === code ||
        /Unknown/.test(enName)
      ) {
        continue;
      }

      options.push({
        code,
        zhName,
        label: `${code} ${zhName}`,
        searchText: `${code} ${zhName} ${enName}`.toLowerCase(),
      });
    }
  }

  return options.sort((left, right) => left.code.localeCompare(right.code));
}

export function getListFieldKey(
  listType: RuleListType,
  dimension: RuleDimension,
): ListFieldKey {
  if (dimension === 'ip') {
    return listType === 'whitelist' ? 'ip_whitelist' : 'ip_blacklist';
  }
  if (dimension === 'ip_group') {
    return listType === 'whitelist'
      ? 'ip_whitelist_group_ids'
      : 'ip_blacklist_group_ids';
  }
  return listType === 'whitelist' ? 'country_whitelist' : 'country_blacklist';
}

export function updateDraftList(
  draft: WAFRuleGroupPayload,
  key: ListFieldKey,
  updater: (items: string[]) => string[],
) {
  switch (key) {
    case 'ip_whitelist':
      return { ...draft, ip_whitelist: updater(draft.ip_whitelist) };
    case 'ip_blacklist':
      return { ...draft, ip_blacklist: updater(draft.ip_blacklist) };
    case 'ip_whitelist_group_ids':
      return {
        ...draft,
        ip_whitelist_group_ids: updater(
          draft.ip_whitelist_group_ids.map(String),
        ).map(Number),
      };
    case 'ip_blacklist_group_ids':
      return {
        ...draft,
        ip_blacklist_group_ids: updater(
          draft.ip_blacklist_group_ids.map(String),
        ).map(Number),
      };
    case 'country_whitelist':
      return { ...draft, country_whitelist: updater(draft.country_whitelist) };
    case 'country_blacklist':
      return { ...draft, country_blacklist: updater(draft.country_blacklist) };
  }
}

export function formatCountryItem(code: string, labelMap: Map<string, string>) {
  return labelMap.get(code) ?? code;
}

export function parseAutomaticConfig(text: string): Record<string, unknown> {
  const parsed = JSON.parse(text || '{}') as unknown;
  if (!parsed || Array.isArray(parsed) || typeof parsed !== 'object') {
    throw new Error('自动配置必须是 JSON 对象。');
  }
  return parsed as Record<string, unknown>;
}

export const ipGroupTypeLabels = {
  manual: '手动',
  automatic: '自动',
  subscription: '订阅',
} as const;

export const automaticPresetRules = [
  {
    name: '单 IP 404 高频扫描',
    expr: 'request_count > 100 && StatusRatio(404) >= 0.8',
  },
  {
    name: '单 IP 直连访问异常',
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
    remark: group.remark ?? '',
  };
}

export function formatIPGroupBanRemaining(
  capturedAt: string,
  ttlSeconds: number,
  now = new Date(),
): string {
  if (ttlSeconds <= 0) {
    return '永久';
  }
  const capturedDate = new Date(capturedAt);
  if (Number.isNaN(capturedDate.getTime())) {
    return '—';
  }
  const expireDate = new Date(capturedDate.getTime() + ttlSeconds * 1000);
  if (expireDate.getTime() <= now.getTime()) {
    return '已过期';
  }
  const diffMins = Math.round((expireDate.getTime() - now.getTime()) / (60 * 1000));
  if (diffMins < 60) {
    return `${diffMins} 分钟后`;
  }
  return `${Math.round(diffMins / 60)} 小时后`;
}

export function getIPGroupViewEntries(group: WAFIPGroup): IPGroupViewEntry[] {
  if (group.type === 'automatic' && group.ext_ips && group.ext_ips.length > 0) {
    const ttl =
      typeof group.auto_config?.ttl === 'number' ? group.auto_config.ttl : -1;
    return group.ext_ips.map((item) => ({
      ip: item.ip,
      capturedAt: item.captured_at,
      banRemaining: formatIPGroupBanRemaining(item.captured_at, ttl),
    }));
  }
  return (group.ip_list ?? []).map((ip) => ({ ip }));
}
