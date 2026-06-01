import type { ProxyRoutePoWConfig } from '@/features/proxy-routes/types';
import type { WAFRuleGroup, WAFRuleGroupPayload } from '@/features/waf/types';
import type {
  CountryOption,
  ListFieldKey,
  RuleDimension,
  RuleListRenderable,
  RuleListType,
  RuleModalState,
  WAFTab,
} from './types';

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

export const emptyDraft: WAFRuleGroupPayload = {
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

export const tabItems: Array<{
  id: WAFTab;
  label: string;
}> = [
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

export function buildDraft(group: WAFRuleGroup | null): WAFRuleGroupPayload {
  if (!group) {
    return { ...emptyDraft };
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
