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
  health_check_model?: string;
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

export interface ClaudeConfigTemplate {
  proxy_url: string;
  api_key: string;
  account_name: string;
  config_json: string;
  config_id: string;
  install_cmd: {
    unix: string;
    windows: string;
  };
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
  build_date_beijing: string;
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

export type ShareExpireIn = '1h' | '24h' | '168h' | 'permanent';

export interface TrendPoint {
  timestamp: string;
  success_rate: number;
  avg_time: number;
}

export interface ProxySummary {
  success_rate: number;
  avg_response_time: number;
  total_requests: number;
  failed_requests: number;
}

export interface HealthSummary {
  status: string; // up/down/stale
  last_check_at: string | null;
  last_ping_ms: number;
  last_ping_err: string;
  check_method: string;
}

export interface HealthCheckRecord {
  node_id?: string;
  check_time: string;
  success: boolean;
  response_time_ms: number;
  error_message: string;
  check_method: string;
}

export interface HealthHistory {
  node_id: string;
  from: string;
  to: string;
  total: number;
  checks: HealthCheckRecord[];
}

export interface MonitorNode {
  id: string;
  name: string;
  url: string;
  status: 'online' | 'offline' | 'degraded' | 'disabled' | 'unknown';
  weight: number;
  is_active: boolean;
  circuit_open: boolean;
  disabled: boolean;
  last_error?: string;
  traffic: ProxySummary;
  health: HealthSummary;
  trend_24h?: TrendPoint[];
}

export interface MonitorDashboard {
  account_id: string;
  account_name: string;
  nodes: MonitorNode[];
  updated_at: string;
}

export interface MonitorShare {
  id: string;
  account_id?: string;
  token: string;
  expire_at?: string | null;
  created_by?: string;
  created_at: string;
  revoked?: boolean;
  revoked_at?: string | null;
  share_url?: string;
}

export interface CreateMonitorShareRequest {
  account_id?: string;
  expire_in: ShareExpireIn;
}

export type WSMessage =
  | {
      type: 'node_status' | 'node_metrics';
      payload: {
        node_id: string;
        node_name: string;
        status?: string;
        error?: string;
        traffic?: Partial<ProxySummary>;
        health?: Partial<HealthSummary>;
        // 兼容旧字段
        success_rate?: number;
        avg_response_time?: number;
        total_requests?: number;
        failed_requests?: number;
        last_ping_ms?: number;
        timestamp?: string;
      };
    }
  | {
      type: 'health_check';
      payload: {
        node_id: string;
        check_time: string;
        success: boolean;
        response_time_ms: number;
        error_message?: string;
        check_method?: string;
      };
    };
