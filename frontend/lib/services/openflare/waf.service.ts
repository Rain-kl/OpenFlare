import type {
  AttackGroup,
  AttackGroupInput,
  AttackEvent,
  AttackLog,
  AttackLogFilter,
  AttackLogPage,
  AttackLogRequest,
  RequestWhitelistEntry,
  RequestWhitelistInput,
} from './attack-types';
import { OpenFlareBaseService } from './base.service';
import type {
  WAFIPGroup,
  WAFIPGroupAutoTestPayload,
  WAFIPGroupAutoTestResult,
  WAFIPGroupPayload,
  WAFIPGroupSyncResult,
  WAFCreateRulePayload,
  WAFRule,
  WAFRuleGroupSite,
  WAFSaveRuleGraphPayload,
  WAFSiteRuleGroups,
  WAFUpdateRuleMetaPayload,
} from './types';

export const attackGroupsQueryKey = [
  'openflare',
  'waf',
  'attack-groups',
] as const;

export const requestWhitelistQueryKey = [
  'openflare',
  'waf',
  'request-whitelist',
] as const;

export const ruleGroupSitesQueryKey = [
  'openflare',
  'waf',
  'rule-group-sites',
] as const;

export class WafService extends OpenFlareBaseService {
  protected static override readonly basePath: string = '/api/v1/d/waf';

  static async listAttackGroups(): Promise<AttackGroup[]> {
    return this.get<AttackGroup[]>('/attack-groups');
  }

  static async saveAttackGroup(
    input: AttackGroupInput,
    id?: number,
  ): Promise<AttackGroup> {
    return this.post<AttackGroup>(
      id ? `/attack-groups/${id}/update` : '/attack-groups',
      input,
    );
  }

  static async listRequestWhitelist(): Promise<RequestWhitelistEntry[]> {
    return this.get<RequestWhitelistEntry[]>('/request-whitelist');
  }

  static async saveRequestWhitelistEntry(
    input: RequestWhitelistInput,
    id?: number,
  ): Promise<RequestWhitelistEntry> {
    return this.post<RequestWhitelistEntry>(
      id ? `/request-whitelist/${id}/update` : '/request-whitelist',
      input,
    );
  }

  static async deleteRequestWhitelistEntry(id: number): Promise<void> {
    await this.post(`/request-whitelist/${id}/delete`);
  }

  static async listAttackLogs(
    filter: AttackLogFilter,
  ): Promise<AttackLogPage<AttackLog>> {
    return this.get<AttackLogPage<AttackLog>>('/attack-logs', filter);
  }

  static async getAttackLogRequest(log: AttackLog): Promise<AttackLogRequest> {
    return this.get<AttackLogRequest>(`/attack-logs/${log.id}/request`, {
      node_id: log.node_id,
      logged_at: Math.floor(Date.parse(log.logged_at) / 1000),
    });
  }

  static async listAttackEvents(
    filter: AttackLogFilter,
  ): Promise<AttackLogPage<AttackEvent>> {
    return this.get<AttackLogPage<AttackEvent>>('/attack-events', filter);
  }

  static async listRuleGroups(): Promise<WAFRule[]> {
    return this.get<WAFRule[]>('/rule-groups');
  }

  static async getRule(id: number): Promise<WAFRule> {
    return this.get<WAFRule>(`/rule-groups/${id}`);
  }

  static async listRuleGroupSites(id: number): Promise<WAFRuleGroupSite[]> {
    return this.get<WAFRuleGroupSite[]>(`/rule-groups/${id}/sites`);
  }

  static async removeRuleGroupSites(
    id: number,
    routeIds: number[],
  ): Promise<{ removed_count: number }> {
    return this.post<{ removed_count: number }>(
      `/rule-groups/${id}/sites/remove`,
      { ids: routeIds },
    );
  }

  static async createRule(payload: WAFCreateRulePayload): Promise<WAFRule> {
    return this.post<WAFRule>('/rule-groups', payload);
  }

  static async saveRuleGraph(
    id: number,
    payload: WAFSaveRuleGraphPayload,
  ): Promise<WAFRule> {
    return this.post<WAFRule>(`/rule-groups/${id}/graph`, payload);
  }

  static async updateRuleMeta(
    id: number,
    payload: WAFUpdateRuleMetaPayload,
  ): Promise<WAFRule> {
    return this.post<WAFRule>(`/rule-groups/${id}/meta`, payload);
  }

  static async deleteRuleGroup(id: number): Promise<void> {
    return this.post<void>(`/rule-groups/${id}/delete`);
  }

  static async listSiteRuleGroups(routeId: number): Promise<WAFSiteRuleGroups> {
    return this.get<WAFSiteRuleGroups>(`/sites/${routeId}/rule-groups`);
  }

  static async updateSiteRuleGroups(
    routeId: number,
    ids: number[],
  ): Promise<WAFSiteRuleGroups> {
    return this.post<WAFSiteRuleGroups>(`/sites/${routeId}/rule-groups`, {
      ids,
    });
  }

  static async listIPGroups(): Promise<WAFIPGroup[]> {
    return this.get<WAFIPGroup[]>('/ip-groups');
  }

  static async getIPGroup(id: number): Promise<WAFIPGroup> {
    return this.get<WAFIPGroup>(`/ip-groups/${id}`);
  }

  static async createIPGroup(payload: WAFIPGroupPayload): Promise<WAFIPGroup> {
    return this.post<WAFIPGroup>('/ip-groups', payload);
  }

  static async updateIPGroup(
    id: number,
    payload: WAFIPGroupPayload,
  ): Promise<WAFIPGroup> {
    return this.post<WAFIPGroup>(`/ip-groups/${id}/update`, payload);
  }

  static async deleteIPGroup(id: number): Promise<void> {
    return this.post<void>(`/ip-groups/${id}/delete`);
  }

  static async testIPGroup(
    payload: WAFIPGroupAutoTestPayload,
  ): Promise<WAFIPGroupAutoTestResult> {
    return this.post<WAFIPGroupAutoTestResult>('/ip-groups/test', payload);
  }

  static async syncIPGroup(id: number): Promise<WAFIPGroupSyncResult> {
    return this.post<WAFIPGroupSyncResult>(`/ip-groups/${id}/sync`);
  }
}
