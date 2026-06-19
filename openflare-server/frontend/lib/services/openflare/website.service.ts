import {OpenFlareBaseService} from './base.service';
import type {
  ManagedDomainItem,
  ManagedDomainMatchResult,
  ManagedDomainMutationPayload,
} from './types';

export class WebsiteService extends OpenFlareBaseService {
  protected static override readonly basePath: string = '/api/v1/d/managed-domains';

  static async list(): Promise<ManagedDomainItem[]> {
    return this.get<ManagedDomainItem[]>('/');
  }

  static async create(
    payload: ManagedDomainMutationPayload,
  ): Promise<ManagedDomainItem> {
    return this.post<ManagedDomainItem>('/', payload);
  }

  static async update(
    id: number,
    payload: ManagedDomainMutationPayload,
  ): Promise<ManagedDomainItem> {
    return this.post<ManagedDomainItem>(`/${id}/update`, payload);
  }

  static async deleteById(id: number): Promise<void> {
    return this.post<void>(`/${id}/delete`);
  }

  static async match(domain: string): Promise<ManagedDomainMatchResult> {
    return this.get<ManagedDomainMatchResult>('/match', {domain});
  }
}