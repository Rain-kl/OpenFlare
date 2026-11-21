import { BaseService } from '@/lib/services/core';
import type { MCPAccessToken, MCPSettings } from './types';

export class AutomationService extends BaseService {
  protected static readonly basePath = '/api/v1/admin/automation';

  static getMCPSettings() {
    return this.get<MCPSettings>('/mcp');
  }

  static createMCPToken(scopes: string[]) {
    return this.post<MCPAccessToken>('/mcp/token', { scopes });
  }

  static revokeGrants() {
    return this.delete<void>('/grants');
  }
}
