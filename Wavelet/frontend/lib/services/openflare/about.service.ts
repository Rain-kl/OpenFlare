import {OpenFlareBaseService} from './base.service';

export class AboutService extends OpenFlareBaseService {
  static getAboutContent(): Promise<string> {
    return this.get<string>('/about');
  }
}