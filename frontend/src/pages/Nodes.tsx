import { useCallback, useEffect, useMemo, useState } from 'react'
import { closestCenter, DndContext, PointerSensor, useSensor, useSensors, type DragEndEvent, type DragStartEvent } from '@dnd-kit/core'
import { arrayMove, SortableContext, useSortable, verticalListSortingStrategy } from '@dnd-kit/sortable'
import { CSS } from '@dnd-kit/utilities'
import Card from '../components/Card'
import Modal from '../components/Modal'
import RecoveryBadge from '../components/RecoveryBadge'
import Toast from '../components/Toast'
import useDialog from '../hooks/useDialog'
import usePrompt from '../hooks/usePrompt'
import api from '../services/api'
import type { Account, Node } from '../types'
import { formatBeijingTime, parseToDate } from '../utils/date'
import './Nodes.css'

interface EditForm {
  name: string
  base_url: string
  weight: string
  api_key: string
  health_check_method: 'api' | 'head' | 'cli'
  health_check_model: string
  model_mapping: Array<{ from: string; to: string }>
}

const healthMethodOptions: { value: 'api' | 'head' | 'cli'; label: string }[] = [
  { value: 'api', label: 'API 调用 (/v1/messages)' },
  { value: 'head', label: 'HEAD 请求' },
  { value: 'cli', label: 'Claude Code CLI (Docker)' },
]

const modelOptions = [
  { value: 'claude-haiku-4-5-20251001', label: 'Claude Haiku 4.5（推荐，最便宜）' },
  { value: 'claude-sonnet-4-5-20250929', label: 'Claude Sonnet 4.5' },
  { value: 'claude-opus-4-20250514', label: 'Claude Opus 4（不推荐，最贵）' },
]

const allModelIds = [
  'claude-haiku-4-5-20251001',
  'claude-sonnet-4-20250514',
  'claude-sonnet-4-5-20250929',
  'claude-opus-4-20250514',
  'claude-opus-4-6-20250918',
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
  const [editForm, setEditForm] = useState<EditForm>({ name: '', base_url: '', weight: '1', api_key: '', health_check_method: 'api', health_check_model: 'claude-haiku-4-5-20251001', model_mapping: [] })
  const [savingOrder, setSavingOrder] = useState(false)
  const [draggingId, setDraggingId] = useState<string | null>(null)
  const sensors = useSensors(useSensor(PointerSensor, { activationConstraint: { distance: 6 } }))
  const dialog = useDialog()
  const prompt = usePrompt()

  const showToast = (message: string, type: 'success' | 'error' = 'success') => {
    setToast({ message, type })
    setTimeout(() => setToast(null), 2200)
  }

  const toTimestamp = (val?: string | number | Date | null) => {
    const d = parseToDate(val)
    return d ? d.getTime() : 0
  }

  const sortByOrder = useCallback(
    (list: Node[]) => {
      return list
        .slice()
        .sort((a, b) => {
          const wa = a.weight ?? 0
          const wb = b.weight ?? 0
          if (wa !== wb) return wa - wb
          const ta = toTimestamp(a.created_at ?? null)
          const tb = toTimestamp(b.created_at ?? null)
          return ta - tb
        })
    },
    [toTimestamp],
  )

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
      setNodes(sortByOrder(list))
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
    const mapping: Array<{ from: string; to: string }> = []
    if (editingNode.model_mapping) {
      for (const [from, to] of Object.entries(editingNode.model_mapping)) {
        mapping.push({ from, to })
      }
    }
    setEditForm({
      name: editingNode.name || '',
      base_url: editingNode.base_url || '',
      weight: String(editingNode.weight || 1),
      api_key: '',
      health_check_method: editingNode.health_check_method || 'api',
      health_check_model: editingNode.health_check_model || 'claude-haiku-4-5-20251001',
      model_mapping: mapping,
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
      if (filter === 'online') return n.status === 'online'
      if (filter === 'offline') return n.status === 'offline'
      if (filter === 'degraded') return n.status === 'degraded'
      if (filter === 'disabled') return n.status === 'disabled'
      return true
    })
  }, [nodes, search, filter])

  const openAddModal = async () => {
    const result = await prompt.form({
      title: '新增节点',
      message: '填写节点信息，权重值越小优先级越高。',
      size: 'lg',
      fields: [
        { name: 'name', label: '节点名称（可选）' },
        { name: 'base_url', label: 'Base URL', placeholder: 'https://api.anthropic.com', required: true },
        { name: 'api_key', label: 'API Key（多个用逗号分隔）', placeholder: 'sk-key1 或 sk-key1,sk-key2,sk-key3', type: 'password' },
        {
          name: 'health_check_method',
          label: '健康检查方式',
          type: 'select',
          defaultValue: 'api',
          options: healthMethodOptions,
        },
        {
          name: 'health_check_model',
          label: 'CLI 健康检查模型（仅 CLI 方式有效）',
          type: 'select',
          defaultValue: 'claude-haiku-4-5-20251001',
          options: modelOptions,
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
    const healthModel = (result.health_check_model as string) || 'claude-haiku-4-5-20251001'
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
          health_check_model: healthModel,
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
        if (node.is_active || node.status === 'disabled') return
        await api.activateNode(node.id)
        showToast('已切换')
        loadNodes()
        return
      }
      if (act === 'toggle') {
        await api.toggleNode(node.id, node.status === 'disabled')
        showToast(node.status === 'disabled' ? '已启用' : '已禁用')
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
    const healthModel = editForm.health_check_model || 'claude-haiku-4-5-20251001'
    const apiKeyInput = editForm.api_key.trim()
    const hasKey = editingNode.has_api_key
    if (requiresApiKey(healthMethod) && !apiKeyInput && !hasKey) {
      showToast('选择 API/CLI 健康检查时需填写 API Key', 'error')
      return
    }
    setSaving(true)
    try {
      // 将 model_mapping 数组转为 Record
      const mappingObj: Record<string, string> = {}
      for (const entry of editForm.model_mapping) {
        const from = entry.from.trim()
        const to = entry.to.trim()
        if (from && to && from !== to) {
          mappingObj[from] = to
        }
      }
      await api.updateNode(editingNode.id, {
        name: editForm.name.trim(),
        base_url: editForm.base_url.trim(),
        weight,
        api_key: apiKeyInput ? apiKeyInput : undefined,
        health_check_method: healthMethod,
        health_check_model: healthModel,
        model_mapping: mappingObj,
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
    switch (n.status) {
      case 'disabled': return { label: 'Disabled', cls: 'off' }
      case 'offline':  return { label: 'Offline', cls: 'fail' }
      case 'degraded': return { label: 'Degraded', cls: 'warn' }
      case 'online':
      default:         return { label: 'Online', cls: 'ok' }
    }
  }

  const errorSeverityLabel = (severity?: string) => {
    switch (severity) {
      case 'key_invalid': return 'Key 失效'
      case 'account_issue': return '账号问题'
      case 'node_down': return '节点宕机'
      case 'degraded': return '性能降级'
      case 'transient': return '临时错误'
      case 'permanent': return '请求错误'
      default: return ''
    }
  }

  const errorSeverityClass = (severity?: string) => {
    switch (severity) {
      case 'key_invalid':
      case 'account_issue': return 'severity-danger'
      case 'node_down': return 'severity-danger'
      case 'degraded': return 'severity-warn'
      default: return ''
    }
  }

  const formatKeyStatus = (n: Node) => {
    if (!n.key_count || n.key_count <= 1) {
      return n.has_api_key ? '✓' : '-'
    }
    return `${n.active_key_count ?? 0}/${n.key_count}`
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
    const formatted = formatBeijingTime(val)
    return formatted === '--' ? '从未检查' : formatted
  }

  const formatHealthMethod = (val?: 'api' | 'head' | 'cli') => {
    if (val === 'head') return 'HEAD'
    if (val === 'cli') return 'CLI'
    return 'API'
  }

  const requiresApiKey = (method?: 'api' | 'head' | 'cli') => method === 'api' || method === 'cli'

  const handleDragStart = (event: DragStartEvent) => {
    setDraggingId(String(event.active.id))
  }

  const handleDragCancel = () => setDraggingId(null)

  const handleDragEnd = (event: DragEndEvent) => {
    setDraggingId(null)
    if (savingOrder) return // 防止保存中重复拖拽
    const { active, over } = event
    if (!over || active.id === over.id) return
    const activeId = String(active.id)
    const overId = String(over.id)
    const oldIndex = nodes.findIndex((n) => n.id === activeId)
    const newIndex = nodes.findIndex((n) => n.id === overId)
    if (oldIndex === -1 || newIndex === -1) return
    const prevNodes = [...nodes]
    const reordered = arrayMove(nodes, oldIndex, newIndex)
    const withWeights = reordered.map((n, idx) => ({
      ...n,
      weight: idx + 1,
    }))
    setNodes(withWeights)
    setSavingOrder(true)

    // 异步保存放在 setTimeout 中，避免阻塞 DnD 生命周期
    setTimeout(() => {
      Promise.all(
        withWeights.map((n, idx) =>
          api.updateNode(n.id, {
            name: n.name || '',
            base_url: n.base_url,
            weight: idx + 1,
            health_check_method: n.health_check_method || 'api',
            health_check_model: n.health_check_model || 'claude-haiku-4-5-20251001',
          }),
        ),
      )
        .then(() => {
          showToast('排序已保存')
        })
        .catch((err) => {
          setNodes(prevNodes)
          showToast((err as Error).message || '保存排序失败', 'error')
        })
        .finally(() => {
          setSavingOrder(false)
        })
    }, 0)
  }

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

  const NodeRow = ({ node }: { node: Node }) => {
    const health = parseHealthRate(node.health_rate)
    const status = statusInfo(node)
    const { attributes, listeners, setNodeRef, setActivatorNodeRef, transform, transition, isDragging } = useSortable({
      id: node.id,
      disabled: loading,
    })
    const style = {
      transform: CSS.Transform.toString(transform),
      transition,
      position: 'relative' as const,
      zIndex: isDragging ? 2 : 1,
    }
    const dragging = isDragging || draggingId === node.id

    return (
      <tr ref={setNodeRef} style={style} className={dragging ? 'dragging' : ''}>
        <td className="drag-handle-cell">
          <button
            type="button"
            className="drag-handle"
            {...attributes}
            {...listeners}
            ref={setActivatorNodeRef}
            disabled={loading}
            aria-label="拖拽排序"
            title="拖拽排序"
          >
            ⋮⋮
          </button>
        </td>
        <td className="node-name-cell">{node.name || '未命名'}<RecoveryBadge nodeId={node.id} /></td>
        <td>
          <div style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
            <div
              className={`pill ${status.cls}`}
              style={{ cursor: (node.status === 'offline' || node.status === 'degraded') && node.last_error ? 'pointer' : 'default' }}
              onClick={() => ((node.status === 'offline' || node.status === 'degraded') ? openErrorDetail(node) : undefined)}
            >
              <span>{status.label}</span>
            </div>
            {node.is_active && <span className="active-badge" title="当前选中节点">IN USE</span>}
          </div>
          {node.error_severity && errorSeverityLabel(node.error_severity) && (
            <div className={`error-severity-tag ${errorSeverityClass(node.error_severity)}`}>
              {errorSeverityLabel(node.error_severity)}
            </div>
          )}
        </td>
        <td>{formatHealthMethod(node.health_check_method)}</td>
        <td>{node.last_ping_ms == null ? '-' : `${node.last_ping_ms}ms`}</td>
        <td>
          {health === null ? (
            '-'
          ) : (
            <span className={healthClass(health)}>{health.toFixed(1)}%</span>
          )}
        </td>
        <td>{`${node.requests ?? 0}/${node.fail_count ?? 0}`}</td>
        <td>
          <span className={node.key_count && node.key_count > 1 && node.active_key_count === 0 ? 'key-status-danger' : ''}>
            {formatKeyStatus(node)}
          </span>
          {(node.active_conns ?? 0) > 0 && (
            <span className="active-conns-badge" title="活跃连接数">{node.active_conns}</span>
          )}
        </td>
        <td>
          <div className="table-actions" style={{ rowGap: 6 }}>
            <div style={{ display: 'flex', gap: 6, flexWrap: 'wrap' }}>
              {!node.is_active && node.status !== 'disabled' && (
                <button
                  className="btn ghost"
                  type="button"
                  onClick={() => handleAction('switch', node)}
                  disabled={actionId === node.id}
                >
                  切换
                </button>
              )}
              <button className="btn ghost" type="button" onClick={() => setEditingNode(node)}>
                编辑
              </button>
              <button className="btn ghost" type="button" onClick={() => setDetailNode(node)}>
                查看详情
              </button>
            </div>
            <div style={{ display: 'flex', gap: 6, flexWrap: 'wrap' }}>
              <button
                className="btn warn"
                type="button"
                onClick={() => handleAction('toggle', node)}
                disabled={actionId === node.id}
              >
                {node.status === 'disabled' ? '启用' : '禁用'}
              </button>
              <button
                className="btn danger"
                type="button"
                onClick={() => handleAction('del', node)}
                disabled={actionId === node.id}
              >
                删除
              </button>
            </div>
          </div>
        </td>
      </tr>
    )
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
            <option value="online">在线</option>
            <option value="offline">离线</option>
            <option value="degraded">降级</option>
            <option value="disabled">已禁用</option>
          </select>
        </div>
        <div className="muted" style={{ margin: '-8px 0 12px', fontSize: 12 }}>
          拖拽左侧手柄调整节点顺序，自动保存{savingOrder ? '中…' : ''}
        </div>

        <div className="table-wrapper">
          <table>
            <thead>
              <tr>
                <th style={{ width: 54 }}>排序</th>
                <th>名称</th>
                <th>状态</th>
                <th>检查方式</th>
                <th>延迟</th>
                <th>成功率</th>
                <th>请求/失败</th>
                <th>Key/连接</th>
                <th style={{ minWidth: 200 }}>操作</th>
              </tr>
            </thead>
            <DndContext
              sensors={sensors}
              collisionDetection={closestCenter}
              onDragStart={handleDragStart}
              onDragEnd={handleDragEnd}
              onDragCancel={handleDragCancel}
            >
              <SortableContext items={filtered.map((n) => n.id)} strategy={verticalListSortingStrategy}>
                <tbody>
                  {loading ? (
                    <tr>
                      <td colSpan={9}>加载中...</td>
                    </tr>
                  ) : filtered.length === 0 ? (
                    <tr>
                      <td colSpan={9}>暂无节点</td>
                    </tr>
                  ) : (
                    filtered.map((n) => <NodeRow key={n.id} node={n} />)
                  )}
                </tbody>
              </SortableContext>
            </DndContext>
          </table>
        </div>
      </Card>

      <Modal
        open={!!detailNode}
        title="节点详情"
        onClose={() => setDetailNode(null)}
        size="lg"
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
              {detailNode.health_check_method === 'cli' &&
                renderStat('健康检查模型', detailNode.health_check_model || 'claude-haiku-4-5-20251001')}
              {renderStat('权重', detailNode.weight ?? '-')} {renderStat('状态', statusInfo(detailNode).label)}
              {detailNode.is_active && renderStat('选中', '当前活跃节点')}
            </div>
            {detailNode.model_mapping && Object.keys(detailNode.model_mapping).length > 0 && (
              <div style={{ margin: '8px 0' }}>
                <div style={{ fontWeight: 600, fontSize: 13, marginBottom: 4 }}>模型映射</div>
                {Object.entries(detailNode.model_mapping).map(([from, to]) => (
                  <div key={from} style={{ fontSize: 12, padding: '2px 0', color: 'var(--text)' }}>
                    {from} &rarr; {to}
                  </div>
                ))}
              </div>
            )}
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
        size="lg"
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
                CLI 健康检查模型
                <select
                  value={editForm.health_check_model}
                  onChange={(e) =>
                    setEditForm((prev) => ({ ...prev, health_check_model: e.target.value }))
                  }
                  disabled={editForm.health_check_method !== 'cli'}
                >
                  {modelOptions.map((opt) => (
                    <option key={opt.value} value={opt.value}>
                      {opt.label}
                    </option>
                  ))}
                </select>
                {editForm.health_check_method !== 'cli' && (
                  <span className="weight-hint">仅在 CLI 健康检查方式下生效</span>
                )}
              </label>
              <label>
                API Key（留空不改，多个用逗号分隔）
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

            <div style={{ marginTop: 12 }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 6 }}>
                <span style={{ fontWeight: 600, fontSize: 13 }}>模型映射</span>
                <button
                  type="button"
                  className="btn ghost"
                  style={{ padding: '2px 8px', fontSize: 12, lineHeight: '20px' }}
                  onClick={() =>
                    setEditForm((prev) => ({
                      ...prev,
                      model_mapping: [...prev.model_mapping, { from: '', to: '' }],
                    }))
                  }
                >
                  + 添加映射
                </button>
                <span className="weight-hint" style={{ marginLeft: 4 }}>请求中的模型自动替换为目标模型</span>
              </div>
              {editForm.model_mapping.map((entry, idx) => (
                <div key={idx} style={{ display: 'flex', alignItems: 'center', gap: 6, marginBottom: 4 }}>
                  <select
                    value={entry.from}
                    onChange={(e) => {
                      const val = e.target.value
                      setEditForm((prev) => {
                        const arr = [...prev.model_mapping]
                        arr[idx] = { ...arr[idx], from: val }
                        return { ...prev, model_mapping: arr }
                      })
                    }}
                    style={{ flex: 1, fontSize: 12, padding: '4px 6px' }}
                  >
                    <option value="">-- 源模型 --</option>
                    {allModelIds.map((m) => (
                      <option key={m} value={m}>{m}</option>
                    ))}
                  </select>
                  <span style={{ fontSize: 13, color: 'var(--text-secondary)', flexShrink: 0 }}>&rarr;</span>
                  <select
                    value={entry.to}
                    onChange={(e) => {
                      const val = e.target.value
                      setEditForm((prev) => {
                        const arr = [...prev.model_mapping]
                        arr[idx] = { ...arr[idx], to: val }
                        return { ...prev, model_mapping: arr }
                      })
                    }}
                    style={{ flex: 1, fontSize: 12, padding: '4px 6px' }}
                  >
                    <option value="">-- 目标模型 --</option>
                    {allModelIds.map((m) => (
                      <option key={m} value={m}>{m}</option>
                    ))}
                  </select>
                  <button
                    type="button"
                    className="btn ghost"
                    style={{ padding: '2px 6px', fontSize: 12, color: 'var(--color-danger)', flexShrink: 0 }}
                    onClick={() =>
                      setEditForm((prev) => ({
                        ...prev,
                        model_mapping: prev.model_mapping.filter((_, i) => i !== idx),
                      }))
                    }
                  >
                    &times;
                  </button>
                </div>
              ))}
              {editForm.model_mapping.length === 0 && (
                <div style={{ fontSize: 12, color: 'var(--text-secondary)', padding: '4px 0' }}>
                  暂无映射规则，点击"+ 添加映射"创建
                </div>
              )}
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
