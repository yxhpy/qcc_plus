import { useCallback, useEffect, useRef, useState } from 'react'
import Card from '../components/Card'
import RecoveryBadge from '../components/RecoveryBadge'
import Toast from '../components/Toast'
import api from '../services/api'
import type { Account, Node, UsageLog, UsageLogAttempt } from '../types'
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
  const [expandedRows, setExpandedRows] = useState<Set<number>>(new Set())
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

      const res = await api.getUsageLogs(params as any, true)
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
                    <th style={{ width: 32 }}></th>
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
                  {logs.map(log => {
                    const hasAttempts = !!(log.attempts && log.attempts.length > 0)
                    const isExpanded = expandedRows.has(log.id)
                    return (
                      <ReqLogRow
                        key={log.id}
                        log={log}
                        hasAttempts={hasAttempts}
                        isExpanded={isExpanded}
                        onToggle={() => {
                          setExpandedRows(prev => {
                            const next = new Set(prev)
                            if (next.has(log.id)) {
                              next.delete(log.id)
                            } else {
                              next.add(log.id)
                            }
                            return next
                          })
                        }}
                        formatDate={formatDate}
                        formatDuration={formatDuration}
                        formatTokens={formatTokens}
                        formatCost={formatCost}
                      />
                    )
                  })}
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

// 严重程度标签
function severityLabel(severity?: string): string {
  switch (severity) {
    case 'transient': return '临时'
    case 'node_down': return '宕机'
    case 'key_invalid': return '密钥失效'
    case 'permanent': return '永久'
    case 'account_issue': return '账号问题'
    case 'context_error': return '超时'
    case 'degraded': return '降级'
    default: return severity || ''
  }
}

// 动作标签
function actionLabel(action?: string): string {
  switch (action) {
    case 'retry': return '重试'
    case 'fail': return '失败'
    case 'success': return '成功'
    case 'circuit_open': return '熔断'
    case 'key_rotate': return '换Key'
    case 'abort': return '中止'
    default: return action || ''
  }
}

interface ReqLogRowProps {
  log: UsageLog
  hasAttempts: boolean
  isExpanded: boolean
  onToggle: () => void
  formatDate: (s: string) => string
  formatDuration: (ms: number) => string
  formatTokens: (n: number) => string
  formatCost: (n: number) => string
}

function ReqLogRow({ log, hasAttempts, isExpanded, onToggle, formatDate, formatDuration, formatTokens, formatCost }: ReqLogRowProps) {
  return (
    <>
      <tr
        className={`${!log.success ? 'failed' : ''} ${hasAttempts ? 'expandable' : ''}`}
        onClick={hasAttempts ? onToggle : undefined}
      >
        <td className="expand-cell">
          {hasAttempts && (
            <span className={`expand-icon ${isExpanded ? 'expanded' : ''}`}>&#9654;</span>
          )}
        </td>
        <td className="time-cell">{formatDate(log.created_at)}</td>
        <td className="model-cell">{log.model_id || '-'}</td>
        <td className="node-cell">
          {log.node_name || log.node_id?.slice(0, 8) || '-'}
          <RecoveryBadge nodeId={log.node_id} />
          {(log.total_attempts || 0) > 1 && <span className="attempts-badge">{log.total_attempts}次</span>}
        </td>
        <td>
          <span className={`status-badge ${log.success ? 'success' : 'failed'}`}>
            {log.success ? '成功' : '失败'}
          </span>
          {!log.success && log.error_msg && (
            <span className="error-msg" title={log.error_msg}>{log.error_msg}</span>
          )}
        </td>
        <td className="duration-cell">{formatDuration(log.duration_ms)}</td>
        <td className="token-cell">{formatTokens(log.input_tokens)}</td>
        <td className="token-cell">{formatTokens(log.output_tokens)}</td>
        <td className="cost-cell">{formatCost(log.cost_usd)}</td>
      </tr>
      {isExpanded && log.attempts && log.attempts.length > 0 && (
        <tr className="attempts-row">
          <td colSpan={9}>
            <div className="attempts-chain">
              {log.attempts.map((attempt: UsageLogAttempt, idx: number) => (
                <div key={attempt.id || idx} className={`attempt-item ${attempt.success ? 'success' : 'failed'}`}>
                  <div className="attempt-header">
                    <span className="attempt-seq">#{attempt.seq}</span>
                    <span className="attempt-node">{attempt.node_name || attempt.node_id}</span>
                    <span className={`attempt-status ${attempt.success ? 'success' : 'failed'}`}>
                      {attempt.status_code > 0 ? attempt.status_code : '--'}
                    </span>
                    <span className="attempt-duration">{formatDuration(attempt.duration_ms)}</span>
                    {attempt.action && (
                      <span className={`attempt-action action-${attempt.action}`}>{actionLabel(attempt.action)}</span>
                    )}
                  </div>
                  {attempt.error_msg && (
                    <div className="attempt-error">
                      {attempt.severity && <span className={`severity-tag severity-${attempt.severity}`}>{severityLabel(attempt.severity)}</span>}
                      <span className="error-text">{attempt.error_msg}</span>
                    </div>
                  )}
                  {idx < log.attempts!.length - 1 && <div className="attempt-connector" />}
                </div>
              ))}
            </div>
          </td>
        </tr>
      )}
    </>
  )
}
