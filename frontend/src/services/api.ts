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
  ModelPricing,
  UsageLog,
  UsageSummary,
  UsageQueryParams,
} from '../types'

const defaultHeaders = { 'Content-Type': 'application/json' }

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

async function request<T>(url: string, options: RequestInit = {}): Promise<T> {
	const res = await fetch(url, { credentials: 'include', ...options })
	if (!res.ok) {
    let message = res.statusText
    try {
      const data = await res.json()
      message = (data as any).error || (data as any).message || message
    } catch (err) {
      /* ignore */
    }
    throw new Error(message || 'request failed')
  }
  // 204 No Content 不需要解析响应体
  if (res.status === 204) {
    return undefined as T
  }
	return parseJSON<T>(res)
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
    await getAccounts()
  } catch (err) {
    throw new Error('账号名称或密码错误')
  }
}

async function logout(): Promise<void> {
  await fetch('/logout', { method: 'POST', credentials: 'include', redirect: 'follow' })
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

async function createNode(payload: {
	name?: string
	base_url: string
	api_key?: string
	weight?: number
	health_check_method?: Node['health_check_method']
	health_check_model?: string
}, accountId?: string): Promise<string> {
	const data = await request<{ id: string }>(withAccount('/admin/api/nodes', accountId), {
		method: 'POST',
		headers: defaultHeaders,
		body: JSON.stringify(payload),
	})
	return data.id
}

async function updateNode(id: string, payload: Partial<Pick<Node, 'name' | 'base_url' | 'weight' | 'health_check_method' | 'health_check_model'>> & { api_key?: string }): Promise<void> {
	await request(`/admin/api/nodes?id=${encodeURIComponent(id)}`, {
		method: 'PUT',
		headers: defaultHeaders,
    body: JSON.stringify(payload),
  })
}

async function deleteNode(id: string): Promise<void> {
  await request(`/admin/api/nodes?id=${encodeURIComponent(id)}`, {
    method: 'DELETE',
  })
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
  return request<VersionInfo>('/version')
}

async function getChangelog(): Promise<string> {
  const res = await fetch('/changelog', { credentials: 'include' })
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

// 使用统计 API
async function getUsageLogs(params: UsageQueryParams = {}): Promise<{ logs: UsageLog[]; count: number }> {
  const search = new URLSearchParams()
  if (params.account_id) search.set('account_id', params.account_id)
  if (params.node_id) search.set('node_id', params.node_id)
  if (params.model_id) search.set('model_id', params.model_id)
  if (params.from) search.set('from', params.from)
  if (params.to) search.set('to', params.to)
  if (params.limit) search.set('limit', String(params.limit))
  if (params.offset) search.set('offset', String(params.offset))
  const qs = search.toString()
  const url = qs ? `/api/usage/logs?${qs}` : '/api/usage/logs'
  return request<{ logs: UsageLog[]; count: number }>(url)
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

export default {
  login,
  logout,
  getAccounts,
  createAccount,
  updateAccount,
  deleteAccount,
  getNodes,
  createNode,
  updateNode,
  deleteNode,
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
  getUsageLogs,
  getUsageSummary,
  cleanupUsageLogs,
}
