export interface Account {
  id: string;
  name: string;
  proxy_api_key: string;
  is_admin: boolean;
}

export interface Node {
  id: string;
  name: string;
  base_url: string;
  weight: number;
  health_check_method?: 'api' | 'head' | 'cli';
  has_api_key?: boolean;
  active: boolean;
  failed: boolean;
  disabled: boolean;
  health_rate?: number;
  requests?: number;
  fail_count?: number;
  fail_streak?: number;
  last_error?: string;
  total_bytes?: number;
  stream_dur_ms?: number;
  input_tokens?: number;
  output_tokens?: number;
  last_health_check_at?: string;
  last_ping_ms?: number;
  last_ping_error?: string;
  created_at?: string;
}

export interface Config {
  retries: number;
  fail_limit: number;
  health_interval_sec: number;
}

export interface TunnelState {
  api_token_set: boolean;
  subdomain: string;
  zone: string;
  enabled: boolean;
  public_url: string;
  status: string;
  last_error: string;
}

export interface VersionInfo {
  version: string;
  git_commit: string;
  build_date: string;
  go_version: string;
}

export interface NotificationChannel {
  id: string;
  name: string;
  channel_type: string; // wechat_work, email, dingtalk, etc.
  enabled: boolean;
  created_at: string;
  updated_at?: string;
}

export interface CreateChannelRequest {
  name: string;
  channel_type: string;
  config: {
    webhook_url?: string;
    [key: string]: any;
  };
  enabled: boolean;
}

export interface NotificationSubscription {
  id: string;
  channel_id: string;
  event_type: string;
  enabled: boolean;
  created_at: string;
  updated_at?: string;
}

export interface CreateSubscriptionsRequest {
  channel_id: string;
  event_types: string[];
  enabled: boolean;
}

export interface EventType {
  type: string;
  category: string; // node, request, account, system
  description: string;
}

export interface TestNotificationRequest {
  channel_id: string;
  title: string;
  content: string;
}
