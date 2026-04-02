import { request } from "./api";

export interface Setting {
  id: number;
  key: string;
  scope: string;
  account_id?: string;
  value: any;
  data_type: string;
  category: string;
  description?: string;
  is_secret: boolean;
  version: number;
  updated_at: string;
}

export interface RuntimeSettingDefinition {
  key: string;
  label: string;
  category: string;
  category_label: string;
  description: string;
  data_type: "number" | "boolean" | string;
  default_value: any;
  unit?: string;
  min?: number;
  max?: number;
  step?: number;
  persisted: boolean;
  hot_reload: boolean;
  apply_mode: "on_change" | "read_through" | "frontend_poll" | string;
  apply_mode_label: string;
  sync_note: string;
}

export interface SettingsResponse {
  data: Setting[];
  version: number;
}

interface SettingResponse {
  data: Setting;
  version: number;
}

interface UpdateSettingPayload {
  value: any;
  version?: number;
  scope?: string;
  data_type?: string;
  category?: string;
  description?: string;
}

export const settingsApi = {
  // 获取配置列表
  list: async (params?: {
    scope?: string;
    category?: string;
  }): Promise<SettingsResponse> => {
    const query = params ? new URLSearchParams(params as any).toString() : "";
    const url = query ? `/api/settings?${query}` : "/api/settings";
    return request<SettingsResponse>(url);
  },

  // 获取单个配置
  get: async (key: string): Promise<Setting> => {
    const res = await request<SettingResponse>(
      `/api/settings/${encodeURIComponent(key)}`,
    );
    return res.data;
  },

  // 获取运行时配置目录
  getRuntimeDefinitions: async (): Promise<RuntimeSettingDefinition[]> => {
    const res = await request<{ data: RuntimeSettingDefinition[] }>(
      "/api/settings/runtime-definitions",
    );
    return res.data || [];
  },

  // 保存配置（存在时更新，不存在时创建）
  save: async (
    key: string,
    payload: UpdateSettingPayload,
  ): Promise<{ success: boolean; new_version: number }> => {
    return request(`/api/settings/${encodeURIComponent(key)}`, {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        scope: "system",
        ...payload,
      }),
    });
  },

  // 更新配置
  update: async (
    key: string,
    value: any,
    version: number,
    scope = "system",
  ): Promise<{ success: boolean; new_version: number }> => {
    return settingsApi.save(key, { value, version, scope });
  },

  // 获取版本号
  getVersion: async (): Promise<number> => {
    const res = await request<{ version: number }>("/api/settings/version");
    return res.version || 0;
  },
};
