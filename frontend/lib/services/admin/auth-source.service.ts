import {BaseService} from '@/lib/services/core';
import type {AuthSource, AuthSourceRequest, ToggleAuthSourceRequest,} from './types';

export class AdminAuthSourceService extends BaseService {
  protected static readonly basePath = '/api/v1/admin';

  static async listAuthSources(): Promise<AuthSource[]> {
    return this.get<AuthSource[]>('/auth-sources');
  }

  static async createAuthSource(
    request: AuthSourceRequest,
  ): Promise<AuthSource> {
    return this.post<AuthSource>('/auth-sources', request);
  }

  static async updateAuthSource(
    id: string,
    request: AuthSourceRequest,
  ): Promise<AuthSource> {
    return this.put<AuthSource>(`/auth-sources/${id}`, request);
  }

  static async toggleAuthSource(
    id: string,
    request: ToggleAuthSourceRequest,
  ): Promise<void> {
    return this.put<void>(`/auth-sources/${id}/toggle`, request);
  }

  static async deleteAuthSource(id: string): Promise<void> {
    return this.delete<void>(`/auth-sources/${id}`);
  }
}
