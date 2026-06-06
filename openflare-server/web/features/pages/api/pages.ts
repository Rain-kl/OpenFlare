import {ApiError, apiRequest, getApiUrl} from '@/lib/api/client';
import type {ApiEnvelope} from '@/types/api';

import type {PagesDeployment, PagesProject, PagesProjectPayload,} from '@/features/pages/types';

export function getPagesProjects() {
  return apiRequest<PagesProject[]>('/pages/');
}

export function getPagesProject(id: number) {
  return apiRequest<PagesProject>(`/pages/${id}`);
}

export function createPagesProject(payload: PagesProjectPayload) {
  return apiRequest<PagesProject>('/pages/', {
    method: 'POST',
    body: JSON.stringify(payload),
  });
}

export function updatePagesProject(id: number, payload: PagesProjectPayload) {
  return apiRequest<PagesProject>(`/pages/${id}/update`, {
    method: 'POST',
    body: JSON.stringify(payload),
  });
}

export function deletePagesProject(id: number) {
  return apiRequest<void>(`/pages/${id}/delete`, {
    method: 'POST',
  });
}

export function getPagesDeployments(projectId: number) {
  return apiRequest<PagesDeployment[]>(`/pages/${projectId}/deployments`);
}

export function uploadPagesDeployment(
  projectId: number,
  file: File,
  rootDir = '',
  entryFile = 'index.html',
  onProgress?: (percent: number) => void,
) {
  if (!onProgress) {
    const formData = new FormData();
    formData.append('package', file);
    formData.append('root_dir', rootDir);
    formData.append('entry_file', entryFile);
    return apiRequest<PagesDeployment>(
      `/pages/${projectId}/deployments/upload`,
      {
        method: 'POST',
        body: formData,
      },
    );
  }

  return new Promise<PagesDeployment>((resolve, reject) => {
    const formData = new FormData();
    formData.append('package', file);
    formData.append('root_dir', rootDir);
    formData.append('entry_file', entryFile);

    const xhr = new XMLHttpRequest();
    xhr.open('POST', getApiUrl(`/pages/${projectId}/deployments/upload`));
    xhr.withCredentials = true;

    xhr.upload.addEventListener('progress', (event) => {
      if (event.lengthComputable) {
        const percent = Math.round((event.loaded / event.total) * 100);
        onProgress(percent);
      }
    });

    xhr.onload = () => {
      let payload: ApiEnvelope<PagesDeployment> | null = null;
      try {
        payload = JSON.parse(xhr.responseText) as ApiEnvelope<PagesDeployment>;
      } catch {
        payload = null;
      }

      if (xhr.status >= 200 && xhr.status < 300) {
        if (payload && payload.success) {
          resolve(payload.data);
        } else {
          reject(new ApiError(payload?.message || '请求失败', xhr.status));
        }
      } else {
        reject(
          new ApiError(
            payload?.message || `请求失败（${xhr.status}）`,
            xhr.status,
          ),
        );
      }
    };

    xhr.onerror = () => {
      reject(new ApiError('网络请求失败', 0));
    };

    xhr.send(formData);
  });
}

export function activatePagesDeployment(
  projectId: number,
  deploymentId: number,
) {
  return apiRequest<PagesProject>(
    `/pages/${projectId}/deployments/${deploymentId}/activate`,
    { method: 'POST' },
  );
}

export function deletePagesDeployment(projectId: number, deploymentId: number) {
  return apiRequest<void>(
    `/pages/${projectId}/deployments/${deploymentId}/delete`,
    { method: 'POST' },
  );
}
