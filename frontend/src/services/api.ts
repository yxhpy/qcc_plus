import type {
  Account,
  Node,
  Config,
  TunnelState,
  VersionInfo,
  NotificationChannel,
  CreateChannelRequest,
  NotificationSubscription,
  CreateSubscriptionsRequest,
  EventType,
  TestNotificationRequest,
  MonitorDashboard,
  MonitorShare,
  CreateMonitorShareRequest,
  HealthHistory,
  ClaudeConfigTemplate,
  CCSwitchImportSummary,
  ModelPricing,
  UsageLog,
  UsageSummary,
  UsageQueryParams,
} from '../types'

const defaultHeaders = { 'Content-Type': 'application/json' }
const noCacheHeaders = { 'Cache-Control': 'no-cache', Pragma: 'no-cache' }

async function parseJSON<T>(res: Response): Promise<T> {
  const ct = res.headers.get('content-type') || ''
  if (res.redirected || res.url.includes('/login')) {
    throw new Error('unauthenticated')
  }
  const text = await res.text()
  if (!ct.includes('application/json')) {
    throw new Error(text || 'unexpected response')
  }
  try {
    return JSON.parse(text) as T
  } catch (err) {
    throw new Error('invalid json response')
  }
}

async function parseErrorMessage(res: Response): Promise<string> {
  const fallback = res.clone()
  let message = res.statusText || 'request failed'
  try {
    const data = await res.json()
    message = (data as any).error || (data as any).message || message
  } catch (err) {
    try {
      const text = await fallback.text()
      if (text) {
        message = text
      }
    } catch (_err) {
      /* ignore */
    }
  }
  return message || 'request failed'
}

async function request<T>(url: string, options: RequestInit = {}): Promise<T> {
  const res = await fetch(url, { credentials: 'include', ...options })
  if (!res.ok) {
    throw new Error(await parseErrorMessage(res))
  }
  // 204 No Content 不需要解析响应体
  if (res.status === 204) {
    return undefined as T
  }
  return parseJSON<T>(res)
}

async function requestBlob(url: string, options: RequestInit = {}): Promise<{ blob: Blob; filename: string }> {
  const res = await fetch(url, { credentials: 'include', ...options })
  if (!res.ok) {
    throw new Error(await parseErrorMessage(res))
  }
  const blob = await res.blob()
  const disposition = res.headers.get('content-disposition') || ''
  const filename = parseContentDispositionFilename(disposition) || 'download.bin'
  return { blob, filename }
}

function parseContentDispositionFilename(disposition: string): string {
  const utf8Match = disposition.match(/filename\*=UTF-8''([^;]+)/i)
  if (utf8Match?.[1]) {
    return decodeURIComponent(utf8Match[1])
  }
  const match = disposition.match(/filename="?([^"]+)"?/i)
  return match?.[1] || ''
}

function withNoCacheParam(url: string): string {
  const separator = url.includes('?') ? '&' : '?'
  return `${url}${separator}_ts=${Date.now()}`
}

export { request }

async function login(username: string, password: string): Promise<void> {
  const body = new URLSearchParams({ username, password })
  const res = await fetch('/login', {
    method: 'POST',
    body,
    credentials: 'include',
    redirect: 'follow',
  })
  if (!res.ok) {
    throw new Error('登录失败')
  }
  // validate session by requesting an authenticated endpoint
  try {
    const account = await getSession()
    if (!account) {
      throw new Error('invalid session')
    }
  } catch (err) {
    throw new Error('账号名称或密码错误')
  }
}

async function logout(): Promise<void> {
  await fetch('/logout', { method: 'POST', credentials: 'include', redirect: 'follow' })
}

async function getSession(): Promise<Account | null> {
  const data = await request<{ account: Account | null }>('/api/session')
  return data.account || null
}

async function getAccounts(): Promise<Account[]> {
  const data = await request<{ accounts: Account[] }>('/admin/api/accounts')
  return data.accounts || []
}

async function createAccount(payload: {
  name: string
  password?: string
  proxy_api_key: string
  is_admin: boolean
}): Promise<string> {
  const data = await request<{ id: string }>('/admin/api/accounts', {
    method: 'POST',
    headers: defaultHeaders,
    body: JSON.stringify(payload),
  })
  return data.id
}

async function updateAccount(id: string, payload: {
  name?: string
  password?: string
  proxy_api_key?: string
  is_admin?: boolean
}): Promise<void> {
  await request(`/admin/api/accounts?id=${encodeURIComponent(id)}`, {
    method: 'PUT',
    headers: defaultHeaders,
    body: JSON.stringify(payload),
  })
}

async function deleteAccount(id: string): Promise<void> {
  await request(`/admin/api/accounts?id=${encodeURIComponent(id)}`, {
    method: 'DELETE',
  })
}

function withAccount(url: string, accountId?: string): string {
  if (!accountId) return url
  const sep = url.includes('?') ? '&' : '?'
  return `${url}${sep}account_id=${encodeURIComponent(accountId)}`
}

async function getNodes(accountId?: string): Promise<Node[]> {
  const data = await request<{ nodes: Node[] }>(withAccount('/admin/api/nodes', accountId))
  return data.nodes || []
}

async function getNode(id: string, accountId?: string): Promise<Node> {
  return request<{ node: Node }>(withAccount(`/admin/api/nodes?id=${encodeURIComponent(id)}`, accountId)).then((data) => data.node)
}

async function createNode(payload: {
  name?: string
  base_url: string
  api_key?: string
  api_keys?: Node['api_keys']
  weight?: number
  max_concurrency?: number
  health_check_method?: Node['health_check_method']
  health_check_model?: string
  model_mapping?: Record<string, string>
  source_protocol?: Node['source_protocol']
  wire_api?: Node['wire_api']
  auth_profile?: string
  capabilities?: string
}, accountId?: string): Promise<string> {
  const data = await request<{ id: string }>(withAccount('/admin/api/nodes', accountId), {
    method: 'POST',
    headers: defaultHeaders,
    body: JSON.stringify(payload),
  })
  return data.id
}

async function updateNode(id: string, payload: Partial<Pick<Node, 'name' | 'base_url' | 'weight' | 'max_concurrency' | 'health_check_method' | 'health_check_model' | 'model_mapping' | 'source_protocol' | 'wire_api' | 'auth_profile' | 'capabilities'>> & { api_key?: string; api_keys?: Node['api_keys'] }): Promise<void> {
  await request(`/admin/api/nodes?id=${encodeURIComponent(id)}`, {
    method: 'PUT',
    headers: defaultHeaders,
    body: JSON.stringify(payload),
  })
}

async function copyNode(id: string): Promise<string> {
  const data = await request<{ id: string }>('/admin/api/nodes/copy', {
    method: 'POST',
    headers: defaultHeaders,
    body: JSON.stringify({ id }),
  })
  return data.id
}

async function deleteNode(id: string): Promise<void> {
  await request(`/admin/api/nodes?id=${encodeURIComponent(id)}`, {
    method: 'DELETE',
  })
}

async function importCCSwitchDB(
  file: File,
  payload: {
    account_id?: string
    import_providers?: boolean
    import_pricing?: boolean
    import_logs?: boolean
    weight_offset?: number
  },
): Promise<{ summary: CCSwitchImportSummary; file_name?: string }> {
  const form = new FormData()
  form.set('file', file)
  if (payload.account_id) form.set('account_id', payload.account_id)
  if (payload.import_providers !== undefined) form.set('import_providers', String(payload.import_providers))
  if (payload.import_pricing !== undefined) form.set('import_pricing', String(payload.import_pricing))
  if (payload.import_logs !== undefined) form.set('import_logs', String(payload.import_logs))
  if (payload.weight_offset !== undefined) form.set('weight_offset', String(payload.weight_offset))
  return request<{ summary: CCSwitchImportSummary; file_name?: string }>('/admin/api/cc-switch/import', {
    method: 'POST',
    body: form,
  })
}

async function exportCCSwitchDB(accountId?: string): Promise<{ blob: Blob; filename: string }> {
  return requestBlob(withAccount('/admin/api/cc-switch/export', accountId))
}

async function activateNode(id: string): Promise<void> {
  await request('/admin/api/nodes/activate', {
    method: 'POST',
    headers: defaultHeaders,
    body: JSON.stringify({ id }),
  })
}

async function toggleNode(id: string, disabled: boolean): Promise<void> {
  const url = disabled ? '/admin/api/nodes/enable' : '/admin/api/nodes/disable'
  await request(url, {
    method: 'POST',
    headers: defaultHeaders,
    body: JSON.stringify({ id }),
  })
}

async function getMonitorDashboard(accountId?: string): Promise<MonitorDashboard> {
  const url = withAccount('/api/monitor/dashboard', accountId)
  return request<MonitorDashboard>(url)
}

async function getHealthHistory(
  nodeId: string,
  from?: string,
  to?: string,
  shareToken?: string,
  source?: string,
): Promise<HealthHistory> {
  const params = new URLSearchParams()
  if (from) params.set('from', from)
  if (to) params.set('to', to)
  if (shareToken) params.set('share_token', shareToken)
  if (source) params.set('source', source)
  const qs = params.toString()
  const url = qs
    ? `/api/nodes/${encodeURIComponent(nodeId)}/health-history?${qs}`
    : `/api/nodes/${encodeURIComponent(nodeId)}/health-history`
  return request<HealthHistory>(url)
}

type CreateMonitorShareResponse = {
  id: string
  token: string
  share_url?: string
  expire_at?: string | null
  created_at: string
  account_id?: string
  created_by?: string
}

async function createMonitorShare(payload: CreateMonitorShareRequest): Promise<MonitorShare> {
  const res = await request<CreateMonitorShareResponse>('/api/monitor/shares', {
    method: 'POST',
    headers: defaultHeaders,
    body: JSON.stringify(payload),
  })
  return {
    id: res.id,
    token: res.token,
    share_url: res.share_url,
    expire_at: res.expire_at ?? undefined,
    created_at: res.created_at,
    account_id: res.account_id || payload.account_id || '',
    created_by: res.created_by || '',
    revoked: false,
    revoked_at: undefined,
  }
}

async function getMonitorShares(
  accountId?: string,
  limit = 20,
  offset = 0,
): Promise<{ shares: MonitorShare[]; total?: number }> {
  const params = new URLSearchParams()
  if (accountId) params.set('account_id', accountId)
  params.set('limit', String(limit))
  params.set('offset', String(offset))
  const qs = params.toString()
  const url = qs ? `/api/monitor/shares?${qs}` : '/api/monitor/shares'
  const data = await request<{ shares: MonitorShare[]; total?: number }>(url)
  return { shares: data.shares || [], total: data.total }
}

async function revokeMonitorShare(id: string): Promise<void> {
  await request(`/api/monitor/shares/${encodeURIComponent(id)}`, { method: 'DELETE' })
}

async function getSharedMonitor(token: string): Promise<MonitorDashboard> {
  return request<MonitorDashboard>(`/api/monitor/share/${encodeURIComponent(token)}`)
}

async function getConfig(accountId?: string): Promise<Config> {
  return request<Config>(withAccount('/admin/api/config', accountId))
}

async function updateConfig(payload: Config, accountId?: string): Promise<void> {
  await request(withAccount('/admin/api/config', accountId), {
    method: 'PUT',
    headers: defaultHeaders,
    body: JSON.stringify(payload),
  })
}

async function getTunnel(): Promise<TunnelState> {
  return request<TunnelState>('/admin/api/tunnel')
}

async function saveTunnel(payload: {
  api_token?: string
  subdomain?: string
  zone?: string
  enabled?: boolean
}): Promise<TunnelState> {
  return request<TunnelState>('/admin/api/tunnel', {
    method: 'PUT',
    headers: defaultHeaders,
    body: JSON.stringify(payload),
  })
}

async function startTunnel(): Promise<TunnelState> {
  return request<TunnelState>('/admin/api/tunnel/start', {
    method: 'POST',
    headers: defaultHeaders,
  })
}

async function stopTunnel(): Promise<TunnelState> {
  return request<TunnelState>('/admin/api/tunnel/stop', {
    method: 'POST',
    headers: defaultHeaders,
  })
}

async function listZones(): Promise<string[]> {
  const res = await request<{ zones: string[] }>('/admin/api/tunnel/zones')
  return res.zones || []
}

async function getVersion(): Promise<VersionInfo> {
  return request<VersionInfo>(withNoCacheParam('/version'), {
    cache: 'no-store',
    headers: noCacheHeaders,
  })
}

async function getChangelog(): Promise<string> {
  const res = await fetch(withNoCacheParam('/changelog'), {
    credentials: 'include',
    cache: 'no-store',
    headers: noCacheHeaders,
  })
  const text = await res.text()
  if (res.redirected || res.url.includes('/login')) {
    throw new Error('unauthenticated')
  }
  if (!res.ok) {
    throw new Error(text || '加载更新日志失败')
  }
  return text
}

async function getNotificationChannels(): Promise<NotificationChannel[]> {
  try {
    const result = await request<{ channels: NotificationChannel[] }>('/api/notification/channels')
    return Array.isArray(result?.channels) ? result.channels : []
  } catch (err) {
    console.error('Failed to fetch notification channels:', err)
    return []
  }
}

async function createNotificationChannel(data: CreateChannelRequest): Promise<NotificationChannel> {
  return request<NotificationChannel>('/api/notification/channels', {
    method: 'POST',
    headers: defaultHeaders,
    body: JSON.stringify(data),
  })
}

async function updateNotificationChannel(id: string, data: Partial<CreateChannelRequest>): Promise<void> {
  await request(`/api/notification/channels/${encodeURIComponent(id)}`, {
    method: 'PUT',
    headers: defaultHeaders,
    body: JSON.stringify(data),
  })
}

async function deleteNotificationChannel(id: string): Promise<void> {
  await request(`/api/notification/channels/${encodeURIComponent(id)}`, {
    method: 'DELETE',
  })
}

async function getNotificationSubscriptions(channelId?: string): Promise<NotificationSubscription[]> {
  const url = channelId
    ? `/api/notification/subscriptions?channel_id=${encodeURIComponent(channelId)}`
    : '/api/notification/subscriptions'
  try {
    const result = await request<{ subscriptions: NotificationSubscription[] }>(url)
    return Array.isArray(result?.subscriptions) ? result.subscriptions : []
  } catch (err) {
    console.error('Failed to fetch subscriptions:', err)
    return []
  }
}

async function createNotificationSubscriptions(data: CreateSubscriptionsRequest): Promise<void> {
  await request('/api/notification/subscriptions', {
    method: 'POST',
    headers: defaultHeaders,
    body: JSON.stringify(data),
  })
}

async function updateNotificationSubscription(id: string, enabled: boolean): Promise<void> {
  await request(`/api/notification/subscriptions/${encodeURIComponent(id)}`, {
    method: 'PUT',
    headers: defaultHeaders,
    body: JSON.stringify({ enabled }),
  })
}

async function deleteNotificationSubscription(id: string): Promise<void> {
  await request(`/api/notification/subscriptions/${encodeURIComponent(id)}`, {
    method: 'DELETE',
  })
}

async function getEventTypes(): Promise<EventType[]> {
  try {
    const result = await request<{ event_types: EventType[] }>('/api/notification/event-types')
    return Array.isArray(result?.event_types) ? result.event_types : []
  } catch (err) {
    console.error('Failed to fetch event types:', err)
    return []
  }
}

async function testNotification(data: TestNotificationRequest): Promise<void> {
  await request('/api/notification/test', {
    method: 'POST',
    headers: defaultHeaders,
    body: JSON.stringify(data),
  })
}

type ClaudeTemplateParams = {
  proxy_url?: string
  api_key?: string
  model?: string
  allow?: string[]
  deny?: string[]
}

async function getClaudeConfigTemplate(params?: ClaudeTemplateParams): Promise<ClaudeConfigTemplate> {
  const search = new URLSearchParams()
  if (params?.proxy_url) search.set('proxy_url', params.proxy_url)
  if (params?.api_key) search.set('api_key', params.api_key)
  if (params?.model) search.set('model', params.model)
  params?.allow?.forEach((v) => {
    const val = v.trim()
    if (val) search.append('allow', val)
  })
  params?.deny?.forEach((v) => {
    const val = v.trim()
    if (val) search.append('deny', val)
  })
  const qs = search.toString()
  const url = qs ? `/api/claude-config/template?${qs}` : '/api/claude-config/template'
  return request<ClaudeConfigTemplate>(url)
}

// 定价管理 API
async function getPricingList(activeOnly = false): Promise<ModelPricing[]> {
  const url = activeOnly ? '/api/pricing?active_only=true' : '/api/pricing'
  const data = await request<{ pricing: ModelPricing[] }>(url)
  return data.pricing || []
}

async function getPricing(modelId: string): Promise<ModelPricing> {
  return request<ModelPricing>(`/api/pricing?id=${encodeURIComponent(modelId)}`)
}

async function savePricing(pricing: Partial<ModelPricing>): Promise<string> {
  const data = await request<{ model_id: string }>('/api/pricing', {
    method: 'POST',
    headers: defaultHeaders,
    body: JSON.stringify(pricing),
  })
  return data.model_id
}

async function deletePricing(modelId: string): Promise<void> {
  await request(`/api/pricing?id=${encodeURIComponent(modelId)}`, {
    method: 'DELETE',
  })
}

async function syncOfficialPricing(): Promise<{ message: string; synced: number }> {
  return request<{ message: string; synced: number }>('/api/pricing/sync', {
    method: 'POST',
  })
}

// 使用统计 API
async function getUsageLogs(params: UsageQueryParams = {}, includeAttempts = false): Promise<{ logs: UsageLog[]; count: number; total: number }> {
  const search = new URLSearchParams()
  if (params.account_id) search.set('account_id', params.account_id)
  if (params.node_id) search.set('node_id', params.node_id)
  if (params.model_id) search.set('model_id', params.model_id)
  if (params.from) search.set('from', params.from)
  if (params.to) search.set('to', params.to)
  if (params.limit) search.set('limit', String(params.limit))
  if (params.offset) search.set('offset', String(params.offset))
  if (params.success) search.set('success', params.success)
  if (includeAttempts) search.set('include_attempts', 'true')
  const qs = search.toString()
  const url = qs ? `/api/usage/logs?${qs}` : '/api/usage/logs'
  return request<{ logs: UsageLog[]; count: number; total: number }>(url)
}

async function getUsageSummary(params: UsageQueryParams = {}): Promise<UsageSummary | UsageSummary[]> {
  const search = new URLSearchParams()
  if (params.account_id) search.set('account_id', params.account_id)
  if (params.node_id) search.set('node_id', params.node_id)
  if (params.model_id) search.set('model_id', params.model_id)
  if (params.from) search.set('from', params.from)
  if (params.to) search.set('to', params.to)
  if (params.group_by) search.set('group_by', params.group_by)
  const qs = search.toString()
  const url = qs ? `/api/usage/summary?${qs}` : '/api/usage/summary'
  const result = await request<UsageSummary | { summaries: UsageSummary[] }>(url)
  if ('summaries' in result) {
    return result.summaries
  }
  return result
}

async function cleanupUsageLogs(retentionDays: number = 365): Promise<void> {
  await request('/api/usage/cleanup', {
    method: 'POST',
    headers: defaultHeaders,
    body: JSON.stringify({ retention_days: retentionDays }),
  })
}

// 环境变量 API
export interface EnvVarCategory {
  key: string
  label: string
  description: string
}

export interface EnvVarDefinition {
  name: string
  category: string
  default_value: string
  description: string
  is_secret: boolean
  current_value: string
  is_set: boolean
}

async function getEnvVarCategories(): Promise<EnvVarCategory[]> {
  const data = await request<{ data: EnvVarCategory[] }>('/api/envvars/categories')
  return data.data || []
}

async function getEnvVars(category?: string): Promise<EnvVarDefinition[]> {
  const url = category ? `/api/envvars?category=${encodeURIComponent(category)}` : '/api/envvars'
  const data = await request<{ data: EnvVarDefinition[] }>(url)
  return data.data || []
}

// 模型恢复 API
async function getModelRecovery(accountId?: string): Promise<{ total: number; items: any[] }> {
  const params = accountId ? `?account_id=${encodeURIComponent(accountId)}` : ''
  return request<{ total: number; items: any[] }>(`/api/model-recovery${params}`)
}

async function dismissModelRecovery(nodeId: string, modelId: string): Promise<{ status: string }> {
  return request<{ status: string }>(`/api/model-recovery/dismiss?node_id=${encodeURIComponent(nodeId)}&model_id=${encodeURIComponent(modelId)}`, {
    method: 'POST',
  })
}

async function setModelRecoveryNonRecoverable(nodeId: string, modelId: string, value: boolean): Promise<{ status: string }> {
  return request<{ status: string }>(`/api/model-recovery/non-recoverable?node_id=${encodeURIComponent(nodeId)}&model_id=${encodeURIComponent(modelId)}&value=${value ? 1 : 0}`, {
    method: 'POST',
  })
}

// 动态异常策略 API
async function getErrorPolicies(): Promise<{ data: import('../types').ErrorPolicySnapshot }> {
  return request<{ data: import('../types').ErrorPolicySnapshot }>('/api/error-policies')
}

async function toggleErrorPolicy(type: 'builtin' | 'observed', id: string, enabled: boolean): Promise<{ success: boolean }> {
  return request<{ success: boolean }>('/api/error-policies/toggle', {
    method: 'POST',
    headers: defaultHeaders,
    body: JSON.stringify({ type, id, enabled }),
  })
}

export default {
  login,
  logout,
  getSession,
  getAccounts,
  createAccount,
  updateAccount,
  deleteAccount,
  getNodes,
  getNode,
  createNode,
  updateNode,
  copyNode,
  deleteNode,
  importCCSwitchDB,
  exportCCSwitchDB,
  activateNode,
  toggleNode,
  getMonitorDashboard,
  getHealthHistory,
  createMonitorShare,
  getMonitorShares,
  revokeMonitorShare,
  getSharedMonitor,
  getConfig,
  updateConfig,
  getTunnel,
  saveTunnel,
  startTunnel,
  stopTunnel,
  listZones,
  getVersion,
  getChangelog,
  getNotificationChannels,
  createNotificationChannel,
  updateNotificationChannel,
  deleteNotificationChannel,
  getNotificationSubscriptions,
  createNotificationSubscriptions,
  updateNotificationSubscription,
  deleteNotificationSubscription,
  getEventTypes,
  testNotification,
  getClaudeConfigTemplate,
  // 定价和使用统计
  getPricingList,
  getPricing,
  savePricing,
  deletePricing,
  syncOfficialPricing,
  getUsageLogs,
  getUsageSummary,
  cleanupUsageLogs,
  // 环境变量
  getEnvVarCategories,
  getEnvVars,
  // 模型恢复
  getModelRecovery,
  dismissModelRecovery,
  setModelRecoveryNonRecoverable,
  getErrorPolicies,
  toggleErrorPolicy,
}
