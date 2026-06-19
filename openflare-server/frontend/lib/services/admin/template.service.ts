import {BaseService} from '@/lib/services/core';
import type {CreateTemplateRequest, Template, UpdateTemplateRequest} from './types';

export class AdminTemplateService extends BaseService {
  protected static readonly basePath = '/api/v1/admin';

  static async listTemplates(): Promise<Template[]> {
    return this.get<Template[]>('/templates');
  }

  static async getTemplate(key: string): Promise<Template> {
    return this.get<Template>(`/templates/${key}`);
  }

  static async createTemplate(request: CreateTemplateRequest): Promise<Template> {
    return this.post<Template>('/templates', request);
  }

  static async updateTemplate(key: string, request: UpdateTemplateRequest): Promise<Template> {
    return this.put<Template>(`/templates/${key}`, request);
  }

  static async deleteTemplate(key: string): Promise<void> {
    return this.delete<void>(`/templates/${key}`);
  }
}