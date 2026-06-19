import {BaseService} from '@/lib/services/core';

export class AdminLogService extends BaseService {
  protected static readonly basePath = '/api/v1/admin';

  static async getLogs(cursor: number = 0, limit: number = 200): Promise<{
    lines: Array<{ index: number; data: string }>;
    has_more: boolean;
    next_cursor: number;
  }> {
    return this.get('/logs', { cursor, limit });
  }

  static async getAccessLogs(params: {
    page: number;
    page_size: number;
    username?: string;
    path?: string;
    start_time?: string;
    end_time?: string;
  }): Promise<{
    total: number;
    list: Array<{
      id: string;
      user_id: string;
      username: string;
      nickname: string;
      path: string;
      method: string;
      ip: string;
      user_agent: string;
      headers: string;
      status: number;
      latency: number;
      created_at: string;
    }>;
  }> {
    return this.get('/logs/access', params as Record<string, unknown>);
  }

  static async getLogsAnalytics(): Promise<{
    trend: Array<{ date: string; count: number }>;
    browsers: Array<{ browser: string; count: number }>;
    top_users: Array<{
      user_id: string;
      username: string;
      nickname: string;
      count: number;
    }>;
  }> {
    return this.get('/logs/analytics');
  }
}