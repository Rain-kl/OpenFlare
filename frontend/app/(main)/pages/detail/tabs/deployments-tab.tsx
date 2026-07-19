'use client';

import { type PagesProject } from '@/lib/services/openflare';

import { DeploymentHistory } from '../components/deployment-history';

interface DeploymentsTabProps {
  project: PagesProject;
}

export function DeploymentsTab({ project }: DeploymentsTabProps) {
  return (
    <DeploymentHistory
      key={`deployments-${project.id}`}
      projectId={project.id}
      activeDeploymentId={project.active_deployment_id}
      rootDir={project.root_dir ?? ''}
      entryFile={project.entry_file}
    />
  );
}
