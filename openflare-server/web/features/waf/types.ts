import type { ProxyRoutePoWConfig } from '@/features/proxy-routes/types';

export interface WAFRuleGroup {
  id: number;
  name: string;
  enabled: boolean;
  is_global: boolean;
  block_status_code: number;
  block_response_body: string;
  ip_whitelist: string[];
  ip_blacklist: string[];
  ip_whitelist_group_ids: number[];
  ip_blacklist_group_ids: number[];
  country_whitelist: string[];
  country_blacklist: string[];
  region_whitelist: string[];
  region_blacklist: string[];
  pow_enabled: boolean;
  pow_config: ProxyRoutePoWConfig;
  remark: string;
  applied_site_ids: number[];
  applied_site_count: number;
  created_at: string;
  updated_at: string;
}

export interface WAFRuleGroupPayload {
  name: string;
  enabled: boolean;
  block_status_code: number;
  block_response_body: string;
  ip_whitelist: string[];
  ip_blacklist: string[];
  ip_whitelist_group_ids: number[];
  ip_blacklist_group_ids: number[];
  country_whitelist: string[];
  country_blacklist: string[];
  region_whitelist: string[];
  region_blacklist: string[];
  pow_enabled: boolean;
  pow_config: ProxyRoutePoWConfig;
  remark: string;
}

export interface WAFSiteRuleGroups {
  route_id: number;
  global_rule_group: WAFRuleGroup | null;
  rule_groups: WAFRuleGroup[];
  applied_rule_groups: WAFRuleGroup[];
  applied_ids: number[];
}

export type WAFIPGroupType = 'manual' | 'automatic' | 'subscription';
export type WAFIPGroupSubscriptionFormat = 'text' | 'json';

export interface WAFIPGroupExtIP {
  ip: string;
  captured_at: string;
}

export interface WAFIPGroup {
  id: number;
  name: string;
  type: WAFIPGroupType;
  enabled: boolean;
  ip_list: string[];
  auto_config: Record<string, unknown>;
  ext_ips?: WAFIPGroupExtIP[];
  subscription_url: string;
  subscription_format: WAFIPGroupSubscriptionFormat;
  subscription_mapping_rule: string;
  sync_interval_minutes: number;
  last_synced_at?: string;
  next_sync_at?: string;
  last_sync_status: string;
  last_sync_message: string;
  remark: string;
  referenced_by_rule_count: number;
  created_at: string;
  updated_at: string;
}

export interface WAFIPGroupPayload {
  name: string;
  type: WAFIPGroupType;
  enabled: boolean;
  ip_list: string[];
  auto_config: Record<string, unknown>;
  subscription_url: string;
  subscription_format: WAFIPGroupSubscriptionFormat;
  subscription_mapping_rule: string;
  sync_interval_minutes: number;
  remark: string;
}

export interface WAFIPGroupSyncResult {
  group: WAFIPGroup;
  ip_count: number;
  synced_at: string;
  next_sync_at: string;
  status: string;
  message: string;
}

export interface WAFIPGroupAutoTestPayload {
  auto_config: Record<string, unknown>;
}

export interface WAFIPGroupAutoTestResult {
  matched_ips: string[];
  matched_count: number;
  lookback_minutes: number;
  rule_count: number;
  tested_at: string;
}
