import { OpenFlareBaseService } from '../base.service';
import type { PerformancePolicies, PerformancePolicy } from './types';

export class PerformanceService extends OpenFlareBaseService {
  protected static override readonly basePath =
    '/api/v1/d/performance-policies';

  static list() {
    return this.get<PerformancePolicies>('/');
  }

  static save(id: number | undefined, policy: Omit<PerformancePolicy, 'id'>) {
    return this.post<PerformancePolicy>(id ? `/${id}/update` : '/', policy);
  }

  static assign(id: number, nodeIDs: string[]) {
    return this.post<void>(`/${id}/assign`, { node_ids: nodeIDs });
  }

  static remove(id: number) {
    return this.post<void>(`/${id}/delete`);
  }
}
