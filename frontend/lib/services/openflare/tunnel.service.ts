import { OpenFlareBaseService } from './base.service';
import type {
  ReleaseChannel,
  TunnelBinaryUpdateInput,
  TunnelDeployView,
  TunnelDetailView,
  TunnelInput,
  TunnelPoolInput,
  TunnelPoolView,
  TunnelRelayBootstrapView,
  TunnelRelayDeployView,
  TunnelRelayDrainInput,
  TunnelRelayInput,
  TunnelRelayView,
  TunnelReleaseInfo,
  TunnelServiceInput,
  TunnelServiceView,
  TunnelTopologyView,
  TunnelView,
} from './types';

export const tunnelQueryKey = ['openflare', 'tunnels'] as const;

export class TunnelService extends OpenFlareBaseService {
  protected static override readonly basePath: string = '/api/v1/d';

  // ==================== Pools ====================

  static async listPools(): Promise<TunnelPoolView[]> {
    return this.get<TunnelPoolView[]>('/tunnel-pools');
  }

  static async getPool(id: number): Promise<TunnelPoolView> {
    return this.get<TunnelPoolView>(`/tunnel-pools/${id}`);
  }

  static async createPool(payload: TunnelPoolInput): Promise<TunnelPoolView> {
    return this.post<TunnelPoolView>('/tunnel-pools', payload);
  }

  static async updatePool(
    id: number,
    payload: TunnelPoolInput,
  ): Promise<TunnelPoolView> {
    return this.post<TunnelPoolView>(`/tunnel-pools/${id}/update`, payload);
  }

  static async deletePool(id: number): Promise<void> {
    return this.post<void>(`/tunnel-pools/${id}/delete`);
  }

  // ==================== Relays ====================

  static async listRelays(): Promise<TunnelRelayView[]> {
    return this.get<TunnelRelayView[]>('/tunnel-relays');
  }

  static async getRelay(id: number): Promise<TunnelRelayView> {
    return this.get<TunnelRelayView>(`/tunnel-relays/${id}`);
  }

  static async getRelayDeploy(id: number): Promise<TunnelRelayDeployView> {
    return this.get<TunnelRelayDeployView>(`/tunnel-relays/${id}/deploy`);
  }

  static async createRelay(
    payload: TunnelRelayInput,
  ): Promise<TunnelRelayDeployView> {
    return this.post<TunnelRelayDeployView>('/tunnel-relays', payload);
  }

  static async updateRelay(
    id: number,
    payload: TunnelRelayInput,
  ): Promise<TunnelRelayView> {
    return this.post<TunnelRelayView>(`/tunnel-relays/${id}/update`, payload);
  }

  static async deleteRelay(id: number): Promise<void> {
    return this.post<void>(`/tunnel-relays/${id}/delete`);
  }

  static async rotateRelayToken(id: number): Promise<TunnelRelayDeployView> {
    return this.post<TunnelRelayDeployView>(
      `/tunnel-relays/${id}/rotate-token`,
    );
  }

  static async setRelayDraining(
    id: number,
    payload: TunnelRelayDrainInput,
  ): Promise<TunnelRelayView> {
    return this.post<TunnelRelayView>(`/tunnel-relays/${id}/drain`, payload);
  }

  static async quarantineRelay(
    id: number,
    reason?: string,
  ): Promise<TunnelRelayView> {
    return this.post<TunnelRelayView>(`/tunnel-relays/${id}/quarantine`, {
      reason: reason ?? '',
    });
  }

  static async unquarantineRelay(id: number): Promise<TunnelRelayDeployView> {
    return this.post<TunnelRelayDeployView>(
      `/tunnel-relays/${id}/unquarantine`,
    );
  }

  static async requestRelayBinaryUpdate(
    id: number,
    payload: TunnelBinaryUpdateInput = {},
  ): Promise<TunnelRelayView> {
    return this.post<TunnelRelayView>(
      `/tunnel-relays/${id}/update-binary`,
      payload,
    );
  }

  static async getRelayBootstrapToken(): Promise<TunnelRelayBootstrapView> {
    return this.get<TunnelRelayBootstrapView>('/tunnel-relays/bootstrap-token');
  }

  static async rotateRelayBootstrapToken(): Promise<TunnelRelayBootstrapView> {
    return this.post<TunnelRelayBootstrapView>(
      '/tunnel-relays/bootstrap-token/rotate',
    );
  }

  // ==================== Tunnels ====================

  static async listTunnels(): Promise<TunnelView[]> {
    return this.get<TunnelView[]>('/tunnels');
  }

  static async getTopology(): Promise<TunnelTopologyView> {
    return this.get<TunnelTopologyView>('/tunnels/topology');
  }

  static async getRelease(
    channel: ReleaseChannel = 'stable',
  ): Promise<TunnelReleaseInfo> {
    return this.get<TunnelReleaseInfo>('/tunnels/release', { channel });
  }

  static async getTunnel(id: number): Promise<TunnelDetailView> {
    return this.get<TunnelDetailView>(`/tunnels/${id}`);
  }

  static async getTunnelDeploy(id: number): Promise<TunnelDeployView> {
    return this.get<TunnelDeployView>(`/tunnels/${id}/deploy`);
  }

  static async createTunnel(payload: TunnelInput): Promise<TunnelDeployView> {
    return this.post<TunnelDeployView>('/tunnels', payload);
  }

  static async updateTunnel(
    id: number,
    payload: TunnelInput,
  ): Promise<TunnelView> {
    return this.post<TunnelView>(`/tunnels/${id}/update`, payload);
  }

  static async deleteTunnel(id: number): Promise<void> {
    return this.post<void>(`/tunnels/${id}/delete`);
  }

  static async rotateTunnelToken(id: number): Promise<TunnelDeployView> {
    return this.post<TunnelDeployView>(`/tunnels/${id}/rotate-token`);
  }

  static async quarantineTunnel(
    id: number,
    reason?: string,
  ): Promise<TunnelView> {
    return this.post<TunnelView>(`/tunnels/${id}/quarantine`, {
      reason: reason ?? '',
    });
  }

  static async unquarantineTunnel(id: number): Promise<TunnelDeployView> {
    return this.post<TunnelDeployView>(`/tunnels/${id}/unquarantine`);
  }

  static async requestTunnelBinaryUpdate(
    id: number,
    payload: TunnelBinaryUpdateInput = {},
  ): Promise<TunnelView> {
    return this.post<TunnelView>(`/tunnels/${id}/update-binary`, payload);
  }

  static async revokeConnector(
    tunnelId: number,
    connectorId: string,
  ): Promise<void> {
    return this.post<void>(
      `/tunnels/${tunnelId}/connectors/${encodeURIComponent(connectorId)}/revoke`,
    );
  }

  static async createService(
    tunnelId: number,
    payload: TunnelServiceInput,
  ): Promise<TunnelServiceView> {
    return this.post<TunnelServiceView>(
      `/tunnels/${tunnelId}/services`,
      payload,
    );
  }

  static async updateService(
    tunnelId: number,
    serviceId: number,
    payload: TunnelServiceInput,
  ): Promise<TunnelServiceView> {
    return this.post<TunnelServiceView>(
      `/tunnels/${tunnelId}/services/${serviceId}/update`,
      payload,
    );
  }

  static async deleteService(
    tunnelId: number,
    serviceId: number,
  ): Promise<void> {
    return this.post<void>(`/tunnels/${tunnelId}/services/${serviceId}/delete`);
  }
}
