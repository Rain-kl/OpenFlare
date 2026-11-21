import { BaseService } from '@/lib/services/core';
import type { MCPSettings } from './types';

export class AutomationService extends BaseService {
  protected static readonly basePath = '/api/v1/admin/automation';

  static getMCPSettings() {
    return this.get<MCPSettings>('/mcp');
  }

  static revokeGrants() {
    return this.delete<void>('/grants');
  }
}
