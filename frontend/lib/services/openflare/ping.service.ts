import { OpenFlareBaseService } from './base.service';
import type {
  PingTask,
  PingTaskBatchUpdateItem,
  PingTaskPayload,
} from './types';

export class PingService extends OpenFlareBaseService {
  protected static override readonly basePath: string = '/api/v1/d/ping-tasks';

  static async listTasks(): Promise<PingTask[]> {
    return this.get<PingTask[]>('/');
  }

  static async createTask(payload: PingTaskPayload): Promise<PingTask> {
    return this.post<PingTask>('/', payload);
  }

  static async updateTask(
    id: number,
    payload: PingTaskPayload,
  ): Promise<PingTask> {
    return this.post<PingTask>(`/${id}/update`, payload);
  }

  static async batchUpdateTasks(
    tasks: PingTaskBatchUpdateItem[],
  ): Promise<PingTask[]> {
    return this.post<PingTask[]>('/batch-update', { tasks });
  }

  static async deleteTask(id: number): Promise<{ deleted: number }> {
    return this.post<{ deleted: number }>(`/${id}/delete`);
  }

  static async batchDeleteTasks(ids: number[]): Promise<{ deleted: number }> {
    return this.post<{ deleted: number }>('/delete', { ids });
  }

  static async reorderTasks(weights: Record<string, number>): Promise<void> {
    return this.post<void>('/order', weights);
  }
}
