import {OpenFlareBaseService} from './base.service';

export class UptimeKumaService extends OpenFlareBaseService {
  protected static override readonly basePath: string = '/api/v1/custom/openflare/uptimekuma';

  static sync(): Promise<void> {
    return this.post<void>('/sync');
  }
}