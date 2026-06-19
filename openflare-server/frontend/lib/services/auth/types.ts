/**
 * 用户基本信息
 */
export interface User {
  /** 用户 ID */
  id: string;
  /** 账户 */
  username: string;
  /** 昵称 */
  nickname: string;
  /** 头像 URL */
  avatar_url: string;
  /** 是否为管理员 */
  is_admin: boolean;
  /** 是否需要修改密码 */
  need_change_password?: boolean;
  /** 邮箱 */
  email: string;
  /** 个人简介 */
  bio?: string;
  /** 手机号码 */
  phone?: string;
  /** 性别 */
  gender?: string;
  /** 个人网站 */
  website?: string;
  /** 所在地 */
  location?: string;
}

export interface UpdateProfileRequest {
  nickname: string;
  email: string;
  avatar_url: string;
  bio?: string;
  phone?: string;
  gender?: string;
  website?: string;
  location?: string;
}

/**
 * OAuth 登录 URL 响应
 * 后端直接返回字符串 URL
 */
export type OAuthLoginUrlResponse = string;

/**
 * OAuth 回调请求参数
 */
export interface OAuthCallbackRequest {
  /** 状态码 */
  state: string;
  /** 授权码 */
  code: string;
}

export interface LoginRequest {
  username: string;
  password: string;
  code?: string;
}

export interface RegisterRequest {
  username: string;
  password: string;
  nickname?: string;
  email?: string;
  code?: string;
}

export interface OAuthAuthorizeResponse {
  authorize_url: string;
}

export interface OAuthCallbackResult {
  status: 'logged_in' | 'bound' | 'need_bind';
  user?: User;
}

export interface AuthSource {
  id: string;
  name: string;
  type: 'oidc';
  display_name: string;
  is_active: boolean;
  icon_url: string;
  client_secret_configured: boolean;
}

export interface ExternalAccountBinding {
  id: string;
  auth_source_id: string;
  auth_source_name: string;
  auth_source_type: string;
  auth_source_label: string;
  external_username: string;
  email: string;
  created_at: string;
}

export interface ChangePasswordRequest {
  old_password: string;
  new_password: string;
}
