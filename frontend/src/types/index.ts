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
