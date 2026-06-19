import {BaseService} from '@/lib/services/core';
import type {
  CreateScheduleRequest,
  DispatchTaskRequest,
  ListTaskExecutionsRequest,
  ListTaskExecutionsResponse,
  Schedule,
  TaskExecution,
  TaskMeta,
  TaskParamType,
  TaskTypeResponse,
  UpdateScheduleRequest,
} from './types';

export class AdminTaskService extends BaseService {
  protected static readonly basePath = '/api/v1/admin';

  static async getTaskTypes(): Promise<TaskMeta[]> {
    const response = await this.get<TaskTypeResponse[]>('/tasks/types');
    return response.map((item) => ({
      type: item.Type || item.type || '',
      asynq_task: item.AsynqTask || item.asynq_task || '',
      name: item.Name || item.name || '',
      description: item.Description || item.description || '',
      supports_time: item.SupportsTime ?? item.supports_time ?? false,
      max_retry: item.MaxRetry ?? item.max_retry ?? 0,
      queue: item.Queue || item.queue || '',
      params: (item.Params || item.params || []).map((p) => ({
        name: p.Name || p.name || '',
        label: p.Label || p.label || '',
        type: (p.Type || p.type || 'string') as TaskParamType,
        required: p.Required ?? p.required ?? false,
        placeholder: p.Placeholder || p.placeholder || '',
        description: p.Description || p.description || '',
      })),
    }));
  }

  static async dispatchTask(request: DispatchTaskRequest): Promise<string> {
    return this.post<string>('/tasks/dispatch', request);
  }

  static async listTaskExecutions(
    request: ListTaskExecutionsRequest = {},
  ): Promise<ListTaskExecutionsResponse> {
    return this.get<ListTaskExecutionsResponse>(
      '/tasks/executions',
      request as unknown as Record<string, unknown>,
    );
  }

  static async getTaskExecution(id: string): Promise<TaskExecution> {
    return this.get<TaskExecution>(`/tasks/executions/${id}`);
  }

  static async retryTaskExecution(id: string): Promise<string> {
    return this.post<string>(`/tasks/executions/${id}/retry`);
  }

  static async listSchedules(): Promise<Schedule[]> {
    return this.get<Schedule[]>('/tasks/schedules');
  }

  static async createSchedule(request: CreateScheduleRequest): Promise<Schedule> {
    return this.post<Schedule>('/tasks/schedules', request);
  }

  static async updateSchedule(id: string, request: UpdateScheduleRequest): Promise<Schedule> {
    return this.put<Schedule>(`/tasks/schedules/${id}`, request);
  }

  static async deleteSchedule(id: string): Promise<void> {
    return this.delete<void>(`/tasks/schedules/${id}`);
  }
}