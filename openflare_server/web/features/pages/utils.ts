export const projectsQueryKey = ['pages-projects'];

export function projectQueryKey(projectId: string | number) {
  return ['pages-project', String(projectId)];
}

export function deploymentsQueryKey(projectId: string | number) {
  return ['pages-deployments', Number(projectId)];
}
