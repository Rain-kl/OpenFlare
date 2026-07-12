import {OpenFlareBaseService} from './base.service'
import type {
  ZoneDomainItem,
  ZoneDomainMutationPayload,
  ZoneItem,
  ZoneMutationPayload,
  ZoneOverview,
  ZoneStats,
  ZoneStatsRange,
} from './types'

export const zoneQueryKey = ['openflare', 'zones'] as const

export class ZoneService extends OpenFlareBaseService {
  protected static override readonly basePath = '/api/v1/d/zones'

  static list(): Promise<ZoneItem[]> { return this.get<ZoneItem[]>('/') }
  static getOverview(id: number): Promise<ZoneOverview> { return this.get<ZoneOverview>(`/${id}/overview`) }
  static getStats(id: number, range: ZoneStatsRange = '24h'): Promise<ZoneStats> {
    return this.get<ZoneStats>(`/${id}/stats`, {range})
  }
  static create(payload: ZoneMutationPayload): Promise<ZoneItem> { return this.post<ZoneItem>('/', payload) }
  static update(id: number, payload: ZoneMutationPayload): Promise<ZoneItem> { return this.post<ZoneItem>(`/${id}/update`, payload) }
  static deleteById(id: number): Promise<void> { return this.post<void>(`/${id}/delete`) }
}

export class ZoneDomainService extends OpenFlareBaseService {
  protected static override readonly basePath = '/api/v1/d/zones'

  static create(zoneId: number, payload: ZoneDomainMutationPayload): Promise<ZoneDomainItem> { return this.post<ZoneDomainItem>(`/${zoneId}/domains`, payload) }
  static update(zoneId: number, id: number, payload: ZoneDomainMutationPayload): Promise<ZoneDomainItem> { return this.post<ZoneDomainItem>(`/${zoneId}/domains/${id}/update`, payload) }
  static deleteById(zoneId: number, id: number): Promise<void> { return this.post<void>(`/${zoneId}/domains/${id}/delete`) }
}
