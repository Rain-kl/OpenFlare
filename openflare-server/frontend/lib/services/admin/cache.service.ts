import {BaseService} from '@/lib/services/core';
import type {CacheConfig, CacheStatus} from './types';

export class AdminCacheService extends BaseService {
  protected static readonly basePath = '/api/v1/admin';

  static async getCacheStatus(): Promise<CacheStatus> {
    return this.get<CacheStatus>('/cache/status');
  }

  static async updateCacheConfig(config: CacheConfig): Promise<void> {
    return this.post<void>('/cache/config', config);
  }

  static async clearCache(): Promise<void> {
    return this.post<void>('/cache/clear');
  }
}