import { useEffect, useMemo, useState } from 'react'
import Card from '../components/Card'
import Modal from '../components/Modal'
import Toast from '../components/Toast'
import useDialog from '../hooks/useDialog'
import usePrompt from '../hooks/usePrompt'
import api from '../services/api'
import type { Account, Node } from '../types'
import './Nodes.css'

interface EditForm {
  name: string
  base_url: string
  weight: string
  api_key: string
  health_check_method: 'api' | 'head' | 'cli'
}

const healthMethodOptions: { value: 'api' | 'head' | 'cli'; label: string }[] = [
  { value: 'api', label: 'API 调用 (/v1/messages)' },
  { value: 'head', label: 'HEAD 请求' },
  { value: 'cli', label: 'Claude Code CLI (Docker)' },
]

export default function Nodes() {
  const [accounts, setAccounts] = useState<Account[]>([])
  const [accountId, setAccountId] = useState('')
  const [nodes, setNodes] = useState<Node[]>([])
  const [loading, setLoading] = useState(false)
  const [saving, setSaving] = useState(false)
  const [actionId, setActionId] = useState('')
  const [toast, setToast] = useState<{ message: string; type: 'success' | 'error' } | null>(null)
  const [search, setSearch] = useState('')
  const [filter, setFilter] = useState('all')
  const [detailNode, setDetailNode] = useState<Node | null>(null)
  const [editingNode, setEditingNode] = useState<Node | null>(null)
  const [editForm, setEditForm] = useState<EditForm>({ name: '', base_url: '', weight: '1', api_key: '', health_check_method: 'api' })
  const dialog = useDialog()
  const prompt = usePrompt()

  const showToast = (message: string, type: 'success' | 'error' = 'success') => {
    setToast({ message, type })
    setTimeout(() => setToast(null), 2200)
  }

  const loadAccounts = async () => {
    try {
      const list = await api.getAccounts()
      setAccounts(list)
      setAccountId((prev) => prev || (list[0]?.id ?? ''))
    } catch (err) {
      showToast('加载账号失败', 'error')
    }
  }

  const loadNodes = async () => {
    if (!accountId) return
    setLoading(true)
    try {
      const list = await api.getNodes(accountId)
      setNodes(list)
    } catch (err) {
      showToast('加载失败', 'error')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    loadAccounts()
  }, [])

  useEffect(() => {
    if (accountId) {
      loadNodes()
    }
  }, [accountId])

  useEffect(() => {
    if (!editingNode) return
    setEditForm({
      name: editingNode.name || '',
      base_url: editingNode.base_url || '',
      weight: String(editingNode.weight || 1),
      api_key: '',
      health_check_method: editingNode.health_check_method || 'api',
    })
  }, [editingNode])

  useEffect(() => {
    if (import.meta.env.DEV) {
      console.debug('[Nodes] detailNode changed', detailNode)
      console.debug('[Nodes] editingNode changed', editingNode)
    }
  }, [detailNode, editingNode])

  const filtered = useMemo(() => {
    const q = search.trim().toLowerCase()
    return nodes.filter((n) => {
      const match =
        !q || (n.name || '').toLowerCase().includes(q) || (n.base_url || '').toLowerCase().includes(q)
      if (!match) return false
      if (filter === 'active') return n.active && !n.failed && !n.disabled
      if (filter === 'failed') return n.failed
      if (filter === 'disabled') return n.disabled
      return true
    })
  }, [nodes, search, filter])

  const openAddModal = async () => {
    const result = await prompt.form({
      title: '新增节点',
      message: '填写节点信息，权重值越小优先级越高。',
      fields: [
        { name: 'name', label: '节点名称（可选）' },
        { name: 'base_url', label: 'Base URL', placeholder: 'https://api.anthropic.com', required: true },
        { name: 'api_key', label: 'API Key', placeholder: 'sk-...', type: 'password' },
        {
          name: 'health_check_method',
          label: '健康检查方式',
          type: 'select',
          defaultValue: 'api',
          options: healthMethodOptions,
        },
        {
          name: 'weight',
          label: '权重（值越小优先级越高）',
          type: 'number',
          defaultValue: '1',
          validate: (val) => {
            if (!val) return null
            const num = Number(val)
            if (!Number.isInteger(num) || num <= 0) return '权重需为正整数'
            return null
          },
        },
      ],
    })
    if (!result) return
    const weight = parseInt(result.weight || '1', 10)
    const healthMethod = (result.health_check_method as 'api' | 'head' | 'cli' | undefined) || 'api'
    const apiKey = (result.api_key || '').trim()
    if (requiresApiKey(healthMethod) && !apiKey) {
      showToast('选择 API/CLI 健康检查时需填写 API Key', 'error')
      return
    }
    try {
      await api.createNode(
        {
          name: (result.name || '').trim(),
          base_url: (result.base_url || '').trim(),
          api_key: apiKey || undefined,
          weight: Number.isNaN(weight) || weight <= 0 ? 1 : weight,
          health_check_method: healthMethod,
        },
        accountId
      )
      showToast('已新增节点')
      loadNodes()
    } catch (err) {
      showToast((err as Error).message || '新增失败', 'error')
    }
  }

  const handleAction = async (act: 'switch' | 'toggle' | 'del', node: Node) => {
    try {
      setActionId(node.id)
      if (act === 'switch') {
        if (node.active || node.disabled) return
        await api.activateNode(node.id)
        showToast('已切换')
        loadNodes()
        return
      }
      if (act === 'toggle') {
        await api.toggleNode(node.id, node.disabled)
        showToast(node.disabled ? '已启用' : '已禁用')
        loadNodes()
        return
      }
      if (act === 'del') {
        const ok = await dialog.confirm({ title: '确认删除', message: '确认删除该节点？' })
        if (!ok) return
        await api.deleteNode(node.id)
        showToast('已删除')
        loadNodes()
      }
    } catch (err) {
      showToast((err as Error).message || '操作失败', 'error')
    } finally {
      setActionId('')
    }
  }

  const submitEdit = async () => {
    if (!editingNode) return
    if (!editForm.base_url.trim()) {
      showToast('Base URL 必填', 'error')
      return
    }
    const weight = parseInt(editForm.weight || '1', 10)
    if (!Number.isInteger(weight) || weight <= 0) {
      showToast('权重需为正整数', 'error')
      return
    }
    const healthMethod = editForm.health_check_method || 'api'
    const apiKeyInput = editForm.api_key.trim()
    const hasKey = editingNode.has_api_key
    if (requiresApiKey(healthMethod) && !apiKeyInput && !hasKey) {
      showToast('选择 API/CLI 健康检查时需填写 API Key', 'error')
      return
    }
    setSaving(true)
    try {
      await api.updateNode(editingNode.id, {
        name: editForm.name.trim(),
        base_url: editForm.base_url.trim(),
        weight,
        api_key: apiKeyInput ? apiKeyInput : undefined,
        health_check_method: healthMethod,
      })
      showToast('已保存')
      setEditingNode(null)
      loadNodes()
    } catch (err) {
      showToast((err as Error).message || '保存失败', 'error')
    } finally {
      setSaving(false)
    }
  }

  const statusInfo = (n: Node) => {
    if (n.disabled) return { label: 'Disabled', cls: 'off', icon: '🚫' }
    if (n.failed) return { label: 'Failed', cls: 'fail', icon: '⚠️' }
    if (n.active) return { label: 'Active', cls: 'ok', icon: '✔️' }
    return { label: 'Standby', cls: 'warn', icon: '⏸' }
  }

  const healthClass = (health: number | null) => {
    if (health === null) return ''
    if (health >= 80) return 'health-good'
    if (health >= 50) return 'health-warn'
    return 'health-bad'
  }

  // 统一处理健康率，过滤掉 undefined/null/NaN，避免界面出现 NaN% 或渲染报错
  const parseHealthRate = (val?: number | null) => {
    if (val === undefined || val === null) return null
    const num = Number(val)
    return Number.isNaN(num) ? null : num
  }

  const formatHealthRate = (val?: number | null) => {
    const parsed = parseHealthRate(val)
    return parsed === null ? '-' : `${parsed.toFixed(1)}%`
  }

  const formatNumber = (val?: number) => {
    if (val === undefined || val === null) return '-'
    return val.toLocaleString()
  }

  const formatDateTime = (val?: string | null) => {
    if (!val) return '从未检查'
    const date = new Date(val)
    if (Number.isNaN(date.getTime())) return '从未检查'
    return date.toLocaleString('zh-CN')
  }

  const formatHealthMethod = (val?: 'api' | 'head' | 'cli') => {
    if (val === 'head') return 'HEAD'
    if (val === 'cli') return 'CLI'
    return 'API'
  }

  const requiresApiKey = (method?: 'api' | 'head' | 'cli') => method === 'api' || method === 'cli'

  const renderStat = (label: string, value: string | number | undefined) => (
    <div className="stat-item">
      <div className="stat-label">{label}</div>
      <div className="stat-value">{value ?? '-'}</div>
    </div>
  )

  const openErrorDetail = (node: Node) => {
    if (!node.last_error) {
      showToast('暂无错误详情', 'error')
      return
    }
    setDetailNode(node)
  }

  return (
    <div className="nodes-page">
      <div className="nodes-header">
        <h1>节点管理</h1>
        <p className="sub">新增 / 编辑 / 切换节点，并查看健康状态与统计。</p>
      </div>

      <Card>
        <div className="toolbar">
          <label style={{ minWidth: 220 }}>
            选择账号
            <select value={accountId} onChange={(e) => setAccountId(e.target.value)}>
              {accounts.map((a) => (
                <option key={a.id} value={a.id}>
                  {a.name}
                  {a.is_admin ? ' [管]' : ''}
                </option>
              ))}
            </select>
          </label>
          <div className="spacer" />
          <button className="btn ghost" type="button" onClick={loadNodes} disabled={loading}>
            刷新
          </button>
          <button className="btn primary" type="button" onClick={openAddModal}>
            ➕ 新增节点
          </button>
        </div>
      </Card>

      <Card>
        <div className="toolbar">
          <input
            id="search"
            placeholder="搜索名称或 Base URL"
            style={{ minWidth: 240 }}
            value={search}
            onChange={(e) => setSearch(e.target.value)}
          />
          <select id="filter" value={filter} onChange={(e) => setFilter(e.target.value)}>
            <option value="all">全部状态</option>
            <option value="active">仅活跃</option>
            <option value="failed">仅失败</option>
            <option value="disabled">已禁用</option>
          </select>
        </div>

        <div className="table-wrapper">
          <table>
            <thead>
              <tr>
                <th>名称</th>
                <th>Base URL</th>
                <th>状态</th>
                <th>健康率</th>
                <th title="值越小优先级越高">权重</th>
                <th>请求数</th>
                <th style={{ minWidth: 230 }}>操作</th>
              </tr>
            </thead>
            <tbody>
              {loading ? (
                <tr>
                  <td colSpan={7}>加载中...</td>
                </tr>
              ) : filtered.length === 0 ? (
                <tr>
                  <td colSpan={7}>暂无节点</td>
                </tr>
              ) : (
                filtered.map((n) => {
                  const health = parseHealthRate(n.health_rate)
                  const status = statusInfo(n)
                  return (
                    <tr key={n.id}>
                      <td>{n.name || '未命名'}</td>
                      <td>
                        <div className="url-cell" title={n.base_url || '-'}>
                          {n.base_url || '-'}
                        </div>
                      </td>
                      <td>
                        <div
                          className={`pill ${status.cls}`}
                          style={{ cursor: n.failed && n.last_error ? 'pointer' : 'default' }}
                          onClick={() => (n.failed ? openErrorDetail(n) : undefined)}
                        >
                          <span>{status.icon}</span>
                          <span>{status.label}</span>
                        </div>
                      </td>
                      <td>
                        {health === null ? (
                          '-'
                        ) : (
                          <span className={healthClass(health)}>{health.toFixed(1)}%</span>
                        )}
                      </td>
                      <td title="值越小优先级越高">{n.weight || 1}</td>
                      <td>{n.requests ?? 0}</td>
                      <td>
                        <div className="table-actions" style={{ rowGap: 6 }}>
                          <div style={{ display: 'flex', gap: 6, flexWrap: 'wrap' }}>
                            {!n.active && !n.disabled && (
                              <button
                                className="btn ghost"
                                type="button"
                                onClick={() => handleAction('switch', n)}
                                disabled={actionId === n.id}
                              >
                                切换
                              </button>
                            )}
                            <button className="btn ghost" type="button" onClick={() => setEditingNode(n)}>
                              编辑
                            </button>
                            <button className="btn ghost" type="button" onClick={() => setDetailNode(n)}>
                              查看详情
                            </button>
                          </div>
                          <div style={{ display: 'flex', gap: 6, flexWrap: 'wrap' }}>
                            <button
                              className="btn warn"
                              type="button"
                              onClick={() => handleAction('toggle', n)}
                              disabled={actionId === n.id}
                            >
                              {n.disabled ? '启用' : '禁用'}
                            </button>
                            <button
                              className="btn danger"
                              type="button"
                              onClick={() => handleAction('del', n)}
                              disabled={actionId === n.id}
                            >
                              删除
                            </button>
                          </div>
                        </div>
                      </td>
                    </tr>
                  )
                })
              )}
            </tbody>
          </table>
        </div>
      </Card>

      <Modal
        open={!!detailNode}
        title="节点详情"
        onClose={() => setDetailNode(null)}
        footer={
          <div className="dialog-actions">
            <button className="btn ghost" type="button" onClick={() => setDetailNode(null)}>
              关闭
            </button>
          </div>
        }
      >
        {detailNode && (
          <div>
            <div className="node-stats">
              {renderStat('名称', detailNode.name || '未命名')}
              {renderStat('Base URL', detailNode.base_url || '-')}
              {renderStat('健康检查', formatHealthMethod(detailNode.health_check_method))}
              {renderStat('权重', detailNode.weight ?? '-')} {renderStat('状态', statusInfo(detailNode).label)}
            </div>
            <div className="node-stats">
              {renderStat('最后健康检查', formatDateTime(detailNode.last_health_check_at))}
              {renderStat('Ping 延迟 (ms)', detailNode.last_ping_ms ?? '-')}
              {detailNode.last_ping_error && (
                <div className="stat-item" style={{ gridColumn: '1 / -1' }}>
                  <div className="stat-label">Ping 错误</div>
                  <div className="stat-value" style={{ color: 'var(--color-danger)' }}>
                    {detailNode.last_ping_error}
                  </div>
                </div>
              )}
            </div>
            <div className="node-stats">
              {renderStat('请求数', formatNumber(detailNode.requests))}
              {renderStat('失败数', formatNumber(detailNode.fail_count))}
              {renderStat('连续失败', formatNumber(detailNode.fail_streak))}
              {renderStat('健康率', formatHealthRate(detailNode.health_rate))}
            </div>
            <div className="node-stats">
              {renderStat('总流量(bytes)', formatNumber(detailNode.total_bytes))}
              {renderStat('流耗时(ms)', formatNumber(detailNode.stream_dur_ms))}
              {renderStat('input_tokens', formatNumber(detailNode.input_tokens))}
              {renderStat('output_tokens', formatNumber(detailNode.output_tokens))}
            </div>
            {detailNode.last_error && (
              <div className="error-detail">
                <div style={{ fontWeight: 700, marginBottom: 4 }}>最后错误</div>
                {detailNode.last_error}
              </div>
            )}
          </div>
        )}
      </Modal>

      <Modal
        open={!!editingNode}
        title="编辑节点"
        onClose={() => (!saving ? setEditingNode(null) : null)}
        footer={
          <div className="dialog-actions">
            <button className="btn ghost" type="button" onClick={() => (!saving ? setEditingNode(null) : null)}>
              取消
            </button>
            <button className="btn primary" type="button" onClick={submitEdit} disabled={saving}>
              保存
            </button>
          </div>
        }
      >
        {editingNode && (
          <div className="prompt-form">
            <div className="prompt-grid">
              <label>
                节点名称
                <input
                  value={editForm.name}
                  onChange={(e) => setEditForm((prev) => ({ ...prev, name: e.target.value }))}
                  placeholder="如：联通-北京"
                />
              </label>
              <label>
                Base URL
                <input
                  value={editForm.base_url}
                  onChange={(e) => setEditForm((prev) => ({ ...prev, base_url: e.target.value }))}
                  placeholder="https://api.anthropic.com"
                  required
                />
              </label>
              <label>
                权重
                <input
                  type="number"
                  min={1}
                  value={editForm.weight}
                  onChange={(e) => setEditForm((prev) => ({ ...prev, weight: e.target.value }))}
                />
                <span className="weight-hint">值越小优先级越高</span>
              </label>
              <label>
                健康检查方式
                <select
                  value={editForm.health_check_method}
                  onChange={(e) =>
                    setEditForm((prev) => ({ ...prev, health_check_method: e.target.value as EditForm['health_check_method'] }))
                  }
                >
                  {healthMethodOptions.map((opt) => (
                    <option key={opt.value} value={opt.value}>
                      {opt.label}
                    </option>
                  ))}
                </select>
                <span className="weight-hint">API/CLI 需要有效的 API Key，CLI 需 Docker</span>
              </label>
              <label>
                API Key（留空不改）
                <input
                  type="password"
                  value={editForm.api_key}
                  onChange={(e) => setEditForm((prev) => ({ ...prev, api_key: e.target.value }))}
                  placeholder="sk-..."
                  autoComplete="off"
                />
                {requiresApiKey(editForm.health_check_method) && !editForm.api_key.trim() && !editingNode.has_api_key && (
                  <span className="weight-hint" style={{ color: 'var(--color-danger)' }}>
                    当前方式需要 API Key，留空将导致健康检查失败
                  </span>
                )}
              </label>
            </div>

            <div className="node-stats">
              {renderStat('请求数', formatNumber(editingNode.requests))}
              {renderStat('失败数', formatNumber(editingNode.fail_count))}
              {renderStat('连续失败', formatNumber(editingNode.fail_streak))}
              {renderStat('健康率', formatHealthRate(editingNode.health_rate))}
              {renderStat('总流量(bytes)', formatNumber(editingNode.total_bytes))}
              {renderStat('流耗时(ms)', formatNumber(editingNode.stream_dur_ms))}
              {renderStat('input_tokens', formatNumber(editingNode.input_tokens))}
              {renderStat('output_tokens', formatNumber(editingNode.output_tokens))}
            </div>
            {editingNode.last_error && (
              <div className="error-detail">
                <div style={{ fontWeight: 700, marginBottom: 4 }}>最后错误</div>
                {editingNode.last_error}
              </div>
            )}
          </div>
        )}
      </Modal>

      <Toast message={toast?.message} type={toast?.type} />
    </div>
  )
}
