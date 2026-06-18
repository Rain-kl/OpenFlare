import type {InternalAxiosRequestConfig} from 'axios';

import {getApiBaseUrl} from '@/lib/services/core/config';
import {ApiErrorBase} from '@/lib/services/core/errors';
import type {ApiResponse} from '@/lib/services/core';

import {OpenFlareBaseService} from './base.service';
import type {LatestReleaseInfo, ReleaseChannel, UpgradeStreamSnapshot, UploadedServerBinaryInfo} from './types';

/**
 * OpenFlare 服务端升级 API（`/api/v1/custom/openflare/update/*`）。
 */
export class UpdateService extends OpenFlareBaseService {
  protected static override readonly basePath: string = '/api/v1/custom/openflare/update';

  private static getApiUrl(path: string): string {
    const baseURL = getApiBaseUrl();
    const fullPath = this.getFullPath(path);
    if (!baseURL) {
      return fullPath;
    }
    return `${baseURL.replace(/\/$/, '')}${fullPath}`;
  }

  static getLatestRelease(channel: ReleaseChannel = 'stable'): Promise<LatestReleaseInfo> {
    return this.get<LatestReleaseInfo>('/latest-release', { channel });
  }

  static upgradeServer(channel: ReleaseChannel = 'stable'): Promise<LatestReleaseInfo> {
    return this.post<LatestReleaseInfo>('/upgrade', { channel });
  }

  static uploadServerBinary(
    binary: File,
    onProgress?: (progress: number) => void,
  ): Promise<UploadedServerBinaryInfo> {
    const formData = new FormData();
    formData.append('binary', binary);

    if (!onProgress) {
      return this.post<UploadedServerBinaryInfo>('/manual-upload', formData, {
        headers: { 'Content-Type': 'multipart/form-data' },
      } as InternalAxiosRequestConfig);
    }

    return new Promise((resolve, reject) => {
      const xhr = new XMLHttpRequest();
      xhr.open('POST', this.getApiUrl('/manual-upload'));
      xhr.withCredentials = true;

      xhr.upload.addEventListener('progress', (event) => {
        if (event.lengthComputable) {
          onProgress(Math.round((event.loaded / event.total) * 100));
        }
      });

      xhr.addEventListener('load', () => {
        let payload: ApiResponse<UploadedServerBinaryInfo> | null = null;
        try {
          payload = JSON.parse(
            xhr.responseText,
          ) as ApiResponse<UploadedServerBinaryInfo>;
        } catch {
          payload = null;
        }
        if (xhr.status < 200 || xhr.status >= 300) {
          reject(
            new ApiErrorBase(
              payload?.message || `请求失败（${xhr.status}）`,
              undefined,
              xhr.status,
            ),
          );
          return;
        }
        if (!payload) {
          reject(new ApiErrorBase('响应格式无效', undefined, xhr.status));
          return;
        }
        resolve(payload.data);
      });

      xhr.addEventListener('error', () => {
        reject(new ApiErrorBase('上传过程中网络连接中断，请检查网络后重试', undefined, 0));
      });

      xhr.send(formData);
    });
  }

  static confirmManualServerUpgrade(uploadToken: string): Promise<UploadedServerBinaryInfo> {
    return this.post<UploadedServerBinaryInfo>('/manual-upgrade', {
      upload_token: uploadToken,
    });
  }

  static createUpgradeLogsWebSocket(): WebSocket | null {
    if (typeof window === 'undefined') {
      return null;
    }

    const apiUrl = this.getApiUrl('/logs/ws');
    const resolvedUrl = apiUrl.startsWith('http://')
      ? `ws://${apiUrl.slice('http://'.length)}`
      : apiUrl.startsWith('https://')
        ? `wss://${apiUrl.slice('https://'.length)}`
        : `${window.location.protocol === 'https:' ? 'wss:' : 'ws:'}//${window.location.host}${apiUrl}`;

    return new WebSocket(resolvedUrl);
  }

  static parseUpgradeStreamSnapshot(rawMessage: string): UpgradeStreamSnapshot | null {
    try {
      const parsed = JSON.parse(rawMessage) as Partial<UpgradeStreamSnapshot>;
      if (
        typeof parsed.in_progress !== 'boolean' ||
        typeof parsed.upgrade_status !== 'string' ||
        !Array.isArray(parsed.upgrade_logs)
      ) {
        return null;
      }
      return {
        in_progress: parsed.in_progress,
        upgrade_status: parsed.upgrade_status,
        upgrade_logs: parsed.upgrade_logs,
      };
    } catch {
      return null;
    }
  }
}