import type {InternalAxiosRequestConfig} from 'axios';

import apiClient from '@/lib/services/core/api-client';
import {BaseService} from '@/lib/services/core';
import type {DBOverview, ExecuteSQLResponse, TableDataResponse} from './types';

/**
 * 数据库管理服务类
 */
export class DbManageService extends BaseService {
  protected static readonly basePath = '/api/v1/admin/db-manage';

  /**
   * 获取数据库运行概览
   */
  static async getOverview(): Promise<DBOverview> {
    return this.get<DBOverview>('/overview');
  }

  /**
   * 获取数据库所有物理表名
   */
  static async listTables(): Promise<string[]> {
    return this.get<string[]>('/tables');
  }

  /**
   * 获取某张数据表的数据（分页）
   */
  static async getTableData(params: {
    table: string;
    page: number;
    pageSize: number;
  }): Promise<TableDataResponse> {
    return this.get<TableDataResponse>('/table-data', params as unknown as Record<string, unknown>);
  }

  /**
   * 在数据库中执行自定义 SQL 语句
   */
  static async executeSQL(sql: string): Promise<ExecuteSQLResponse> {
    return this.post<ExecuteSQLResponse>('/query', { sql });
  }

  /**
   * 导出数据库备份（SQLite .db / PostgreSQL .sql）
   */
  static async exportDatabase(): Promise<{ blob: Blob; filename: string }> {
    const response = await apiClient.get<Blob>('/api/v1/admin/db-export', {
      withCredentials: true,
      responseType: 'blob',
    } as InternalAxiosRequestConfig);

    const disposition = response.headers['content-disposition'] as string | undefined;
    let filename = 'openflare_export';
    if (disposition) {
      const match = disposition.match(/filename="?([^";]+)"?/);
      if (match) filename = match[1];
    }
    return { blob: response.data, filename };
  }
}
