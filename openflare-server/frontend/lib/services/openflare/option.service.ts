import {OpenFlareBaseService} from './base.service';
import type {
  DatabaseCleanupPayload,
  DatabaseCleanupResult,
  GeoIPLookupResult,
  OptionBatchPayload,
  OptionItem,
} from './types';

export class OptionService extends OpenFlareBaseService {
  protected static override readonly basePath: string = '/api/v1/d/option';

  static list(): Promise<OptionItem[]> {
    return this.get<OptionItem[]>('/');
  }

  static update(key: string, value: string): Promise<void> {
    return this.post<void>('/update', { key, value });
  }

  static updateBatch(options: OptionItem[]): Promise<void> {
    const payload: OptionBatchPayload = { options };
    return this.post<void>('/update-batch', payload);
  }

  static lookupGeoIP(provider: string, ip: string): Promise<GeoIPLookupResult> {
    return this.post<GeoIPLookupResult>('/geoip/lookup', { provider, ip });
  }

  static cleanupDatabase(
    payload: DatabaseCleanupPayload,
  ): Promise<DatabaseCleanupResult> {
    return this.post<DatabaseCleanupResult>('/database/cleanup', payload);
  }
}