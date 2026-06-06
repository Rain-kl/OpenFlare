export interface ManagedDomainItem {
  id: number;
  domain: string;
  cert_id: number | null;
  enabled: boolean;
  remark: string;
  created_at: string;
  updated_at: string;
}

export interface ManagedDomainMutationPayload {
  domain: string;
  cert_id: number | null;
  enabled: boolean;
  remark: string;
}

export interface ManagedDomainCertificateOption {
  id: number;
  name: string;
  not_after: string | null;
}
