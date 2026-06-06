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

export interface ApplyLogList {
  rows: ApplyLogItem[];
  current: number;
  total: number;
  totalPage: number;
}

export interface ApplyLogListQuery {
  node_id?: string;
  pageNo?: number;
  pageSize?: number;
}

export interface ApplyLogCleanupPayload {
  delete_all?: boolean;
  retention_days?: number;
}

export interface ApplyLogCleanupResult {
  delete_all: boolean;
  retention_days: number;
  deleted_count: number;
  cutoff?: string;
}
