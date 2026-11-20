export const attackCategories = [
  'sql_injection',
  'xss',
  'malicious_upload',
  'path_traversal',
  'file_inclusion',
  'command_injection',
  'ssrf',
  'xxe',
  'crlf_injection',
  'java_injection',
  'java_deserialization',
  'php_injection',
  'php_deserialization',
  'asp_injection',
  'template_injection',
  'csrf',
  'malformed_http',
] as const;

export const attackLevels = ['off', 'observe', 'balanced', 'strict'] as const;
export type AttackCategory = (typeof attackCategories)[number];
export type AttackLevel = (typeof attackLevels)[number];

export type AttackGroupInput = {
  name: string;
  levels: Partial<Record<AttackCategory, AttackLevel>>;
};
export type AttackGroup = AttackGroupInput & { id: number; updated_at: string };

export type AttackLog = {
  id: string;
  node_id: string;
  logged_at: string;
  remote_addr: string;
  region: string;
  host: string;
  path: string;
  status_code: number;
  action: 'blocked' | 'observed';
  reason: string;
  method: string;
  https: boolean;
  flow_id: number;
  flow_name: string;
  flow_node_id: string;
  attack_group_id?: number;
  attack_group_name?: string;
  rules?: {
    id: number;
    category: AttackCategory;
    message: string;
    level: AttackLevel;
  }[];
};

export type AttackEvent = {
  remote_addr: string;
  host: string;
  bucket_unix: number;
  first_unix: number;
  last_unix: number;
  count: number;
};

export type AttackLogFilter = {
  page?: number;
  page_size?: number;
  remote_addr?: string;
  host?: string;
  node_id?: string;
  action?: 'all' | AttackLog['action'];
  since?: number;
  until?: number;
  exact?: boolean;
};

export type AttackLogPage<T> = { total: number; results: T[] };
