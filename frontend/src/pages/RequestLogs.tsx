import { useCallback, useEffect, useRef, useState } from 'react'
import Card from '../components/Card'
import Toast from '../components/Toast'
import api from '../services/api'
import type { Account, Node, UsageLog } from '../types'
import './RequestLogs.css'

const PAGE_SIZE = 20
const AUTO_REFRESH_INTERVAL = 10000 // 10s

export default function RequestLogs() {
  const [accounts, setAccounts] = useState<Account[]>([])
  const [nodes, setNodes] = useState<Node[]>([])
  const [logs, setLogs] = useState<UsageLog[]>([])
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [loading, setLoading] = useState(true)
  const [autoRefresh, setAutoRefresh] = useState(true)
  const [toast, setToast] = useState<{ message: string; type: 'success' | 'error' } | null>(null)
  const toastTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  const refreshTimerRef = useRef<ReturnType<typeof setInterval> | null>(null)

  // 筛选条件
  const [filters, setFilters] = useState({
    account_id: '',
    node_id: '',
    model_id: '',
    success: '', // '' | 'true' | 'false'
  })

  const showToast = useCallback((message: string, type: 'success' | 'error' = 'success') => {
    if (toastTimerRef.current) clearTimeout(toastTimerRef.current)
    setToast({ message, type })
    toastTimerRef.current = setTimeout(() => setToast(null), 2200)
  }, [])

  useEffect(() => {
    return () => {
      if (toastTimerRef.current) clearTimeout(toastTimerRef.current)
      if (refreshTimerRef.current) clearInterval(refreshTimerRef.current)
    }
  }, [])

  const loadAccounts = async () => {
    try {
      setAccounts(await api.getAccounts())
    } catch (err) {
      console.error('Failed to load accounts:', err)
    }
  }

  const loadNodes = async (accountId?: string) => {
    try {
      setNodes(await api.getNodes(accountId))
    } catch (err) {
      console.error('Failed to load nodes:', err)
    }
  }

  const loadData = useCallback(async (silent = false) => {
    if (!silent) setLoading(true)
    try {
      const params: Record<string, string | number> = {
        limit: PAGE_SIZE,
        offset: (page - 1) * PAGE_SIZE,
      }
      if (filters.account_id) params.account_id = filters.account_id
      if (filters.node_id) params.node_id = filters.node_id
      if (filters.model_id) params.model_id = filters.model_id
      if (filters.success) params.success = filters.success

      const res = await api.getUsageLogs(params as any)
      setLogs(res.logs || [])
      setTotal(res.total || 0)
    } catch (err) {
      if (!silent) showToast((err as Error).message || '加载失败', 'error')
    } finally {
      if (!silent) setLoading(false)
    }
  }, [page, filters, showToast])

  useEffect(() => {
    loadAccounts()
    loadNodes()
  }, [])

  useEffect(() => {
    loadNodes(filters.account_id || undefined)
  }, [filters.account_id])

  useEffect(() => {
    loadData()
  }, [loadData])

  // 自动刷新
  useEffect(() => {
    if (refreshTimerRef.current) clearInterval(refreshTimerRef.current)
    if (autoRefresh) {
      refreshTimerRef.current = setInterval(() => loadData(true), AUTO_REFRESH_INTERVAL)
    }
    return () => {
      if (refreshTimerRef.current) clearInterval(refreshTimerRef.current)
    }
  }, [autoRefresh, loadData])

  const totalPages = Math.max(1, Math.ceil(total / PAGE_SIZE))

  const handleFilterChange = (key: string, value: string) => {
    setFilters(prev => ({ ...prev, [key]: value }))
    setPage(1) // 筛选变化时回到第一页
  }

  const handleReset = () => {
    setFilters({ account_id: '', node_id: '', model_id: '', success: '' })
    setPage(1)
  }

  const formatDate = (dateStr: string) => {
    const date = new Date(dateStr)
    return date.toLocaleString('zh-CN', {
      month: '2-digit',
      day: '2-digit',
      hour: '2-digit',
      minute: '2-digit',
      second: '2-digit',
    })
  }

  const formatDuration = (ms: number) => {
    if (ms < 1000) return `${ms}ms`
    return `${(ms / 1000).toFixed(1)}s`
  }

  const formatTokens = (tokens: number) => {
    if (tokens >= 1000000) return `${(tokens / 1000000).toFixed(2)}M`
    if (tokens >= 1000) return `${(tokens / 1000).toFixed(1)}K`
    return tokens.toString()
  }

  const formatCost = (cost: number) => {
    if (cost < 0.01) return `$${cost.toFixed(6)}`
    if (cost < 1) return `$${cost.toFixed(4)}`
    return `$${cost.toFixed(2)}`
  }

  // 从 logs 中提取唯一的 model_id 列表
  const modelOptions = Array.from(new Set(logs.map(l => l.model_id).filter(Boolean)))

  return (
    <div className="reqlog-page">
      <div className="reqlog-header">
        <div className="reqlog-header-left">
          <h1>请求日志</h1>
          <p className="sub">实时查看每次 API 请求的模型、节点、状态和耗时。</p>
        </div>
        <div className="reqlog-header-right">
          <label className="auto-refresh-toggle">
            <input
              type="checkbox"
              checked={autoRefresh}
              onChange={e => setAutoRefresh(e.target.checked)}
            />
            <span>自动刷新</span>
          </label>
          <span className="reqlog-total">共 {total} 条</span>
        </div>
      </div>

      {/* 筛选器 */}
      <Card className="filter-card">
        <div className="filters">
          <label>
            <span>账号</span>
            <select
              value={filters.account_id}
              onChange={e => handleFilterChange('account_id', e.target.value)}
            >
              <option value="">全部账号</option>
              {accounts.map(a => (
                <option key={a.id} value={a.id}>{a.name}</option>
              ))}
            </select>
          </label>
          <label>
            <span>节点</span>
            <select
              value={filters.node_id}
              onChange={e => handleFilterChange('node_id', e.target.value)}
            >
              <option value="">全部节点</option>
              {nodes.map(n => (
                <option key={n.id} value={n.id}>{n.name}</option>
              ))}
            </select>
          </label>
          <label>
            <span>模型</span>
            <select
              value={filters.model_id}
              onChange={e => handleFilterChange('model_id', e.target.value)}
            >
              <option value="">全部模型</option>
              {modelOptions.map(m => (
                <option key={m} value={m}>{m}</option>
              ))}
            </select>
          </label>
          <label>
            <span>状态</span>
            <select
              value={filters.success}
              onChange={e => handleFilterChange('success', e.target.value)}
            >
              <option value="">全部</option>
              <option value="true">成功</option>
              <option value="false">失败</option>
            </select>
          </label>
          <button className="btn ghost" onClick={handleReset}>重置</button>
          <button className="btn primary" onClick={() => loadData()} disabled={loading}>刷新</button>
        </div>
      </Card>

      {/* 日志表格 */}
      <Card>
        {loading ? (
          <div className="loading-text">加载中...</div>
        ) : logs.length === 0 ? (
          <div className="empty-text">暂无请求日志</div>
        ) : (
          <>
            <div className="table-wrapper">
              <table className="reqlog-table">
                <thead>
                  <tr>
                    <th>时间</th>
                    <th>模型</th>
                    <th>节点</th>
                    <th>状态</th>
                    <th>耗时</th>
                    <th>输入</th>
                    <th>输出</th>
                    <th>费用</th>
                  </tr>
                </thead>
                <tbody>
                  {logs.map(log => (
                    <tr key={log.id} className={!log.success ? 'failed' : ''}>
                      <td className="time-cell">{formatDate(log.created_at)}</td>
                      <td className="model-cell">{log.model_id || '-'}</td>
                      <td className="node-cell">{log.node_name || log.node_id?.slice(0, 8) || '-'}</td>
                      <td>
                        <span className={`status-badge ${log.success ? 'success' : 'failed'}`}>
                          {log.success ? '成功' : '失败'}
                        </span>
                      </td>
                      <td className="duration-cell">{formatDuration(log.duration_ms)}</td>
                      <td className="token-cell">{formatTokens(log.input_tokens)}</td>
                      <td className="token-cell">{formatTokens(log.output_tokens)}</td>
                      <td className="cost-cell">{formatCost(log.cost_usd)}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>

            {/* 分页 */}
            {totalPages > 1 && (
              <div className="pagination">
                <button
                  className="btn ghost pagination-btn"
                  disabled={page <= 1}
                  onClick={() => setPage(p => Math.max(1, p - 1))}
                >
                  上一页
                </button>
                <span className="pagination-info">
                  {page} / {totalPages}
                </span>
                <button
                  className="btn ghost pagination-btn"
                  disabled={page >= totalPages}
                  onClick={() => setPage(p => Math.min(totalPages, p + 1))}
                >
                  下一页
                </button>
              </div>
            )}
          </>
        )}
      </Card>

      <Toast message={toast?.message} type={toast?.type} />
    </div>
  )
}
