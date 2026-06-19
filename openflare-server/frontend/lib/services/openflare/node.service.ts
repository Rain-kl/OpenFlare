import {OpenFlareBaseService} from './base.service';
import type {
  NodeAgentReleaseInfo,
  NodeAgentUpdatePayload,
  NodeBootstrapToken,
  NodeItem,
  NodeMutationPayload,
  NodeObservability,
  ReleaseChannel,
} from './types';

export class NodeService extends OpenFlareBaseService {
  protected static override readonly basePath: string = '/api/v1/d/nodes';

  static async listNodes(): Promise<NodeItem[]> {
    return this.get<NodeItem[]>('/');
  }

  static async createNode(payload: NodeMutationPayload): Promise<NodeItem> {
    return this.post<NodeItem>('/', payload);
  }

  static async updateNode(id: number, payload: NodeMutationPayload): Promise<NodeItem> {
    return this.post<NodeItem>(`/${id}/update`, payload);
  }

  static async deleteNode(id: number): Promise<void> {
    return this.post<void>(`/${id}/delete`);
  }

  static async getBootstrapToken(): Promise<NodeBootstrapToken> {
    return this.get<NodeBootstrapToken>('/bootstrap-token');
  }

  static async rotateBootstrapToken(): Promise<NodeBootstrapToken> {
    return this.post<NodeBootstrapToken>('/bootstrap-token/rotate');
  }

  static async requestAgentUpdate(
    id: number,
    payload?: NodeAgentUpdatePayload,
  ): Promise<NodeItem> {
    return this.post<NodeItem>(`/${id}/agent-update`, payload ?? {});
  }

  static async requestForceSync(id: number): Promise<NodeItem> {
    return this.post<NodeItem>(`/${id}/force-sync`);
  }

  static async requestOpenrestyRestart(id: number): Promise<NodeItem> {
    return this.post<NodeItem>(`/${id}/openresty-restart`);
  }

  static async getAgentRelease(
    id: number,
    channel: ReleaseChannel = 'stable',
  ): Promise<NodeAgentReleaseInfo> {
    return this.get<NodeAgentReleaseInfo>(`/${id}/agent-release`, { channel });
  }

  static async getObservability(
    id: number,
    options?: { hours?: number; limit?: number },
  ): Promise<NodeObservability> {
    return this.get<NodeObservability>(`/${id}/observability`, {
      hours: options?.hours,
      limit: options?.limit,
    });
  }

  static async cleanupHealthEvents(
    id: number,
  ): Promise<{ node_id: string; deleted_count: number }> {
    return this.post<{ node_id: string; deleted_count: number }>(
      `/${id}/observability/cleanup`,
    );
  }
}