export function formatBytes(bytes: number, decimals = 2) {
  if (bytes === 0) return '0 B';
  const k = 1024;
  const dm = decimals < 0 ? 0 : decimals;
  const sizes = ['B', 'KB', 'MB', 'GB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return `${parseFloat((bytes / Math.pow(k, i)).toFixed(dm))} ${sizes[i]}`;
}

export const projectsQueryKey = ['openflare', 'pages', 'projects'] as const;

export function projectQueryKey(projectId: string | number) {
  return ['openflare', 'pages', 'project', String(projectId)] as const;
}

export function deploymentsQueryKey(projectId: number) {
  return ['openflare', 'pages', 'deployments', projectId] as const;
}

export function deploymentFilesQueryKey(projectId: number, deploymentId: number) {
  return ['openflare', 'pages', 'deployment-files', projectId, deploymentId] as const;
}