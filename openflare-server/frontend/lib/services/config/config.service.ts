import {BaseService} from '@/lib/services/core';
import type {PublicConfigResponse} from './types';

/**
 * 配置服务
 * 处理系统公共配置相关的 API 请求
 */
export class ConfigService extends BaseService {
  protected static readonly basePath = '/api/v1/config';

  /**
   * 获取公共配置
   * @returns 公共配置信息
   *
   * @example
   * ```typescript
   * const config = await ConfigService.getPublicConfig();
   * ```
   */
  static async getPublicConfig(): Promise<PublicConfigResponse> {
    return this.get<PublicConfigResponse>('/public');
  }
}
