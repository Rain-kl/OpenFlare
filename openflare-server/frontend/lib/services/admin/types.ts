

/**
 * 系统配置信息
 */
export interface SystemConfig {
  /** 配置键 */
  key: string;
  /** 配置值 */
  value: string;
  /** 配置类型：'system' | 'business' */
  type: 'system' | 'business';
  /** 是否对公共配置接口可见：0 不可见，1 可见 */
  visibility: 0 | 1;
  /** 配置描述 */
  description: string;
  /** 创建时间 */
  created_at: string;
  /** 更新时间 */
  updated_at: string;
}

/**
 * 创建系统配置请求参数
 */
export interface CreateSystemConfigRequest {
  /** 配置键（最大64字符） */
  key: string;
  /** 配置值 */
  value: string;
  /** 配置类型：'system' | 'business' */
  type: 'system' | 'business';
  /** 是否对公共配置接口可见：0 不可见，1 可见 */
  visibility?: 0 | 1;
  /** 配置描述（最大255字符，可选） */
  description?: string;
}

/**
 * 更新系统配置请求参数
 */
export interface UpdateSystemConfigRequest {
  /** 配置值 */
  value: string;
  /** 是否对公共配置接口可见：0 不可见，1 可见 */
  visibility?: 0 | 1;
  /** 配置描述（最大255字符，可选） */
  description?: string;
}

export type StorageDriver = 'local' | 's3' | 'r2' | 'minio' | 'oss' | 'webdav';

export interface ObjectStorageConfig {
  endpoint: string;
  region: string;
  bucket: string;
  access_key_id: string;
  secret_access_key: string;
  account_id?: string;
  path_style: boolean;
  key_prefix: string;
  cdn_url: string;
}

export interface StorageConfig {
  driver: StorageDriver;
  local: { root: string };
  s3: ObjectStorageConfig;
  r2: ObjectStorageConfig;
  minio: ObjectStorageConfig;
  oss: ObjectStorageConfig;
  webdav: {
    endpoint: string;
    username: string;
    password: string;
    base_path: string;
  };
}

// ==================== 任务管理 ====================

/**
 * 任务参数响应
 */
export interface TaskParamResponse {
  Name?: string;
  name?: string;
  Label?: string;
  label?: string;
  Type?: string;
  type?: string;
  Required?: boolean;
  required?: boolean;
  Placeholder?: string;
  placeholder?: string;
  Description?: string;
  description?: string;
}

/**
 * 任务类型响应
 */
export interface TaskTypeResponse {
  Type?: string;
  type?: string;
  AsynqTask?: string;
  asynq_task?: string;
  Name?: string;
  name?: string;
  Description?: string;
  description?: string;
  SupportsTime?: boolean;
  supports_time?: boolean;
  MaxRetry?: number;
  max_retry?: number;
  Queue?: string;
  queue?: string;
  Params?: TaskParamResponse[];
  params?: TaskParamResponse[];
}

/**
 * 任务参数的数据类型
 * - string  → 单行文本输入，JSON 中序列化为 string
 * - text    → 多行文本输入，JSON 中序列化为 string
 * - number  → 数字输入，JSON 中序列化为 number（而非 string）
 * - boolean → 开关输入，JSON 中序列化为 boolean（而非 string）
 */
export type TaskParamType = 'string' | 'text' | 'number' | 'boolean';

/**
 * 任务参数定义
 */
export interface TaskParam {
  name: string;
  label: string;
  type: TaskParamType;
  required: boolean;
  placeholder: string;
  description: string;
}

/**
 * 任务元数据
 */
export interface TaskMeta {
  /** 任务类型标识 */
  type: string;
  /** Asynq 任务名称 */
  asynq_task: string;
  /** 任务名称 */
  name: string;
  /** 任务描述 */
  description: string;
  /** 是否支持时间范围参数 */
  supports_time: boolean;
  /** 最大重试次数 */
  max_retry: number;
  /** 队列名称 */
  queue: string;
  /** 任务所需的自定义参数定义 */
  params?: TaskParam[];
}

/**
 * 下发任务请求参数
 */
export interface DispatchTaskRequest {
  /** 任务类型 */
  task_type: string;
  /** 开始时间（可选，仅部分任务支持） */
  start_time?: string;
  /** 结束时间（可选，仅部分任务支持） */
  end_time?: string;
  /** 用户 ID（可选，仅部分任务需要） */
  user_id?: string;
  /** 任务自定义参数 JSON 字符串 */
  payload?: string;
}

export type TaskExecutionStatus = 'pending' | 'running' | 'succeeded' | 'failed';

/**
 * 任务执行记录
 */
export interface TaskExecution {
  id: string;
  task_id: string;
  task_type: string;
  task_name: string;
  status: TaskExecutionStatus;
  retryable: boolean;
  max_retry: number;
  retry_count: number;
  log: string;
  error_message: string;
  result: string;
  started_at?: string | null;
  finished_at?: string | null;
  duration: number;
  payload: string;
  triggered_by: string;
  created_at: string;
  updated_at: string;
}

/**
 * 查询任务执行记录请求参数
 */
export interface ListTaskExecutionsRequest {
  status?: TaskExecutionStatus;
  task_type?: string;
  page?: number;
  page_size?: number;
}

/**
 * 查询任务执行记录响应
 */
export interface ListTaskExecutionsResponse {
  items: TaskExecution[];
  total: number;
  page: number;
  page_size: number;
}

/**
 * 定时任务配置信息
 */
export interface Schedule {
  id: string;
  name: string;
  task_type: string;
  cron: string;
  payload: string;
  is_active: boolean;
  created_at: string;
  updated_at: string;
}

/**
 * 创建定时任务请求参数
 */
export interface CreateScheduleRequest {
  name: string;
  task_type: string;
  cron: string;
  payload: string;
  is_active: boolean;
}

/**
 * 修改定时任务请求参数
 */
export interface UpdateScheduleRequest {
  name: string;
  task_type: string;
  cron: string;
  payload: string;
  is_active: boolean;
}

// ==================== 用户管理 ====================

/**
 * 管理员用户信息
 */
export interface AdminUser {
  /** 用户 ID */
  id: string;
  /** 用户名 */
  username: string;
  /** 昵称 */
  nickname: string;
  /** 邮箱 */
  email?: string;
  /** 头像 URL */
  avatar_url: string;
  /** 是否激活 */
  is_active: boolean;
  /** 是否管理员 */
  is_admin: boolean;
  /** 个人简介 */
  bio?: string;
  /** 手机号 */
  phone?: string;
  /** 性别 */
  gender?: string;
  /** 个人网站 */
  website?: string;
  /** 所在地 */
  location?: string;
  /** 最后登录时间 */
  last_login_at: string;
  /** 创建时间 */
  created_at: string;
  /** 更新时间 */
  updated_at: string;
}

/**
 * 用户列表查询请求参数
 */
export interface ListUsersRequest {
  /** 页码（从 1 开始） */
  page: number;
  /** 每页数量（1-100） */
  page_size: number;
  /** 用户 ID 精确过滤（可选） */
  user_id?: string;
  /** 用户名前缀过滤（可选） */
  username?: string;
}

/**
 * 用户列表响应
 */
export interface ListUsersResponse {
  /** 用户列表 */
  users: AdminUser[];
  /** 总数 */
  total: number;
}

/**
 * 更新用户状态请求参数
 */
export interface UpdateUserStatusRequest {
  /** 是否激活 */
  is_active: boolean;
}

/**
 * 创建用户请求参数
 */
export interface CreateUserRequest {
  /** 用户名 */
  username: string;
  /** 密码 */
  password: string;
  /** 昵称 */
  nickname?: string;
  /** 邮箱 */
  email: string;
  /** 是否激活 */
  is_active?: boolean;
  /** 是否管理员 */
  is_admin?: boolean;
}

/**
 * 认证源信息
 */
export interface AuthSource {
  id: string;
  name: string;
  type: 'oidc';
  display_name: string;
  is_active: boolean;
  client_id: string;
  client_secret?: string;
  client_secret_configured?: boolean;
  openid_discovery_url: string;
  scopes: string;
  icon_url: string;
  created_at: string;
  updated_at: string;
}

export interface AuthSourceRequest {
  name: string;
  type: 'oidc';
  display_name: string;
  is_active: boolean;
  client_id: string;
  client_secret: string;
  openid_discovery_url: string;
  scopes: string;
  icon_url: string;
}

export interface ToggleAuthSourceRequest {
  is_active: boolean;
}

/**
 * 系统状态信息
 */
export interface SystemStatus {
  uptime: string;
  num_goroutine: number;
  alloc: string;
  total_alloc: string;
  sys: string;
  lookups: number;
  mallocs: number;
  frees: number;
  heap_alloc: string;
  heap_sys: string;
  heap_idle: string;
  heap_inuse: string;
  heap_released: string;
  heap_objects: number;
  stack_inuse: string;
  stack_sys: string;
  mspan_inuse: string;
  mspan_sys: string;
  mcache_inuse: string;
  mcache_sys: string;
  buck_hash_sys: string;
  gc_sys: string;
  other_sys: string;
  next_gc: string;
  last_gc_time: string;
  pause_total_ns: string;
  last_pause: string;
  num_gc: number;
}

/**
 * 模板配置信息
 */
export interface Template {
  id: string;
  key: string;
  name: string;
  type: string;
  subject: string;
  content: string;
  description: string;
  is_system: boolean;
  created_at: string;
  updated_at: string;
}

/**
 * 创建模板请求参数
 */
export interface CreateTemplateRequest {
  key: string;
  name: string;
  type: string;
  subject: string;
  content: string;
  description: string;
}

/**
 * 更新模板请求参数
 */
export interface UpdateTemplateRequest {
  name: string;
  type: string;
  subject: string;
  content: string;
  description: string;
}

/**
 * 应用更新状态
 */
export interface AppUpdateStatus {
  current_version: string;
  build_time: string;
  latest_version: string;
  update_available: boolean;
  can_upgrade: boolean;
  prerelease: boolean;
  release_name: string;
  release_notes: string;
  release_url: string;
  published_at: string;
  upstream_repository: string;
  asset_name: string;
  platform: string;
}

// ==================== 缓存管理 ====================

/**
 * 缓存状态信息
 */
export interface CacheStatus {
  /** 缓存文件总大小（字节） */
  total_size: number;
  /** 缓存 Key 数量 */
  keys_count: number;
  /** 最大容量限制 (MB) */
  max_size_mb: number;
  /** 过期时间 (分钟) */
  ttl_minutes: number;
  /** 是否启用 LRU 淘汰 */
  lru_enabled: boolean;
  /** 缓存目录基准路径 */
  base_path: string;
}

/**
 * 缓存配置请求参数
 */
export interface CacheConfig {
  /** 最大容量限制 (MB) */
  max_size_mb: number;
  /** 过期时间 (分钟) */
  ttl_minutes: number;
  /** 是否启用 LRU 淘汰 */
  lru_enabled: boolean;
}
