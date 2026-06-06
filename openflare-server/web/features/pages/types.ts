export interface PagesDeployment {
  id: number;
  project_id: number;
  deployment_number: number;
  checksum: string;
  status: 'uploaded' | 'active';
  file_count: number;
  total_size: number;
  root_dir?: string;
  entry_file: string;
  created_by: string;
  created_at: string;
  activated_at?: string | null;
}

export interface PagesProject {
  id: number;
  name: string;
  slug: string;
  description: string;
  enabled: boolean;
  spa_fallback_enabled: boolean;
  spa_fallback_path: string;
  api_proxy_enabled: boolean;
  api_proxy_path: string;
  api_proxy_pass: string;
  api_proxy_rewrite: string;
  root_dir?: string;
  entry_file: string;
  active_deployment_id?: number | null;
  active_deployment?: PagesDeployment | null;
  deployment_count: number;
  created_at: string;
  updated_at: string;
}

export interface PagesProjectPayload {
  name: string;
  slug: string;
  description: string;
  enabled: boolean;
  spa_fallback_enabled: boolean;
  spa_fallback_path: string;
  api_proxy_enabled: boolean;
  api_proxy_path: string;
  api_proxy_pass: string;
  api_proxy_rewrite: string;
  root_dir?: string;
  entry_file: string;
}
