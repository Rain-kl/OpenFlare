/**
 * 数据库概览信息
 */
export interface DBOverview {
  type: string;
  version: string;
  name: string;
  size: string;
  table_count: number;
  connections: number;
}

/**
 * 动态数据表分页数据响应
 */
export interface TableDataResponse {
  columns: string[];
  total: number;
  results: Record<string, unknown>[];
}

/**
 * 执行自定义 SQL 的响应
 */
export interface ExecuteSQLResponse {
  type: "select" | "exec";
  columns?: string[];
  results?: Record<string, unknown>[];
  affected_rows: number;
  execution_time_ms: number;
}
