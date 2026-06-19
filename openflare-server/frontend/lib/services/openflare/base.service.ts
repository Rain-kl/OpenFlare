import {BaseService} from '@/lib/services/core';

export class OpenFlareBaseService extends BaseService {
  protected static override readonly basePath: string = '/api/v1/d';
}