import { OpenFlareBaseService } from './base.service';
import type { ExecRunPayload, ExecRunResponse, ExecTask } from './types';

export class ExecService extends OpenFlareBaseService {
  protected static override readonly basePath: string = '/api/v1/d/remote-exec';

  static async runCommand(payload: ExecRunPayload): Promise<ExecRunResponse> {
    return this.post<ExecRunResponse>('/', payload);
  }

  static async listTasks(limit = 50): Promise<ExecTask[]> {
    return this.get<ExecTask[]>('/tasks', { limit });
  }

  static async getTask(taskId: string): Promise<ExecTask> {
    return this.get<ExecTask>(`/tasks/${encodeURIComponent(taskId)}`);
  }

  static async deleteTask(taskId: string): Promise<void> {
    return this.post<void>(`/tasks/${encodeURIComponent(taskId)}/delete`);
  }
}
