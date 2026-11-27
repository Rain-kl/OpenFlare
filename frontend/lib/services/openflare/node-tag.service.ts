import { OpenFlareBaseService } from './base.service';
import type { NodeTagInput, NodeTagItem } from './types';

export const nodeTagQueryKey = ['openflare', 'node-tags'] as const;

export class NodeTagService extends OpenFlareBaseService {
  protected static override readonly basePath: string = '/api/v1/d/node-tags';

  static list(): Promise<NodeTagItem[]> {
    return this.get<NodeTagItem[]>('/');
  }

  static create(payload: NodeTagInput): Promise<NodeTagItem> {
    return this.post<NodeTagItem>('/', payload);
  }

  static update(id: number, payload: NodeTagInput): Promise<NodeTagItem> {
    return this.post<NodeTagItem>(`/${id}/update`, payload);
  }

  static deleteById(id: number): Promise<void> {
    return this.post<void>(`/${id}/delete`);
  }
}
