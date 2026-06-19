// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

import {BaseService} from '@/lib/services/core';
import type {
  ChannelDefinition,
  CreateChannelRequest,
  CreatePushEventRequest,
  EventMetadata,
  ListPushHistoriesRequest,
  ListPushHistoriesResponse,
  PushChannel,
  PushEvent,
  TestChannelRequest,
  TestPushRequest,
  UpdateChannelRequest,
  UpdatePushEventRequest
} from './types';

/**
 * 通知推送服务类
 */
export class PushService extends BaseService {
  protected static readonly basePath = '/api/v1/admin/push';

  /**
   * 获取通知事件列表
   */
  static async listEvents(): Promise<PushEvent[]> {
    return this.get<PushEvent[]>('/events');
  }

  /**
   * 获取系统内置通知事件元数据列表
   */
  static async listBuiltInEvents(): Promise<EventMetadata[]> {
    return this.get<EventMetadata[]>('/events/builtin');
  }

  /**
   * 创建通知事件配置
   */
  static async createEvent(data: CreatePushEventRequest): Promise<PushEvent> {
    return this.post<PushEvent>('/events', data as unknown as Record<string, unknown>);
  }

  /**
   * 更新指定通知事件配置
   */
  static async updateEvent(id: number, data: UpdatePushEventRequest): Promise<void> {
    return this.put<void>(`/events/${id}`, data as unknown as Record<string, unknown>);
  }

  /**
   * 删除指定通知事件配置
   */
  static async deleteEvent(id: number): Promise<void> {
    return this.delete<void>(`/events/${id}`);
  }

  /**
   * 快捷切换事件启用状态
   */
  static async toggleEvent(id: number): Promise<boolean> {
    return this.post<boolean>(`/events/${id}/toggle`);
  }

  /**
   * 分页查询通知推送历史
   */
  static async listHistories(params: ListPushHistoriesRequest): Promise<ListPushHistoriesResponse> {
    return this.get<ListPushHistoriesResponse>('/histories', params as unknown as Record<string, unknown>);
  }

  /**
   * 发送测试推送进行联通性校验
   */
  static async testPush(data: TestPushRequest): Promise<void> {
    return this.post<void>('/test', data as unknown as Record<string, unknown>);
  }

  /**
   * 获取所有消息通道
   */
  static async listChannels(): Promise<PushChannel[]> {
    return this.get<PushChannel[]>('/channels');
  }

  /**
   * 创建新的消息通道
   */
  static async createChannel(data: CreateChannelRequest): Promise<PushChannel> {
    return this.post<PushChannel>('/channels', data as unknown as Record<string, unknown>);
  }

  /**
   * 更新指定消息通道
   */
  static async updateChannel(id: number, data: UpdateChannelRequest): Promise<PushChannel> {
    return this.put<PushChannel>(`/channels/${id}`, data as unknown as Record<string, unknown>);
  }

  /**
   * 删除指定消息通道
   */
  static async deleteChannel(id: number): Promise<void> {
    return this.delete<void>(`/channels/${id}`);
  }

  /**
   * 测试消息通道连通性
   */
  static async testChannel(data: TestChannelRequest): Promise<void> {
    return this.post<void>('/channels/test', data as unknown as Record<string, unknown>);
  }

  /**
   * 获取各消息通道的动态表单定义
   */
  static async listChannelDefinitions(): Promise<ChannelDefinition[]> {
    return this.get<ChannelDefinition[]>('/channels/definitions');
  }
}
