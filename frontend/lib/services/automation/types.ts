export interface MCPSettings {
  enabled: boolean;
  configuration_error?: 'session_secret_too_short';
  client_name?: string;
  mcp_url?: string;
  instance_id?: string;
  scopes?: string[];
}

export interface MCPAccessToken {
  access_token: string;
  expires_at: string;
}

export interface ConsentDetails {
  client_name: string;
  redirect_uri: string;
  scopes: string[];
}
