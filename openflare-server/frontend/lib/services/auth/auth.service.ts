import type {InternalAxiosRequestConfig} from 'axios';
import {BaseService} from '@/lib/services/core';
import type {
  AuthSource,
  ChangePasswordRequest,
  ExternalAccountBinding,
  LoginRequest,
  OAuthAuthorizeResponse,
  OAuthCallbackRequest,
  OAuthCallbackResult,
  OAuthLoginUrlResponse,
  RegisterRequest,
  UpdateProfileRequest,
  User,
} from './types';

/**
 * 认证服务
 * 处理 OAuth 认证、用户信息获取、登出等
 */
export class AuthService extends BaseService {
  protected static readonly basePath = '/api/v1';

  /**
   * 获取 OAuth 登录 URL
   * @returns OAuth 授权 URL
   * @throws {ApiErrorBase} 当获取失败时
   *
   * @example
   * ```typescript
   * const url = await AuthService.getLoginUrl();
   * window.location.href = url; // 重定向到 OAuth 授权页面
   * ```
   */
  static async getLoginUrl(): Promise<OAuthLoginUrlResponse> {
    return this.get<OAuthLoginUrlResponse>('/oauth/login');
  }

  static async getAuthSources(): Promise<AuthSource[]> {
    return this.get<AuthSource[]>('/oauth/sources');
  }

  static async getAuthorizeUrl(source: string, purpose: 'login' | 'bind' = 'login'): Promise<OAuthAuthorizeResponse> {
    return this.get<OAuthAuthorizeResponse>(`/oauth/${encodeURIComponent(source)}/authorize?purpose=${purpose}`);
  }

  /**
   * 处理 OAuth 回调
   * @param request - OAuth 回调参数（state 和 code）
   * @throws {ApiErrorBase} 当回调处理失败时
   * @throws {ValidationError} 当 state 无效时
   *
   * @example
   * ```typescript
   * // 在回调页面获取 URL 参数
   * const params = new URLSearchParams(window.location.search);
   * const state = params.get('state');
   * const code = params.get('code');
   *
   * if (state && code) {
   *   await AuthService.handleCallback({ state, code });
   *   // 登录成功，跳转到首页
   *   router.push('/home');
   * }
   * ```
   */
  static async handleCallback(request: OAuthCallbackRequest): Promise<OAuthCallbackResult> {
    return this.post<OAuthCallbackResult>('/oauth/callback', request);
  }

  /**
   * 获取当前登录用户信息
   * @returns 用户信息
   * @throws {UnauthorizedError} 当未登录时
   *
   * @example
   * ```typescript
   * try {
   *   const user = await AuthService.getUserInfo();
   *   console.log('当前用户:', user.username);
   * } catch (error) {
   *   if (error instanceof UnauthorizedError) {
   *     // 跳转到登录页
   *     router.push('/login');
   *   }
   * }
   * ```
   */
  static async getUserInfo(): Promise<User> {
    return this.get<User>('/user-info');
  }

  /**
   * 用户登出
   * @throws {ApiErrorBase} 当登出失败时
   *
   * @example
   * ```typescript
   * await AuthService.logout();
   * // 清除本地状态
   * router.push('/login');
   * ```
   */
  static async logout(): Promise<void> {
    await this.get<void>('/oauth/logout');
  }

  static async login(request: LoginRequest, headers?: Record<string, string>): Promise<User> {
    return this.post<User>('/user/login', request, headers ? ({ headers } as unknown as InternalAxiosRequestConfig) : undefined);
  }

  static async register(request: RegisterRequest, headers?: Record<string, string>): Promise<User> {
    return this.post<User>('/user/register', request, headers ? ({ headers } as unknown as InternalAxiosRequestConfig) : undefined);
  }

  static async sendEmailCode(email: string, scene: 'register' | 'login', headers?: Record<string, string>): Promise<void> {
    return this.post<void>('/user/send-email-code', { email, scene }, headers ? ({ headers } as unknown as InternalAxiosRequestConfig) : undefined);
  }

  static async changePassword(request: ChangePasswordRequest): Promise<void> {
    return this.post<void>('/user/change-password', request);
  }

  static async updateProfile(request: UpdateProfileRequest): Promise<User> {
    return this.put<User>('/user/profile', request);
  }

  static async getExternalAccountBindings(): Promise<ExternalAccountBinding[]> {
    return this.get<ExternalAccountBinding[]>('/oauth/external-accounts');
  }

  static async deleteExternalAccountBinding(id: string): Promise<void> {
    return this.post<void>(`/oauth/external-accounts/${encodeURIComponent(id)}/delete`);
  }
}
