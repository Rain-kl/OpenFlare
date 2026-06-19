import {BaseService} from '@/lib/services/core';
import type {AppUpdateStatus, SystemStatus} from './types';

export class AdminStatusService extends BaseService {
  protected static readonly basePath = '/api/v1/admin';

  static async getSystemStatus(): Promise<SystemStatus> {
    return this.get<SystemStatus>('/status');
  }

  static async getUpdateStatus(): Promise<AppUpdateStatus> {
    return this.get<AppUpdateStatus>('/update');
  }

  static async applyUpdate(): Promise<void> {
    return this.post<void>('/update/apply');
  }
}