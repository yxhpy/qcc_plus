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
  return parseJSON<T>(res)
}

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
}, accountId?: string): Promise<string> {
  const data = await request<{ id: string }>(withAccount('/admin/api/nodes', accountId), {
    method: 'POST',
    headers: defaultHeaders,
    body: JSON.stringify(payload),
  })
  return data.id
}

async function updateNode(id: string, payload: Partial<Pick<Node, 'name' | 'base_url' | 'weight'>> & { api_key?: string }): Promise<void> {
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
  getConfig,
  updateConfig,
  getTunnel,
  saveTunnel,
  startTunnel,
  stopTunnel,
  listZones,
  getVersion,
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
}
