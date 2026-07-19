'use client';

import { type PagesProject } from '@/lib/services/openflare';

import { DangerZoneCard } from '../components/danger-zone-card';
import { PagesSourceCard } from '../components/pages-source-card';
import { ProjectSettingsCard } from '../components/project-settings-card';

interface SettingsTabProps {
  project: PagesProject;
}

export function SettingsTab({ project }: SettingsTabProps) {
  return (
    <div className='flex flex-col gap-6'>
      <ProjectSettingsCard project={project} />
      <PagesSourceCard key={`source-${project.id}`} projectId={project.id} />
      <DangerZoneCard project={project} />
    </div>
  );
}
