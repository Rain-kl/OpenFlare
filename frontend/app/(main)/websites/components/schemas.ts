import { z } from 'zod';

import type { TranslateFn } from './website-utils';

export function createManualImportSchema(t: TranslateFn) {
  return z.object({
    name: z
      .string()
      .trim()
      .min(1, t('validation.nameRequired'))
      .max(255, t('validation.nameTooLong')),
    cert_pem: z.string().trim().min(1, t('validation.certPemRequired')),
    key_pem: z.string().trim().min(1, t('validation.keyPemRequired')),
    remark: z.string().max(255, t('validation.remarkTooLong')),
  });
}

export type ManualImportFormValues = z.infer<
  ReturnType<typeof createManualImportSchema>
>;

export type FileImportFormValues = {
  name: string;
  remark: string;
};

export const defaultManualImportValues: ManualImportFormValues = {
  name: '',
  cert_pem: '',
  key_pem: '',
  remark: '',
};

export const defaultFileImportValues: FileImportFormValues = {
  name: '',
  remark: '',
};

export function createAcmeApplySchema(t: TranslateFn) {
  return z.object({
    name: z.string().trim().min(1, t('validation.nameRequired')).max(255),
    primary_domain: z
      .string()
      .trim()
      .min(1, t('validation.primaryDomainRequired')),
    other_domains: z.string(),
    dns_account_id: z.number().min(1, t('validation.dnsAccountRequired')),
    acme_account_id: z.number(),
    key_algorithm: z.string(),
    auto_renew: z.boolean(),
    disable_cname: z.boolean(),
    skip_dns: z.boolean(),
    dns1: z.string(),
    dns2: z.string(),
    remark: z.string().max(255),
  });
}

export type AcmeApplyFormValues = z.infer<
  ReturnType<typeof createAcmeApplySchema>
>;

export const defaultAcmeApplyValues: AcmeApplyFormValues = {
  name: '',
  primary_domain: '',
  other_domains: '',
  dns_account_id: 0,
  acme_account_id: 0,
  key_algorithm: 'RSA2048',
  auto_renew: true,
  disable_cname: false,
  skip_dns: false,
  dns1: '',
  dns2: '',
  remark: '',
};
