import {OpenFlareBaseService} from './base.service';
import type {OpenFlarePublicStatus} from './types';

export class StatusService extends OpenFlareBaseService {
  static getPublicStatus(): Promise<OpenFlarePublicStatus> {
    return this.get<OpenFlarePublicStatus>('/status');
  }
}