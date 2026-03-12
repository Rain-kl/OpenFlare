export interface ApplyLogItem {
  id: number;
  node_id: string;
  version: string;
  result: 'success' | 'failed' | string;
  message: string;
  checksum: string;
  main_config_checksum: string;
  route_config_checksum: string;
  support_file_count: number;
  created_at: string;
}
