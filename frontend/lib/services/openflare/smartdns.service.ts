import { OpenFlareBaseService } from './base.service';
import type {
  SmartDNSDelegationApplyResult,
  SmartDNSDelegationVerifyResult,
  SmartDNSDiscoveryToken,
  SmartDNSInstance,
  SmartDNSInstanceInput,
  SmartDNSOverview,
  SmartDNSOverviewWindow,
  SmartDNSPolicy,
  SmartDNSPolicyInput,
  SmartDNSPolicyValidateInput,
  SmartDNSPolicyValidateResult,
  SmartDNSQuarantineInput,
  SmartDNSSettings,
  SmartDNSSettingsInput,
  SmartDNSUpdateChannel,
  SmartDNSSimulateInput,
  SmartDNSSimulateResult,
} from './types';

export const smartdnsQueryKey = ['openflare', 'smartdns'] as const;

export class SmartDNSService extends OpenFlareBaseService {
  protected static override readonly basePath: string = '/api/v1/d/smartdns';

  static getOverview(
    window: SmartDNSOverviewWindow = '24h',
  ): Promise<SmartDNSOverview> {
    return this.get<SmartDNSOverview>('/overview', { window });
  }

  static getSettings(): Promise<SmartDNSSettings> {
    return this.get<SmartDNSSettings>('/settings');
  }

  static saveSettings(
    payload: SmartDNSSettingsInput,
  ): Promise<SmartDNSSettings> {
    return this.post<SmartDNSSettings>('/settings', payload);
  }

  static applyDelegation(
    domain: string,
    hosts: string[],
  ): Promise<SmartDNSDelegationApplyResult> {
    return this.post<SmartDNSDelegationApplyResult>(
      '/settings/apply-delegation',
      {
        delegation_domain: domain,
        ns_hosts: hosts,
      },
    );
  }

  static verifyDelegation(): Promise<SmartDNSDelegationVerifyResult> {
    return this.post<SmartDNSDelegationVerifyResult>(
      '/settings/verify-delegation',
    );
  }

  static getDiscoveryToken(): Promise<SmartDNSDiscoveryToken> {
    return this.get<SmartDNSDiscoveryToken>('/discovery-token');
  }

  static rotateDiscoveryToken(): Promise<SmartDNSDiscoveryToken> {
    return this.post<SmartDNSDiscoveryToken>('/discovery-token/rotate');
  }

  static listInstances(): Promise<SmartDNSInstance[]> {
    return this.get<SmartDNSInstance[]>('/instances');
  }

  static createInstance(
    payload: SmartDNSInstanceInput,
  ): Promise<SmartDNSInstance> {
    return this.post<SmartDNSInstance>('/instances', payload);
  }

  static updateInstance(
    id: number,
    payload: SmartDNSInstanceInput,
  ): Promise<SmartDNSInstance> {
    return this.post<SmartDNSInstance>(`/instances/${id}/update`, payload);
  }

  static requestInstanceUpdates(
    instanceIds: number[],
    channel: SmartDNSUpdateChannel,
  ): Promise<{ queued: number }> {
    return this.post('/instances/remote-update', {
      instance_ids: instanceIds,
      channel,
    });
  }

  static deleteInstance(id: number): Promise<void> {
    return this.post<void>(`/instances/${id}/delete`);
  }

  static quarantineInstance(
    id: number,
    payload: SmartDNSQuarantineInput,
  ): Promise<SmartDNSInstance> {
    return this.post<SmartDNSInstance>(`/instances/${id}/quarantine`, payload);
  }

  static unquarantineInstance(id: number): Promise<SmartDNSInstance> {
    return this.post<SmartDNSInstance>(`/instances/${id}/unquarantine`);
  }

  static rotateInstanceToken(id: number): Promise<SmartDNSInstance> {
    return this.post<SmartDNSInstance>(`/instances/${id}/rotate-token`);
  }

  static listPolicies(): Promise<SmartDNSPolicy[]> {
    return this.get<SmartDNSPolicy[]>('/policies');
  }

  static getPolicy(id: number): Promise<SmartDNSPolicy> {
    return this.get<SmartDNSPolicy>(`/policies/${id}`);
  }

  static createPolicy(payload: SmartDNSPolicyInput): Promise<SmartDNSPolicy> {
    return this.post<SmartDNSPolicy>('/policies', payload);
  }

  static updatePolicy(
    id: number,
    payload: SmartDNSPolicyInput,
  ): Promise<SmartDNSPolicy> {
    return this.post<SmartDNSPolicy>(`/policies/${id}/update`, payload);
  }

  static deletePolicy(id: number): Promise<void> {
    return this.post<void>(`/policies/${id}/delete`);
  }

  static setDefaultPolicy(id: number): Promise<SmartDNSPolicy> {
    return this.post<SmartDNSPolicy>(`/policies/${id}/set-default`);
  }

  static validatePolicy(
    payload: SmartDNSPolicyValidateInput,
  ): Promise<SmartDNSPolicyValidateResult> {
    return this.post<SmartDNSPolicyValidateResult>(
      '/policies/validate',
      payload,
    );
  }

  static simulate(
    payload: SmartDNSSimulateInput,
  ): Promise<SmartDNSSimulateResult> {
    return this.post<SmartDNSSimulateResult>('/simulate', payload);
  }
}
