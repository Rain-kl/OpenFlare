import {OpenFlareBaseService} from './base.service';
import type {
  ApplyLogCleanupPayload,
  ApplyLogCleanupResult,
  ApplyLogList,
  ApplyLogListQuery,
} from './types';

export class ApplyLogService extends OpenFlareBaseService {
  protected static override readonly basePath: string = '/api/v1/d/apply-logs';

  static list(query: ApplyLogListQuery = {}): Promise<ApplyLogList> {
    const params: Record<string, unknown> = {};

    const normalizedNodeId = query.node_id?.trim();
    if (normalizedNodeId) {
      params.node_id = normalizedNodeId;
    }
    if (query.pageNo) {
      params.pageNo = query.pageNo;
    }
    if (query.pageSize) {
      params.pageSize = query.pageSize;
    }

    return this.get<ApplyLogList>('/', params);
  }

  static cleanup(payload: ApplyLogCleanupPayload): Promise<ApplyLogCleanupResult> {
    return this.post<ApplyLogCleanupResult>('/cleanup', payload);
  }
}