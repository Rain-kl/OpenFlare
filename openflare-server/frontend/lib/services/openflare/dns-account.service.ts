import {OpenFlareBaseService} from './base.service';
import type {DnsAccountItem, DnsAccountMutationPayload} from './types';

export class DnsAccountService extends OpenFlareBaseService {
  protected static override readonly basePath: string = '/api/v1/d/dns-accounts';

  static async list(): Promise<DnsAccountItem[]> {
    return this.get<DnsAccountItem[]>('/');
  }

  static async create(
    payload: DnsAccountMutationPayload,
  ): Promise<DnsAccountItem> {
    return this.post<DnsAccountItem>('/', payload);
  }

  static async update(
    id: number,
    payload: DnsAccountMutationPayload,
  ): Promise<DnsAccountItem> {
    return this.post<DnsAccountItem>(`/${id}/update`, payload);
  }

  static async deleteById(id: number): Promise<void> {
    return this.post<void>(`/${id}/delete`);
  }
}