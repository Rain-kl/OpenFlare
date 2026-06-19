import {OpenFlareBaseService} from './base.service';
import type {
  AcmeAccountItem,
  TlsCertificateApplyPayload,
  TlsCertificateContentItem,
  TlsCertificateDetailItem,
  TlsCertificateFileImportPayload,
  TlsCertificateItem,
  TlsCertificateMutationPayload,
} from './types';

class AcmeAccountApi extends OpenFlareBaseService {
  protected static override readonly basePath: string = '/api/v1/d/acme-accounts';

  static getDefault(): Promise<AcmeAccountItem> {
    return this.get<AcmeAccountItem>('/default');
  }
}

export class TlsCertificateService extends OpenFlareBaseService {
  protected static override readonly basePath: string = '/api/v1/d/tls-certificates';

  static async list(): Promise<TlsCertificateItem[]> {
    return this.get<TlsCertificateItem[]>('/');
  }

  static async getById(id: number): Promise<TlsCertificateDetailItem> {
    return this.get<TlsCertificateDetailItem>(`/${id}`);
  }

  static async getContent(id: number): Promise<TlsCertificateContentItem> {
    return this.get<TlsCertificateContentItem>(`/${id}/content`);
  }

  static async create(
    payload: TlsCertificateMutationPayload,
  ): Promise<TlsCertificateItem> {
    return this.post<TlsCertificateItem>('/', payload);
  }

  static async update(
    id: number,
    payload: TlsCertificateMutationPayload,
  ): Promise<TlsCertificateItem> {
    return this.post<TlsCertificateItem>(`/${id}/update`, payload);
  }

  static async deleteById(id: number): Promise<void> {
    return this.post<void>(`/${id}/delete`);
  }

  static async apply(
    payload: TlsCertificateApplyPayload,
  ): Promise<TlsCertificateItem> {
    return this.post<TlsCertificateItem>('/apply', payload);
  }

  static async renew(id: number): Promise<TlsCertificateItem> {
    return this.post<TlsCertificateItem>(`/${id}/renew`);
  }

  static async updateAcme(
    id: number,
    payload: TlsCertificateApplyPayload,
  ): Promise<TlsCertificateItem> {
    return this.post<TlsCertificateItem>(`/${id}/update-acme`, payload);
  }

  static async convertToAcme(
    id: number,
    payload: TlsCertificateApplyPayload,
  ): Promise<TlsCertificateItem> {
    return this.post<TlsCertificateItem>(`/${id}/convert-acme`, payload);
  }

  static async importFile(
    payload: TlsCertificateFileImportPayload,
  ): Promise<TlsCertificateItem> {
    const formData = new FormData();
    formData.append('name', payload.name);
    formData.append('remark', payload.remark);
    formData.append('cert_file', payload.certFile);
    formData.append('key_file', payload.keyFile);

    return this.post<TlsCertificateItem>('/import-file', formData);
  }

  static getDefaultAcmeAccount(): Promise<AcmeAccountItem> {
    return AcmeAccountApi.getDefault();
  }
}