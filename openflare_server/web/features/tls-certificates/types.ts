export interface TlsCertificateItem {
  id: number;
  name: string;
  cert_pem?: string;
  key_pem?: string;
  provider: string;
  acme_account_id: number;
  dns_account_id: number;
  key_algorithm: string;
  auto_renew: boolean;
  primary_domain: string;
  other_domains: string;
  disable_cname: boolean;
  skip_dns: boolean;
  dns1: string;
  dns2: string;
  apply_status: string;
  apply_message: string;
  not_before: string;
  not_after: string;
  remark: string;
  created_at: string;
  updated_at: string;
}

export interface TlsCertificateDetailItem extends TlsCertificateItem {
  cert_pem?: never;
  key_pem?: never;
}

export interface TlsCertificateContentItem extends TlsCertificateItem {
  cert_pem: string;
  key_pem: string;
}

export interface TlsCertificateMutationPayload {
  name: string;
  cert_pem: string;
  key_pem: string;
  remark: string;
}

export interface TlsCertificateApplyPayload {
  name: string;
  remark: string;
  acme_account_id: number;
  dns_account_id: number;
  key_algorithm: string;
  auto_renew: boolean;
  primary_domain: string;
  other_domains: string;
  disable_cname: boolean;
  skip_dns: boolean;
  dns1: string;
  dns2: string;
}

export interface TlsCertificateFileImportPayload {
  name: string;
  remark: string;
  certFile: File;
  keyFile: File;
}
