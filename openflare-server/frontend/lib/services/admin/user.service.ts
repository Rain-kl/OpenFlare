import {BaseService} from '@/lib/services/core';
import type {
  AdminUser,
  CreateUserRequest,
  ListUsersRequest,
  ListUsersResponse,
  UpdateUserStatusRequest,
} from './types';

export class AdminUserService extends BaseService {
  protected static readonly basePath = '/api/v1/admin';

  static async listUsers(request: ListUsersRequest): Promise<ListUsersResponse> {
    return this.get<ListUsersResponse>('/users', request as unknown as Record<string, unknown>);
  }

  static async getUser(id: string): Promise<AdminUser> {
    return this.get<AdminUser>(`/users/${id}`);
  }

  static async updateUserStatus(id: string, request: UpdateUserStatusRequest): Promise<void> {
    return this.put<void>(`/users/${id}/status`, request);
  }

  static async createUser(request: CreateUserRequest): Promise<AdminUser> {
    return this.post<AdminUser>('/users', request);
  }

  static async deleteUser(id: string): Promise<void> {
    return this.delete<void>(`/users/${id}`);
  }
}