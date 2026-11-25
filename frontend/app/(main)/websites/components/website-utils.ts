import { runtimeText } from '@/i18n/runtime';
import type {
  TlsCertificateFileImportPayload,
  TlsCertificateItem,
  TlsCertificateListItem,
  TlsCertificateMutationPayload,
} from '@/lib/services/openflare';
import { formatDateTime } from '@/lib/utils';

import type { FileImportFormValues, ManualImportFormValues } from './schemas';

export type StatusTone = 'success' | 'warning' | 'danger' | 'info';

export type TranslateFn = (
  key: string,
  values?: Record<string, string | number>,
) => string;

export function getErrorMessage(error: unknown, fallback?: string) {
  return error instanceof Error
    ? error.message
    : (fallback ??
        runtimeText('request_failed_please_try_again_later_8977b79a'));
}

export function getMatchTypeMeta(domain: string): {
  label: string;
  tone: StatusTone;
} {
  return domain.startsWith('*.')
    ? { label: runtimeText('wildcard_c6b0739e'), tone: 'warning' }
    : { label: runtimeText('exact_match_391fe725'), tone: 'info' };
}

export function getCertificateStatus(
  certificate: TlsCertificateItem,
  t: TranslateFn,
): {
  label: string;
  tone: StatusTone;
} {
  const expiresAt = new Date(certificate.not_after).getTime();
  const diffMs = expiresAt - Date.now();
  const days = Math.ceil(diffMs / (1000 * 60 * 60 * 24));

  if (Number.isNaN(expiresAt)) {
    return { label: t('certStatus.unknownExpiry'), tone: 'warning' };
  }

  if (days < 0) {
    return { label: t('certStatus.expired'), tone: 'danger' };
  }

  if (days <= 30) {
    return { label: t('certStatus.expiresInDays', { days }), tone: 'warning' };
  }

  return { label: t('certStatus.valid'), tone: 'success' };
}

export function buildCertificateLabel(certificate: TlsCertificateItem) {
  return certificate.not_after
    ? runtimeText('value0_expires_value1_7d8600f8', {
        value0: certificate.name,
        value1: formatDateTime(certificate.not_after),
      })
    : certificate.name;
}

export function getCertificateMatch(
  certificate: TlsCertificateListItem,
  domain: string,
) {
  const names = (certificate.dns_names ?? []).map((name) => name.toLowerCase());
  if (names.includes(domain)) {
    return 'exact';
  }
  // A wildcard covers one label. A wildcard domain needs the same wildcard SAN.
  if (names.includes(domain.replace(/^[^.]+\./, '*.'))) {
    return 'wildcard';
  }
}

export function findMatchingCertificate(
  certificates: TlsCertificateListItem[],
  domain: string,
) {
  if (!domain) return;

  const now = Date.now();
  const validCertificates = certificates
    .filter(
      (certificate) =>
        Date.parse(certificate.not_before) <= now &&
        now < Date.parse(certificate.not_after),
    )
    .sort((a, b) => Date.parse(b.not_after) - Date.parse(a.not_after));
  return (
    validCertificates.find(
      (certificate) => getCertificateMatch(certificate, domain) === 'exact',
    ) ??
    validCertificates.find(
      (certificate) => getCertificateMatch(certificate, domain) === 'wildcard',
    )
  );
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
  missingFilesMessage: string,
): TlsCertificateFileImportPayload {
  if (!certFile || !keyFile) {
    throw new Error(missingFilesMessage);
  }

  return {
    name: values.name.trim(),
    remark: values.remark.trim(),
    certFile,
    keyFile,
  };
}
