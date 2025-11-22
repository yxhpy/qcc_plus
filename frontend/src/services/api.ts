import type { Account, Node, Config } from '../types'

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
}
