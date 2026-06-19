import {OpenFlareBaseService} from './base.service';
import type {ProxyRouteItem, ProxyRouteMutationPayload} from './types';

export class ProxyRouteService extends OpenFlareBaseService {
  protected static override readonly basePath: string = '/api/v1/d/proxy-routes';

  static async list(): Promise<ProxyRouteItem[]> {
    return this.get<ProxyRouteItem[]>('/');
  }

  static async getById(id: number): Promise<ProxyRouteItem> {
    return this.get<ProxyRouteItem>(`/${id}`);
  }

  static async create(payload: ProxyRouteMutationPayload): Promise<ProxyRouteItem> {
    return this.post<ProxyRouteItem>('/', payload);
  }

  static async update(
    id: number,
    payload: ProxyRouteMutationPayload,
  ): Promise<ProxyRouteItem> {
    return this.post<ProxyRouteItem>(`/${id}/update`, payload);
  }

  static async deleteById(id: number): Promise<void> {
    return this.post<void>(`/${id}/delete`);
  }
}