export interface PublicAuthSource {
  id: number;
  name: string;
  type: 'github' | 'oidc';
  display_name: string;
  authorize_url: string;
  icon_url?: string;
}

export interface PublicStatus {
  version: string;
  start_time: number;
  email_verification: boolean;
  github_oauth: boolean;
  github_client_id: string;
  system_name: string;
  home_page_link: string;
  footer_html: string;
  wechat_qrcode: string;
  wechat_login: boolean;
  server_address: string;
  password_register_enabled: boolean;
  auth_sources: PublicAuthSource[];
}
