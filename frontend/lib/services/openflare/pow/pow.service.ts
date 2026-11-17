import { OpenFlareBaseService } from '../base.service';
import type { PoWPolicy } from './types';

export class PoWService extends OpenFlareBaseService {
  protected static override readonly basePath: string = '/api/v1/d/pow';

  static async list(): Promise<PoWPolicy[]> {
    return this.get<PoWPolicy[]>('/policies');
  }

  static async save(policy: PoWPolicy): Promise<void> {
    return this.put<void>(`/policies/${policy.route_id}`, policy);
  }
}
