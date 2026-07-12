import type {
  TlsCertificateFileImportPayload,
  TlsCertificateItem,
  TlsCertificateMutationPayload,
} from '@/lib/services/openflare';
import {formatDateTime} from '@/lib/utils';

import type {FileImportFormValues, ManualImportFormValues} from './schemas';

export type StatusTone = 'success' | 'warning' | 'danger' | 'info';

export function getErrorMessage(error: unknown) {
  return error instanceof Error ? error.message : '请求失败，请稍后重试。';
}

export function getMatchTypeMeta(domain: string): {label: string; tone: StatusTone} {
  return domain.startsWith('*.')
    ? {label: '通配符', tone: 'warning'}
    : {label: '精确匹配', tone: 'info'};
}

export function getCertificateStatus(certificate: TlsCertificateItem): {
  label: string;
  tone: StatusTone;
} {
  const expiresAt = new Date(certificate.not_after).getTime();
  const diffMs = expiresAt - Date.now();
  const days = Math.ceil(diffMs / (1000 * 60 * 60 * 24));

  if (Number.isNaN(expiresAt)) {
    return {label: '有效期未知', tone: 'warning'};
  }

  if (days < 0) {
    return {label: '已过期', tone: 'danger'};
  }

  if (days <= 30) {
    return {label: `${days} 天内到期`, tone: 'warning'};
  }

  return {label: '有效', tone: 'success'};
}

export function buildCertificateLabel(certificate: TlsCertificateItem) {
  return certificate.not_after
    ? `${certificate.name}（到期：${formatDateTime(certificate.not_after)}）`
    : certificate.name;
}

export function toManualPayload(
  values: ManualImportFormValues,
): TlsCertificateMutationPayload {
  return {
    name: values.name.trim(),
    cert_pem: values.cert_pem.trim(),
    key_pem: values.key_pem.trim(),
    remark: values.remark.trim(),
  };
}

export function toFilePayload(
  values: FileImportFormValues,
  certFile: File | null,
  keyFile: File | null,
): TlsCertificateFileImportPayload {
  if (!certFile || !keyFile) {
    throw new Error('请选择证书文件和私钥文件。');
  }

  return {
    name: values.name.trim(),
    remark: values.remark.trim(),
    certFile,
    keyFile,
  };
}
