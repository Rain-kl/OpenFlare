import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { AxiosResponse } from 'axios';

import apiClient from '@/lib/services/core/api-client';
import { apiConfig } from '@/lib/services/core/config';
import { PagesService } from '@/lib/services/openflare/pages.service';

vi.mock('@/lib/services/core/api-client', () => ({
  default: {
    get: vi.fn(),
    post: vi.fn(),
  },
}));

function response<T>(data: T) {
  return {
    data: { error_msg: '', data },
    status: 200,
    statusText: 'OK',
    headers: {},
    config: { headers: {} },
  } as AxiosResponse;
}

describe('PagesService', () => {
  beforeEach(() => {
    vi.mocked(apiClient.get).mockReset();
    vi.mocked(apiClient.post).mockReset();
  });

  it('requests deployment files using the backend deployment route', async () => {
    vi.mocked(apiClient.get).mockResolvedValue(response([]));

    await PagesService.listDeploymentFiles(7);

    expect(apiClient.get).toHaveBeenCalledWith(
      '/api/v1/d/pages/deployments/7/files',
      expect.objectContaining({ params: undefined }),
    );
  });

  it('connects all source endpoints with explicit action payloads', async () => {
    vi.mocked(apiClient.get).mockResolvedValue(
      response({ source_type: 'manual' }),
    );
    vi.mocked(apiClient.post).mockResolvedValue(
      response({ source_type: 'manual' }),
    );

    await PagesService.getSource(12);
    await PagesService.updateSource(12, {
      source_type: 'remote_url',
            remote_url: 'https://example.com/site.zip?token=secret',
      allow_insecure: false,
    });
    await PagesService.deleteSource(12);
    await PagesService.checkSource(12);
    await PagesService.syncSource(12);

    expect(apiClient.get).toHaveBeenCalledWith(
      '/api/v1/d/pages/12/source',
      expect.objectContaining({ params: undefined }),
    );
    expect(apiClient.post).toHaveBeenNthCalledWith(
      1,
      '/api/v1/d/pages/12/source/update',
      expect.objectContaining({
        source_type: 'remote_url',
              }),
      undefined,
    );
    expect(apiClient.post).toHaveBeenNthCalledWith(
      2,
      '/api/v1/d/pages/12/source/delete',
      undefined,
      undefined,
    );
    expect(apiClient.post).toHaveBeenNthCalledWith(
      3,
      '/api/v1/d/pages/12/source/check',
      {},
      undefined,
    );
    expect(apiClient.post).toHaveBeenNthCalledWith(
      4,
      '/api/v1/d/pages/12/source/sync',
      {},
      undefined,
    );
  });

  it('sends the complete GitHub latest discriminator payload', async () => {
    vi.mocked(apiClient.post).mockResolvedValue(
      response({
        source: { source_type: 'manual' },
        check_task: null,
        warning: '',
      }),
    );

    await PagesService.updateSource(12, {
      source_type: 'github_release',
      repository_url: 'https://github.com/openflare/site',
      release_selector: 'latest',
      release_tag: '',
      asset_name: 'dist.zip',
      auto_update_enabled: false,
      check_interval_minutes: 1440,
    });

    expect(apiClient.post).toHaveBeenCalledWith(
      '/api/v1/d/pages/12/source/update',
      {
        source_type: 'github_release',
        repository_url: 'https://github.com/openflare/site',
        release_selector: 'latest',
        release_tag: '',
        asset_name: 'dist.zip',
        auto_update_enabled: false,
        check_interval_minutes: 1440,
      },
      undefined,
    );
  });

  it('sends safe disabled defaults with the GitHub tag discriminator', async () => {
    vi.mocked(apiClient.post).mockResolvedValue(
      response({
        source: { source_type: 'manual' },
        check_task: null,
        warning: '',
      }),
    );

    await PagesService.updateSource(12, {
      source_type: 'github_release',
      repository_url: 'https://github.com/openflare/site',
      release_selector: 'tag',
      release_tag: 'v1.2.3',
      asset_name: 'site.tar.gz',
      auto_update_enabled: false,
      check_interval_minutes: 0,
    });

    expect(apiClient.post).toHaveBeenCalledWith(
      '/api/v1/d/pages/12/source/update',
      {
        source_type: 'github_release',
        repository_url: 'https://github.com/openflare/site',
        release_selector: 'tag',
        release_tag: 'v1.2.3',
        asset_name: 'site.tar.gz',
        auto_update_enabled: false,
        check_interval_minutes: 0,
      },
      undefined,
    );
  });

  it('uploads only the package multipart field', async () => {
    vi.mocked(apiClient.post).mockResolvedValue(response({}));
    const file = new File(['site'], 'site.zip', {
      type: 'application/zip',
    });

    await PagesService.uploadDeployment(8, { file });

    const formData = vi.mocked(apiClient.post).mock.calls[0]?.[1];
    expect(formData).toBeInstanceOf(FormData);
    expect(Array.from((formData as FormData).keys())).toEqual(['package']);
    expect(apiClient.post).toHaveBeenCalledWith(
      '/api/v1/d/pages/8/deployments/upload',
      formData,
      expect.objectContaining({ timeout: apiConfig.uploadTimeout }),
    );
  });

  it('keeps the compatibility URL import on the long upload timeout', async () => {
    vi.mocked(apiClient.post).mockResolvedValue(response({}));

    await PagesService.uploadDeploymentFromURL(8, {
      url: 'https://example.com/site.zip',
    });

    expect(apiClient.post).toHaveBeenCalledWith(
      '/api/v1/d/pages/8/deployments/upload-from-url',
      { url: 'https://example.com/site.zip' },
      expect.objectContaining({ timeout: apiConfig.uploadTimeout }),
    );
  });
});
