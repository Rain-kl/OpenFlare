// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

export interface PushEvent {
  id: number;
  event_key: string;
  name: string;
  task_type?: string;
  channels: string[];
  targets: string[];
  template: string;
  enabled: boolean;
  created_at: string;
  updated_at: string;
}

export interface PushHistory {
  id: number;
  event_key: string;
  channel: string;
  target: string;
  title: string;
  content: string;
  level: 'INFO' | 'IMPORTANT' | 'CRITICAL';
  status: 'success' | 'failed';
  error_msg?: string;
  created_at: string;
}

export interface PushChannelConfig {
  channel: string;
  url?: string;
  secret?: string;
  key?: string;
  ext?: Record<string, unknown>;
}

export interface ListPushHistoriesRequest {
  page?: number;
  page_size?: number;
  event_key?: string;
  status?: string;
}

export interface ListPushHistoriesResponse {
  total: number;
  results: PushHistory[];
}

export interface UpdatePushEventRequest {
  channels: string[];
  targets: string[];
  template: string;
  enabled: boolean;
}

export interface TestPushRequest {
  config: PushChannelConfig;
  target?: string;
}

export interface EventMetadata {
  key: string;
  name: string;
  default_template: {
    title: string;
    content: string;
    level: string;
    ext?: Record<string, unknown>;
  };
  description: string;
}

export interface CreatePushEventRequest {
  event_key?: string;
  task_type?: string;
  channels: string[];
  targets?: string[];
  template?: string;
  enabled: boolean;
}

export interface PushChannel {
  id: number;
  name: string;
  description?: string;
  type: string;
  token?: string;
  url: string;
  other: string;
  enabled: boolean;
  created_at?: string;
  updated_at?: string;
}

export interface CreateChannelRequest {
  name: string;
  description?: string;
  type: string;
  token?: string;
  url: string;
  other: string;
  enabled?: boolean;
}

export interface UpdateChannelRequest {
  description?: string;
  type: string;
  token?: string;
  url: string;
  other: string;
  enabled?: boolean;
}

export interface TestChannelRequest {
  name?: string;
  type?: string;
  url?: string;
  other?: string;
  target?: string;
}

export interface ChannelFieldDef {
  key: string;
  label: string;
  type: 'text' | 'password' | 'textarea';
  required: boolean;
  placeholder?: string;
  description?: string;
}

export interface ChannelDefinition {
  type: string;
  name: string;
  description: string;
  fields: ChannelFieldDef[];
}

