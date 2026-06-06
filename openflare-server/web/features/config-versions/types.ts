export interface ConfigVersionSummary {
  id: number;
  version: string;
  checksum: string;
  is_active: boolean;
  created_by: string;
  created_at: string;
}

export interface ConfigVersionDetail extends ConfigVersionSummary {
  snapshot_json: string;
  main_config: string;
  rendered_config: string;
  support_files_json: string;
}

export interface SupportFile {
  path: string;
  content: string;
}

export interface ConfigPreviewResult {
  snapshot_json: string;
  main_config: string;
  route_config: string;
  rendered_config: string;
  support_files: SupportFile[];
  checksum: string;
  route_count: number;
  website_count: number;
}

export interface ConfigDiffResult {
  active_version?: string;
  added_sites: string[];
  removed_sites: string[];
  modified_sites: string[];
  added_domains: string[];
  removed_domains: string[];
  modified_domains: string[];
  main_config_changed: boolean;
  waf_config_changed: boolean;
  changed_option_keys: string[];
  changed_option_details: ConfigOptionDiffItem[];
  current_website_count: number;
  active_website_count: number;
}

export interface ConfigOptionDiffItem {
  key: string;
  previous_value: string;
  current_value: string;
}
