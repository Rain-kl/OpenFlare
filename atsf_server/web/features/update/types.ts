export interface LatestReleaseInfo {
  tag_name: string;
  body: string;
  html_url: string;
  published_at: string;
  current_version: string;
  has_update: boolean;
  upgrade_supported: boolean;
  in_progress: boolean;
}
