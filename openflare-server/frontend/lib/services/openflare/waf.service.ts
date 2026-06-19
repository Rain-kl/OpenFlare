import {OpenFlareBaseService} from './base.service';
import type {
  WAFIPGroup,
  WAFIPGroupAutoTestPayload,
  WAFIPGroupAutoTestResult,
  WAFIPGroupPayload,
  WAFIPGroupSyncResult,
  WAFRuleGroup,
  WAFRuleGroupPayload,
  WAFSiteRuleGroups,
} from './types';

export class WafService extends OpenFlareBaseService {
  protected static override readonly basePath: string = '/api/v1/d/waf';

  static async listRuleGroups(): Promise<WAFRuleGroup[]> {
    return this.get<WAFRuleGroup[]>('/rule-groups');
  }

  static async getRuleGroup(id: number): Promise<WAFRuleGroup> {
    return this.get<WAFRuleGroup>(`/rule-groups/${id}`);
  }

  static async createRuleGroup(payload: WAFRuleGroupPayload): Promise<WAFRuleGroup> {
    return this.post<WAFRuleGroup>('/rule-groups', payload);
  }

  static async updateRuleGroup(
    id: number,
    payload: WAFRuleGroupPayload,
  ): Promise<WAFRuleGroup> {
    return this.post<WAFRuleGroup>(`/rule-groups/${id}/update`, payload);
  }

  static async deleteRuleGroup(id: number): Promise<void> {
    return this.post<void>(`/rule-groups/${id}/delete`);
  }

  static async updateRuleGroupSites(id: number, ids: number[]): Promise<WAFRuleGroup> {
    return this.post<WAFRuleGroup>(`/rule-groups/${id}/sites`, { ids });
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

  static async updateIPGroup(id: number, payload: WAFIPGroupPayload): Promise<WAFIPGroup> {
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