import { useCallback, useEffect, useMemo, useState } from 'react'
import { closestCenter, DndContext, PointerSensor, useSensor, useSensors, type DragEndEvent, type DragStartEvent } from '@dnd-kit/core'
import { arrayMove, SortableContext, useSortable, verticalListSortingStrategy } from '@dnd-kit/sortable'
import { CSS } from '@dnd-kit/utilities'
import Card from '../components/Card'
import ErrorDetailBlock from '../components/errors/ErrorDetailBlock'
import Modal from '../components/Modal'
import RecoveryBadge from '../components/RecoveryBadge'
import { allKnownModelIds, defaultHealthCheckModels, healthCheckModelOptions, normalizeHealthCheckModel, type SupportedSourceProtocol } from '../config/modelCatalog'
import Toast from '../components/Toast'
import useDialog from '../hooks/useDialog'
import api from '../services/api'
import type { Account, CCSwitchImportSummary, Node, NodeAPIKey } from '../types'
import { formatBeijingTime, parseToDate } from '../utils/date'
import './Nodes.css'

interface EditKeyForm {
  id: string
  name: string
  key: string
}

interface EditMappingForm {
  id: string
  from: string
  to: string
}

interface EditForm {
  name: string
  base_url: string
  weight: string
  api_keys: EditKeyForm[]
  health_check_method: 'api' | 'head' | 'cli'
  health_check_model: string
  source_protocol: 'claude' | 'openai' | 'gemini'
  auth_profile: string
  capabilities: string
  model_mapping: EditMappingForm[]
}

interface CCSwitchImportForm {
  file: File | null
  import_providers: boolean
  import_pricing: boolean
  import_logs: boolean
  weight_offset: string
}

type ProtocolTab = 'claude' | 'openai' | 'gemini'
type SourceProtocol = EditForm['source_protocol']

const healthMethodOptions: { value: 'api' | 'head' | 'cli'; label: string }[] = [
  { value: 'api', label: 'API 调用 (/v1/messages)' },
  { value: 'head', label: 'HEAD 请求' },
  { value: 'cli', label: 'Claude Code CLI (Docker)' },
]

const createFormRowID = () => `${Date.now()}-${Math.random().toString(36).slice(2, 8)}`

const makeKeyRow = (name = '', key = ''): EditKeyForm => ({
  id: createFormRowID(),
  name,
  key,
})

const makeMappingRow = (from = '', to = ''): EditMappingForm => ({
  id: createFormRowID(),
  from,
  to,
})

const defaultImportForm = (): CCSwitchImportForm => ({
  file: null,
  import_providers: true,
  import_pricing: true,
  import_logs: true,
  weight_offset: '1000',
})

const protocolLockedHealthMethod = (protocol: ProtocolTab | SourceProtocol) => {
  if (protocol === 'openai' || protocol === 'gemini') return 'api' as const
  return null
}

const requiresApiKey = (method?: 'api' | 'head' | 'cli') => method === 'api' || method === 'cli'

const getDefaultHealthCheckModel = (protocol?: ProtocolTab | SourceProtocol) => {
  return defaultHealthCheckModels[(protocol || 'claude') as SupportedSourceProtocol] || defaultHealthCheckModels.claude
}

const getHealthCheckModelOptions = (protocol?: ProtocolTab | SourceProtocol) => {
  switch (protocol) {
    case 'openai':
      return healthCheckModelOptions.openai
    case 'gemini':
      return healthCheckModelOptions.gemini
    default:
      return healthCheckModelOptions.claude
  }
}

const getDisplayHealthCheckModel = (protocol?: ProtocolTab | SourceProtocol, model?: string) => {
  return normalizeHealthCheckModel((protocol || 'claude') as SupportedSourceProtocol, model)
}

const displayNodeKeyName = (nodeName: string, keyName: string) => {
  const trimmedNode = nodeName.trim()
  const trimmedKey = keyName.trim()
  if (!trimmedKey) return trimmedNode
  if (!trimmedNode) return trimmedKey
  return `${trimmedNode}-${trimmedKey}`
}

const buildEmptyEditForm = (protocol: SourceProtocol = 'claude'): EditForm => ({
  name: '',
  base_url: '',
  weight: '1',
  api_keys: [makeKeyRow()],
  health_check_method: protocolLockedHealthMethod(protocol) || 'api',
  health_check_model: getDefaultHealthCheckModel(protocol),
  source_protocol: protocol,
  auth_profile: '',
  capabilities: '',
  model_mapping: [],
})

const buildEditFormFromNode = (node: Node): EditForm => {
  const protocol = (node.source_protocol || 'claude') as SourceProtocol
  const mapping = Object.entries(node.model_mapping || {}).map(([from, to]) => makeMappingRow(from, to))
  const apiKeys = (node.api_keys && node.api_keys.length > 0 ? node.api_keys : node.api_key ? [{ name: '', key: node.api_key }] : [])
    .map((item) => makeKeyRow(item.name || '', item.key || ''))

  return {
    name: node.name || '',
    base_url: node.base_url || '',
    weight: String(node.weight || 1),
    api_keys: apiKeys.length > 0 ? apiKeys : [makeKeyRow()],
    health_check_method: protocolLockedHealthMethod(protocol) || node.health_check_method || 'api',
    health_check_model: getDisplayHealthCheckModel(protocol, node.health_check_model),
    source_protocol: protocol,
    auth_profile: node.auth_profile || '',
    capabilities: node.capabilities || '',
    model_mapping: mapping,
  }
}

const normalizeApiKeys = (items: EditKeyForm[]): NodeAPIKey[] =>
  items
    .map((item) => ({ name: item.name.trim(), key: item.key.trim() }))
    .filter((item) => item.key)

export default function Nodes() {
  const [accounts, setAccounts] = useState<Account[]>([])
  const [accountId, setAccountId] = useState('')
  const [nodes, setNodes] = useState<Node[]>([])
  const [loading, setLoading] = useState(false)
  const [saving, setSaving] = useState(false)
  const [actionId, setActionId] = useState('')
  const [detailLoadingId, setDetailLoadingId] = useState('')
  const [exporting, setExporting] = useState(false)
  const [toast, setToast] = useState<{ message: string; type: 'success' | 'error' } | null>(null)
  const [search, setSearch] = useState('')
  const [filter, setFilter] = useState('all')
  const [protocolTab, setProtocolTab] = useState<ProtocolTab>('claude')
  const [detailNode, setDetailNode] = useState<Node | null>(null)
  const [editingNode, setEditingNode] = useState<Node | null>(null)
  const [isCreateModalOpen, setIsCreateModalOpen] = useState(false)
  const [editForm, setEditForm] = useState<EditForm>(buildEmptyEditForm())
  const [savingOrder, setSavingOrder] = useState(false)
  const [draggingId, setDraggingId] = useState<string | null>(null)
  const [isImportModalOpen, setIsImportModalOpen] = useState(false)
  const [importForm, setImportForm] = useState<CCSwitchImportForm>(defaultImportForm())
  const [importing, setImporting] = useState(false)
  const [lastImportSummary, setLastImportSummary] = useState<CCSwitchImportSummary | null>(null)
  const sensors = useSensors(useSensor(PointerSensor, { activationConstraint: { distance: 6 } }))
  const dialog = useDialog()
  const isEditing = !!editingNode
  const isNodeModalOpen = isCreateModalOpen || isEditing
  const healthCheckModelListId = 'health-check-model-options'
  const healthCheckModelChoices = getHealthCheckModelOptions(editForm.source_protocol)

  const showToast = (message: string, type: 'success' | 'error' = 'success') => {
    setToast({ message, type })
    setTimeout(() => setToast(null), 2200)
  }

  const toTimestamp = (val?: string | number | Date | null) => {
    const d = parseToDate(val)
    return d ? d.getTime() : 0
  }

  const sortByOrder = useCallback(
    (list: Node[]) =>
      list
        .slice()
        .sort((a, b) => {
          const wa = a.weight ?? 0
          const wb = b.weight ?? 0
          if (wa !== wb) return wa - wb
          const ta = toTimestamp(a.created_at ?? null)
          const tb = toTimestamp(b.created_at ?? null)
          return ta - tb
        }),
    [],
  )

  const loadAccounts = useCallback(async () => {
    try {
      const list = await api.getAccounts()
      setAccounts(list)
      setAccountId((prev) => prev || list[0]?.id || '')
    } catch (err) {
      showToast('加载账号失败', 'error')
    }
  }, [])

  const loadNodes = useCallback(async () => {
    if (!accountId) return
    setLoading(true)
    try {
      const list = await api.getNodes(accountId)
      setNodes(sortByOrder(list))
    } catch (err) {
      showToast((err as Error).message || '加载失败', 'error')
    } finally {
      setLoading(false)
    }
  }, [accountId, sortByOrder])

  useEffect(() => {
    void loadAccounts()
  }, [loadAccounts])

  useEffect(() => {
    if (accountId) {
      void loadNodes()
    }
  }, [accountId, loadNodes])

  const filtered = useMemo(() => {
    const q = search.trim().toLowerCase()
    return nodes.filter((node) => {
      const proto = node.source_protocol || 'claude'
      if (proto !== protocolTab) return false
      const match = !q || (node.name || '').toLowerCase().includes(q) || (node.base_url || '').toLowerCase().includes(q)
      if (!match) return false
      if (filter === 'online') return node.status === 'online'
      if (filter === 'offline') return node.status === 'offline'
      if (filter === 'degraded') return node.status === 'degraded'
      if (filter === 'disabled') return node.status === 'disabled'
      return true
    })
  }, [nodes, search, filter, protocolTab])

  const accountName = useMemo(() => accounts.find((item) => item.id === accountId)?.name || '', [accounts, accountId])

  const resetNodeModal = () => {
    setEditingNode(null)
    setIsCreateModalOpen(false)
    setEditForm(buildEmptyEditForm(protocolTab))
  }

  const closeNodeModal = () => {
    if (saving) return
    resetNodeModal()
  }

  const closeImportModal = () => {
    if (importing) return
    setIsImportModalOpen(false)
    setImportForm(defaultImportForm())
  }

  const loadNodeDetail = async (id: string) => {
    const node = await api.getNode(id, accountId)
    return node
  }

  const openAddModal = () => {
    setEditingNode(null)
    setEditForm(buildEmptyEditForm(protocolTab))
    setIsCreateModalOpen(true)
  }

  const openEditModal = async (node: Node) => {
    try {
      setDetailLoadingId(node.id)
      const full = await loadNodeDetail(node.id)
      setEditForm(buildEditFormFromNode(full))
      setDetailNode(null)
      setEditingNode(full)
      setIsCreateModalOpen(false)
    } catch (err) {
      showToast((err as Error).message || '加载节点失败', 'error')
    } finally {
      setDetailLoadingId('')
    }
  }

  const openDetailModal = async (node: Node) => {
    try {
      setDetailLoadingId(node.id)
      const full = await loadNodeDetail(node.id)
      setEditingNode(null)
      setDetailNode(full)
    } catch (err) {
      showToast((err as Error).message || '加载节点失败', 'error')
    } finally {
      setDetailLoadingId('')
    }
  }

  const handleSourceProtocolChange = (nextProtocol: SourceProtocol) => {
    setEditForm((prev) => {
      const nextHealthCheckModel = getHealthCheckModelOptions(nextProtocol).includes(prev.health_check_model)
        ? prev.health_check_model
        : getDefaultHealthCheckModel(nextProtocol)
      return {
        ...prev,
        source_protocol: nextProtocol,
        health_check_method: protocolLockedHealthMethod(nextProtocol) || prev.health_check_method,
        health_check_model: nextHealthCheckModel,
      }
    })
  }

  const handleAction = async (act: 'switch' | 'toggle' | 'del' | 'copy', node: Node) => {
    try {
      setActionId(node.id)
      if (act === 'switch') {
        if (node.is_active || node.status === 'disabled') return
        await api.activateNode(node.id)
        showToast('已切换')
        await loadNodes()
        return
      }
      if (act === 'toggle') {
        await api.toggleNode(node.id, node.status === 'disabled')
        showToast(node.status === 'disabled' ? '已启用' : '已禁用')
        await loadNodes()
        return
      }
      if (act === 'copy') {
        const copiedId = await api.copyNode(node.id)
        await loadNodes()
        showToast('节点已复制，已为副本打开编辑面板')
        const copied = await loadNodeDetail(copiedId)
        setEditForm(buildEditFormFromNode(copied))
        setEditingNode(copied)
        setDetailNode(null)
        return
      }
      const ok = await dialog.confirm({ title: '确认删除', message: '确认删除该节点？' })
      if (!ok) return
      await api.deleteNode(node.id)
      showToast('已删除')
      await loadNodes()
    } catch (err) {
      showToast((err as Error).message || '操作失败', 'error')
    } finally {
      setActionId('')
    }
  }

  const submitNode = async () => {
    if (!editForm.base_url.trim()) {
      showToast('Base URL 必填', 'error')
      return
    }
    if (!isEditing && !accountId) {
      showToast('请先选择账号', 'error')
      return
    }
    const weight = parseInt(editForm.weight || '1', 10)
    if (!Number.isInteger(weight) || weight <= 0) {
      showToast('权重需为正整数', 'error')
      return
    }

    const lockedMethod = protocolLockedHealthMethod(editForm.source_protocol)
    const healthMethod = lockedMethod || editForm.health_check_method || 'api'
    const healthModel = getDisplayHealthCheckModel(editForm.source_protocol, editForm.health_check_model)
    const apiKeys = normalizeApiKeys(editForm.api_keys)
    if (requiresApiKey(healthMethod) && apiKeys.length === 0) {
      showToast('选择 API/CLI 健康检查时需至少配置一个 API Key', 'error')
      return
    }

    const mappingObj: Record<string, string> = {}
    for (const entry of editForm.model_mapping) {
      const from = entry.from.trim()
      const to = entry.to.trim()
      if (from && to && from !== to) {
        mappingObj[from] = to
      }
    }

    setSaving(true)
    try {
      const payload = {
        name: editForm.name.trim(),
        base_url: editForm.base_url.trim(),
        weight,
        api_keys: apiKeys,
        health_check_method: healthMethod,
        health_check_model: healthModel,
        source_protocol: editForm.source_protocol,
        auth_profile: editForm.auth_profile.trim() || undefined,
        capabilities: editForm.capabilities.trim() || undefined,
        model_mapping: mappingObj,
      }
      if (editingNode) {
        await api.updateNode(editingNode.id, payload)
        showToast('已保存')
      } else {
        await api.createNode(payload, accountId)
        setProtocolTab(editForm.source_protocol)
        showToast('已新增节点')
      }
      resetNodeModal()
      await loadNodes()
    } catch (err) {
      showToast((err as Error).message || (editingNode ? '保存失败' : '新增失败'), 'error')
    } finally {
      setSaving(false)
    }
  }

  const handleImport = async () => {
    if (!accountId) {
      showToast('请先选择账号', 'error')
      return
    }
    if (!importForm.file) {
      showToast('请选择 cc-switch 数据库文件', 'error')
      return
    }
    setImporting(true)
    try {
      const weightOffset = parseInt(importForm.weight_offset || '1000', 10)
      const result = await api.importCCSwitchDB(importForm.file, {
        account_id: accountId,
        import_providers: importForm.import_providers,
        import_pricing: importForm.import_pricing,
        import_logs: importForm.import_logs,
        weight_offset: Number.isFinite(weightOffset) ? weightOffset : 1000,
      })
      setLastImportSummary(result.summary)
      showToast(`导入完成：节点 ${result.summary.providers_imported}，定价 ${result.summary.pricing_imported}，日志 ${result.summary.logs_imported}`)
      setIsImportModalOpen(false)
      setImportForm(defaultImportForm())
      await loadNodes()
    } catch (err) {
      showToast((err as Error).message || '导入失败', 'error')
    } finally {
      setImporting(false)
    }
  }

  const handleExport = async () => {
    if (!accountId) {
      showToast('请先选择账号', 'error')
      return
    }
    setExporting(true)
    try {
      const { blob, filename } = await api.exportCCSwitchDB(accountId)
      const url = window.URL.createObjectURL(blob)
      const a = document.createElement('a')
      a.href = url
      a.download = filename
      document.body.appendChild(a)
      a.click()
      a.remove()
      window.URL.revokeObjectURL(url)
      showToast('导出成功')
    } catch (err) {
      showToast((err as Error).message || '导出失败', 'error')
    } finally {
      setExporting(false)
    }
  }

  const statusInfo = (node: Node) => {
    switch (node.status) {
      case 'disabled':
        return { label: 'Disabled', cls: 'off' }
      case 'offline':
        return { label: 'Offline', cls: 'fail' }
      case 'degraded':
        return { label: 'Degraded', cls: 'warn' }
      case 'online':
      default:
        return { label: 'Online', cls: 'ok' }
    }
  }

  const errorSeverityLabel = (severity?: string) => {
    switch (severity) {
      case 'key_invalid':
        return 'Key 失效'
      case 'account_issue':
        return '账号问题'
      case 'node_down':
        return '节点宕机'
      case 'degraded':
        return '性能降级'
      case 'transient':
        return '临时错误'
      case 'permanent':
        return '请求错误'
      default:
        return ''
    }
  }

  const errorSeverityClass = (severity?: string) => {
    switch (severity) {
      case 'key_invalid':
      case 'account_issue':
      case 'node_down':
        return 'severity-danger'
      case 'degraded':
        return 'severity-warn'
      default:
        return ''
    }
  }

  const formatKeyStatus = (node: Node) => {
    if (!node.key_count || node.key_count <= 1) {
      return node.has_api_key ? '✓' : '-'
    }
    return `${node.active_key_count ?? 0}/${node.key_count}`
  }

  const formatKeyPreview = (node: Node) => {
    const keyNames = node.key_names || []
    if (keyNames.length > 0) {
      const labels = keyNames.slice(0, 2).map((name) => displayNodeKeyName(node.name || '', name))
      const extra = keyNames.length > 2 ? ` +${keyNames.length - 2}` : ''
      return `${labels.join(' / ')}${extra}`
    }
    if ((node.key_count || 0) > 1) {
      return `共 ${node.key_count} 个 Key`
    }
    return node.has_api_key ? '已配置 Key' : '未配置 Key'
  }

  const healthClass = (health: number | null) => {
    if (health === null) return ''
    if (health >= 80) return 'health-good'
    if (health >= 50) return 'health-warn'
    return 'health-bad'
  }

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

  const handleDragStart = (event: DragStartEvent) => {
    setDraggingId(String(event.active.id))
  }

  const handleDragCancel = () => setDraggingId(null)

  const handleDragEnd = (event: DragEndEvent) => {
    setDraggingId(null)
    if (savingOrder) return
    const { active, over } = event
    if (!over || active.id === over.id) return
    const activeId = String(active.id)
    const overId = String(over.id)
    const oldIndex = nodes.findIndex((node) => node.id === activeId)
    const newIndex = nodes.findIndex((node) => node.id === overId)
    if (oldIndex === -1 || newIndex === -1) return

    const prevNodes = [...nodes]
    const reordered = arrayMove(nodes, oldIndex, newIndex)
    const withWeights = reordered.map((node, idx) => ({ ...node, weight: idx + 1 }))
    setNodes(withWeights)
    setSavingOrder(true)

    setTimeout(() => {
      Promise.all(
        withWeights.map((node, idx) =>
          api.updateNode(node.id, {
            name: node.name || '',
            base_url: node.base_url,
            weight: idx + 1,
            health_check_method: node.health_check_method || 'api',
            health_check_model: getDisplayHealthCheckModel(node.source_protocol, node.health_check_model),
            source_protocol: node.source_protocol || 'claude',
            auth_profile: node.auth_profile || '',
            capabilities: node.capabilities || '',
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
        <td className="node-name-cell">
          <div>{node.name || '未命名'} <RecoveryBadge nodeId={node.id} /></div>
          <div className="node-key-preview">{formatKeyPreview(node)}</div>
        </td>
        <td>
          <div style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
            <div
              className={`pill ${status.cls}`}
              style={{ cursor: (node.status === 'offline' || node.status === 'degraded') && node.last_error ? 'pointer' : 'default' }}
              onClick={() => ((node.status === 'offline' || node.status === 'degraded') ? void openDetailModal(node) : undefined)}
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
        <td>{health === null ? '-' : <span className={healthClass(health)}>{health.toFixed(1)}%</span>}</td>
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
                <button className="btn ghost" type="button" onClick={() => void handleAction('switch', node)} disabled={actionId === node.id}>
                  切换
                </button>
              )}
              <button className="btn ghost" type="button" onClick={() => void openEditModal(node)} disabled={detailLoadingId === node.id}>
                编辑
              </button>
              <button className="btn ghost" type="button" onClick={() => void handleAction('copy', node)} disabled={actionId === node.id}>
                复制
              </button>
              <button className="btn ghost" type="button" onClick={() => void openDetailModal(node)} disabled={detailLoadingId === node.id}>
                查看详情
              </button>
            </div>
            <div style={{ display: 'flex', gap: 6, flexWrap: 'wrap' }}>
              <button className="btn warn" type="button" onClick={() => void handleAction('toggle', node)} disabled={actionId === node.id}>
                {node.status === 'disabled' ? '启用' : '禁用'}
              </button>
              <button className="btn danger" type="button" onClick={() => void handleAction('del', node)} disabled={actionId === node.id}>
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
        <p className="sub">按协议分组维护节点，支持多 Key 命名、节点复制，以及 cc-switch 数据导入导出。</p>
      </div>

      <Card>
        <div className="tabs" role="tablist" aria-label="节点协议类型">
          <button className={`tab ${protocolTab === 'claude' ? 'active' : ''}`} type="button" onClick={() => setProtocolTab('claude')}>Claude</button>
          <button className={`tab ${protocolTab === 'openai' ? 'active' : ''}`} type="button" onClick={() => setProtocolTab('openai')}>OpenAI</button>
          <button className={`tab ${protocolTab === 'gemini' ? 'active' : ''}`} type="button" onClick={() => setProtocolTab('gemini')}>Gemini</button>
        </div>
        <div className="toolbar">
          <label style={{ minWidth: 220 }}>
            选择账号
            <select value={accountId} onChange={(e) => setAccountId(e.target.value)}>
              {accounts.map((account) => (
                <option key={account.id} value={account.id}>
                  {account.name}
                  {account.is_admin ? ' [管]' : ''}
                </option>
              ))}
            </select>
          </label>
          <div className="spacer" />
          <button className="btn ghost" type="button" onClick={() => void loadNodes()} disabled={loading}>
            刷新
          </button>
          <button className="btn ghost" type="button" onClick={() => setIsImportModalOpen(true)} disabled={!accountId || importing}>
            导入 cc-switch DB
          </button>
          <button className="btn ghost" type="button" onClick={() => void handleExport()} disabled={!accountId || exporting}>
            {exporting ? '导出中...' : '导出 cc-switch DB'}
          </button>
          <button className="btn primary" type="button" onClick={openAddModal}>
            新增节点
          </button>
        </div>
        {lastImportSummary && (
          <div className="import-summary-bar">
            最近一次导入到账户 {lastImportSummary.account_id}：
            节点 {lastImportSummary.providers_imported}，
            定价 {lastImportSummary.pricing_imported}，
            日志 {lastImportSummary.logs_imported}
          </div>
        )}
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
          拖拽左侧手柄调整节点顺序，自动保存{savingOrder ? '中…' : ''}。当前账号：{accountName || '未选择'}
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
                <th style={{ minWidth: 240 }}>操作</th>
              </tr>
            </thead>
            <DndContext
              sensors={sensors}
              collisionDetection={closestCenter}
              onDragStart={handleDragStart}
              onDragEnd={handleDragEnd}
              onDragCancel={handleDragCancel}
            >
              <SortableContext items={filtered.map((node) => node.id)} strategy={verticalListSortingStrategy}>
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
                    filtered.map((node) => <NodeRow key={node.id} node={node} />)
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
        footer={(
          <div className="dialog-actions">
            <button className="btn ghost" type="button" onClick={() => setDetailNode(null)}>
              关闭
            </button>
          </div>
        )}
      >
        {detailNode && (
          <div>
            <div className="node-stats">
              {renderStat('名称', detailNode.name || '未命名')}
              {renderStat('Base URL', detailNode.base_url || '-')}
              {renderStat('健康检查', formatHealthMethod(detailNode.health_check_method))}
              {renderStat('健康检查模型', getDisplayHealthCheckModel(detailNode.source_protocol, detailNode.health_check_model))}
              {renderStat('权重', detailNode.weight ?? '-')}
              {renderStat('状态', statusInfo(detailNode).label)}
              {renderStat('源协议', detailNode.source_protocol || 'claude')}
              {renderStat('Auth Profile', detailNode.auth_profile || '-')}
              {renderStat('Capabilities', detailNode.capabilities || '-')}
              {detailNode.is_active && renderStat('选中', '当前活跃节点')}
            </div>

            <div className="node-stats">
              {renderStat('最后健康检查', formatDateTime(detailNode.last_health_check_at))}
              {renderStat('Ping 延迟 (ms)', detailNode.last_ping_ms ?? '-')}
              {renderStat('Key 数量', detailNode.key_count ?? 0)}
              {renderStat('可用 Key', detailNode.active_key_count ?? 0)}
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

            {(detailNode.api_keys?.length || 0) > 0 && (
              <div className="key-list-card">
                <div className="key-list-title">节点 Key</div>
                {detailNode.api_keys?.map((item, idx) => (
                  <div key={`${item.name}-${idx}`} className="key-list-row">
                    <div className="key-list-name">{displayNodeKeyName(detailNode.name || '', item.name || `key${idx + 1}`)}</div>
                    <div className="key-list-value">{item.key}</div>
                  </div>
                ))}
              </div>
            )}

            {detailNode.model_mapping && Object.keys(detailNode.model_mapping).length > 0 && (
              <div className="key-list-card">
                <div className="key-list-title">模型映射</div>
                {Object.entries(detailNode.model_mapping).map(([from, to]) => (
                  <div key={from} className="mapping-row">
                    <span>{from}</span>
                    <span>&rarr;</span>
                    <span>{to}</span>
                  </div>
                ))}
              </div>
            )}

            {detailNode.last_error && (
              <div className="error-detail">
                <div style={{ fontWeight: 700, marginBottom: 4 }}>最后错误</div>
                <ErrorDetailBlock detail={detailNode.last_error} />
              </div>
            )}
          </div>
        )}
      </Modal>

      <Modal
        open={isNodeModalOpen}
        title={isEditing ? '编辑节点' : '新增节点'}
        onClose={closeNodeModal}
        size="lg"
        footer={(
          <div className="dialog-actions">
            <button className="btn ghost" type="button" onClick={closeNodeModal}>
              取消
            </button>
            <button className="btn primary" type="button" onClick={() => void submitNode()} disabled={saving}>
              {isEditing ? '保存' : '创建'}
            </button>
          </div>
        )}
      >
        {isNodeModalOpen && (
          <div className="prompt-form">
            <div className="prompt-grid">
              <label>
                节点名称
                <input
                  data-autofocus="true"
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
                源协议类型
                <select
                  value={editForm.source_protocol}
                  onChange={(e) => handleSourceProtocolChange(e.target.value as SourceProtocol)}
                >
                  <option value="claude">Claude</option>
                  <option value="openai">OpenAI/Codex</option>
                  <option value="gemini">Gemini</option>
                </select>
              </label>
              <label>
                健康检查方式
                <select
                  value={protocolLockedHealthMethod(editForm.source_protocol) || editForm.health_check_method}
                  onChange={(e) => setEditForm((prev) => ({ ...prev, health_check_method: e.target.value as EditForm['health_check_method'] }))}
                  disabled={!!protocolLockedHealthMethod(editForm.source_protocol)}
                >
                  {healthMethodOptions.map((opt) => (
                    <option key={opt.value} value={opt.value}>
                      {opt.label}
                    </option>
                  ))}
                </select>
                <span className="weight-hint">
                  {protocolLockedHealthMethod(editForm.source_protocol)
                    ? 'OpenAI/Gemini 协议固定为 API 健康检查'
                    : 'API/CLI 需要有效的 API Key，CLI 需 Docker'}
                </span>
              </label>
              <label>
                健康检查模型
                <div className="combobox-field">
                  <input
                    list={healthCheckModelListId}
                    value={editForm.health_check_model}
                    onChange={(e) => setEditForm((prev) => ({ ...prev, health_check_model: e.target.value }))}
                    placeholder="选择推荐模型，或手动输入自定义模型名"
                  />
                  <datalist id={healthCheckModelListId}>
                    {healthCheckModelChoices.map((model) => (
                      <option key={model} value={model} />
                    ))}
                  </datalist>
                </div>
                <span className="weight-hint">可从下拉列表选择，也可直接输入；切换协议时会自动校正为该协议默认模型</span>
              </label>
              <label>
                Auth Profile(JSON)
                <input
                  value={editForm.auth_profile}
                  onChange={(e) => setEditForm((prev) => ({ ...prev, auth_profile: e.target.value }))}
                  placeholder='{"header":"Authorization"}'
                />
              </label>
              <label>
                Capabilities(JSON)
                <input
                  value={editForm.capabilities}
                  onChange={(e) => setEditForm((prev) => ({ ...prev, capabilities: e.target.value }))}
                  placeholder='{"supports_stream":true}'
                />
              </label>
            </div>

            <div className="key-list-card">
              <div className="key-list-header">
                <div className="key-list-title">节点 Key</div>
                <button
                  type="button"
                  className="btn ghost"
                  style={{ padding: '4px 10px', fontSize: 12 }}
                  onClick={() => setEditForm((prev) => ({ ...prev, api_keys: [...prev.api_keys, makeKeyRow()] }))}
                >
                  添加 Key
                </button>
              </div>
              {editForm.api_keys.map((item, idx) => (
                <div key={item.id} className="key-editor-row">
                  <input
                    value={item.name}
                    onChange={(e) =>
                      setEditForm((prev) => ({
                        ...prev,
                        api_keys: prev.api_keys.map((row) => (row.id === item.id ? { ...row, name: e.target.value } : row)),
                      }))
                    }
                    placeholder={`Key 名称（如：primary-${idx + 1}）`}
                  />
                  <input
                    value={item.key}
                    onChange={(e) =>
                      setEditForm((prev) => ({
                        ...prev,
                        api_keys: prev.api_keys.map((row) => (row.id === item.id ? { ...row, key: e.target.value } : row)),
                      }))
                    }
                    placeholder="直接展示并编辑 API Key"
                  />
                  <div className="key-editor-preview">{displayNodeKeyName(editForm.name || '节点', item.name || `key${idx + 1}`)}</div>
                  <button
                    type="button"
                    className="btn ghost"
                    onClick={() =>
                      setEditForm((prev) => ({
                        ...prev,
                        api_keys: prev.api_keys.filter((row) => row.id !== item.id),
                      }))
                    }
                    disabled={editForm.api_keys.length <= 1}
                  >
                    删除
                  </button>
                </div>
              ))}
              <div className="weight-hint">多 Key 会以“节点名称-Key 名称”的形式在导出和其他展示区域出现。</div>
            </div>

            <div style={{ marginTop: 12 }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 6 }}>
                <span style={{ fontWeight: 600, fontSize: 13 }}>模型映射</span>
                <button
                  type="button"
                  className="btn ghost"
                  style={{ padding: '2px 8px', fontSize: 12, lineHeight: '20px' }}
                  onClick={() => setEditForm((prev) => ({ ...prev, model_mapping: [...prev.model_mapping, makeMappingRow()] }))}
                >
                  添加映射
                </button>
                <span className="weight-hint" style={{ marginLeft: 4 }}>请求中的模型自动替换为目标模型</span>
              </div>
              {editForm.model_mapping.map((entry, idx) => (
                <div key={entry.id} className="mapping-editor-row">
                  <input
                    list="model-list-from"
                    value={entry.from}
                    onChange={(e) => {
                      const val = e.target.value
                      setEditForm((prev) => {
                        const arr = [...prev.model_mapping]
                        arr[idx] = { ...arr[idx], from: val }
                        return { ...prev, model_mapping: arr }
                      })
                    }}
                    placeholder="源模型（选择或输入）"
                  />
                  <span>&rarr;</span>
                  <input
                    list="model-list-to"
                    value={entry.to}
                    onChange={(e) => {
                      const val = e.target.value
                      setEditForm((prev) => {
                        const arr = [...prev.model_mapping]
                        arr[idx] = { ...arr[idx], to: val }
                        return { ...prev, model_mapping: arr }
                      })
                    }}
                    placeholder="目标模型（选择或输入）"
                  />
                  <button
                    type="button"
                    className="btn ghost"
                    onClick={() => setEditForm((prev) => ({ ...prev, model_mapping: prev.model_mapping.filter((_, i) => i !== idx) }))}
                  >
                    删除
                  </button>
                </div>
              ))}
              <datalist id="model-list-from">
                {allKnownModelIds.map((item) => <option key={item} value={item} />)}
              </datalist>
              <datalist id="model-list-to">
                {allKnownModelIds.map((item) => <option key={item} value={item} />)}
              </datalist>
              {editForm.model_mapping.length === 0 && (
                <div style={{ fontSize: 12, color: 'var(--text-secondary)', padding: '4px 0' }}>
                  暂无映射规则，点击“添加映射”创建
                </div>
              )}
            </div>

            {editingNode && (
              <>
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
                    <ErrorDetailBlock detail={editingNode.last_error} />
                  </div>
                )}
              </>
            )}
          </div>
        )}
      </Modal>

      <Modal
        open={isImportModalOpen}
        title="导入 cc-switch 数据库"
        onClose={closeImportModal}
        size="md"
        footer={(
          <div className="dialog-actions">
            <button className="btn ghost" type="button" onClick={closeImportModal}>
              取消
            </button>
            <button className="btn primary" type="button" onClick={() => void handleImport()} disabled={importing}>
              {importing ? '导入中...' : '开始导入'}
            </button>
          </div>
        )}
      >
        <div className="prompt-form">
          <label className="import-field">
            <span>目标账号</span>
            <div className="import-target-account">{accountName || accountId || '未选择账号'}</div>
          </label>
          <label className="import-field">
            <span>选择数据库文件</span>
            <input
              type="file"
              accept=".db,.sqlite,.sqlite3,application/vnd.sqlite3"
              onChange={(e) => setImportForm((prev) => ({ ...prev, file: e.target.files?.[0] || null }))}
            />
          </label>
          <label className="import-field">
            <span>权重偏移</span>
            <input
              type="number"
              value={importForm.weight_offset}
              onChange={(e) => setImportForm((prev) => ({ ...prev, weight_offset: e.target.value }))}
            />
          </label>
          <label className="import-checkbox">
            <input
              type="checkbox"
              checked={importForm.import_providers}
              onChange={(e) => setImportForm((prev) => ({ ...prev, import_providers: e.target.checked }))}
            />
            <span>导入节点</span>
          </label>
          <label className="import-checkbox">
            <input
              type="checkbox"
              checked={importForm.import_pricing}
              onChange={(e) => setImportForm((prev) => ({ ...prev, import_pricing: e.target.checked }))}
            />
            <span>导入模型定价</span>
          </label>
          <label className="import-checkbox">
            <input
              type="checkbox"
              checked={importForm.import_logs}
              onChange={(e) => setImportForm((prev) => ({ ...prev, import_logs: e.target.checked }))}
            />
            <span>导入请求日志</span>
          </label>
          <div className="weight-hint">支持直接选择本地 `cc-switch.db`，导入成功后会自动刷新当前账号节点。</div>
        </div>
      </Modal>

      <Toast message={toast?.message} type={toast?.type} />
    </div>
  )
}
