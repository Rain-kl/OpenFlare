import { OpenFlareBaseService } from './base.service';
import type {
  CloudflareAvailableDomain,
  CloudflareConnection,
  CloudflareConnectionPayload,
  CloudflareGroup,
  CloudflareGroupDetail,
  CloudflareGroupPayload,
  CloudflareMember,
  CloudflareMemberCreatePayload,
  CloudflareOverview,
  CloudflareSyncReceipt,
} from './types';

export const cloudflareQueryKey = ['openflare', 'cloudflare'] as const;

export class CloudflareService extends OpenFlareBaseService {
  protected static override readonly basePath = '/api/v1/d/cloudflare';

  static getConnection(): Promise<CloudflareConnection> {
    return this.get<CloudflareConnection>('/connection');
  }

  static saveConnection(
    payload: CloudflareConnectionPayload,
  ): Promise<CloudflareConnection> {
    return this.put<CloudflareConnection>('/connection', payload);
  }

  static verifyConnection(): Promise<CloudflareConnection> {
    return this.post<CloudflareConnection>('/connection/verify');
  }

  static clearConnection(): Promise<void> {
    return this.post<void>('/connection/clear');
  }

  static getOverview(): Promise<CloudflareOverview> {
    return this.get<CloudflareOverview>('/overview');
  }

  static listGroups(): Promise<CloudflareGroup[]> {
    return this.get<CloudflareGroup[]>('/groups');
  }

  static getGroup(id: number): Promise<CloudflareGroupDetail> {
    return this.get<CloudflareGroupDetail>(`/groups/${id}`);
  }

  static createGroup(
    payload: CloudflareGroupPayload,
  ): Promise<CloudflareGroup> {
    return this.post<CloudflareGroup>('/groups', payload);
  }

  static updateGroup(
    id: number,
    payload: CloudflareGroupPayload,
  ): Promise<CloudflareGroup> {
    return this.post<CloudflareGroup>(`/groups/${id}/update`, payload);
  }

  static deleteGroup(id: number): Promise<void> {
    return this.post<void>(`/groups/${id}/delete`);
  }

  static syncGroup(id: number): Promise<CloudflareSyncReceipt> {
    return this.post<CloudflareSyncReceipt>(`/groups/${id}/sync`);
  }

  static listAvailableDomains(): Promise<CloudflareAvailableDomain[]> {
    return this.get<CloudflareAvailableDomain[]>('/domains/available');
  }

  static createMember(
    groupId: number,
    payload: CloudflareMemberCreatePayload,
  ): Promise<CloudflareMember> {
    return this.post<CloudflareMember>(`/groups/${groupId}/members`, payload);
  }

  static updateMember(
    groupId: number,
    memberId: number,
    proxied: boolean,
  ): Promise<CloudflareMember> {
    return this.post<CloudflareMember>(
      `/groups/${groupId}/members/${memberId}/update`,
      { proxied },
    );
  }

  static removeMember(groupId: number, memberId: number): Promise<void> {
    return this.post<void>(`/groups/${groupId}/members/${memberId}/remove`);
  }

  static syncMember(
    groupId: number,
    memberId: number,
  ): Promise<CloudflareSyncReceipt> {
    return this.post<CloudflareSyncReceipt>(
      `/groups/${groupId}/members/${memberId}/sync`,
    );
  }
}
