import { useCallback, useEffect, useRef, useState } from 'react'
import Card from '../components/Card'
import Toast from '../components/Toast'
import api from '../services/api'
import type { Account, Node, UsageLog, UsageSummary, UsageQueryParams } from '../types'
import './Usage.css'

export default function Usage() {
  const [accounts, setAccounts] = useState<Account[]>([])
  const [nodes, setNodes] = useState<Node[]>([])
  const [logs, setLogs] = useState<UsageLog[]>([])
  const [summary, setSummary] = useState<UsageSummary | null>(null)
  const [modelSummaries, setModelSummaries] = useState<UsageSummary[]>([])
  const [nodeSummaries, setNodeSummaries] = useState<UsageSummary[]>([])
  const [loading, setLoading] = useState(true)
  const [toast, setToast] = useState<{ message: string; type: 'success' | 'error' } | null>(null)
  const [activeTab, setActiveTab] = useState<'model' | 'node' | 'logs'>('model')
  const toastTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)

  // 筛选条件
  const [filters, setFilters] = useState<UsageQueryParams>({
    account_id: '',
    node_id: '',
    model_id: '',
    from: '',
    to: '',
    limit: 50,
  })

  const showToast = useCallback((message: string, type: 'success' | 'error' = 'success') => {
    if (toastTimerRef.current) {
      clearTimeout(toastTimerRef.current)
    }
    setToast({ message, type })
    toastTimerRef.current = setTimeout(() => setToast(null), 2200)
  }, [])

  // 清理定时器
  useEffect(() => {
    return () => {
      if (toastTimerRef.current) {
        clearTimeout(toastTimerRef.current)
      }
    }
  }, [])

  const loadAccounts = async () => {
    try {
      const list = await api.getAccounts()
      setAccounts(list)
    } catch (err) {
      console.error('Failed to load accounts:', err)
    }
  }

  const loadNodes = async (accountId?: string) => {
    try {
      const list = await api.getNodes(accountId)
      setNodes(list)
    } catch (err) {
      console.error('Failed to load nodes:', err)
    }
  }

  const loadData = async () => {
    setLoading(true)
    try {
      const params: UsageQueryParams = { ...filters }
      // 清理空值
      Object.keys(params).forEach((key) => {
        if (params[key as keyof UsageQueryParams] === '') {
          delete params[key as keyof UsageQueryParams]
        }
      })

      const [summaryRes, modelRes, nodeRes, logsRes] = await Promise.all([
        api.getUsageSummary(params),
        api.getUsageSummary({ ...params, group_by: 'model' }),
        api.getUsageSummary({ ...params, group_by: 'node' }),
        api.getUsageLogs(params),
      ])

      // 处理返回类型
      if (Array.isArray(summaryRes)) {
        setSummary(summaryRes[0] || null)
      } else {
        setSummary(summaryRes)
      }

      if (Array.isArray(modelRes)) {
        setModelSummaries(modelRes)
      } else {
        setModelSummaries([])
      }

      if (Array.isArray(nodeRes)) {
        setNodeSummaries(nodeRes)
      } else {
        setNodeSummaries([])
      }

      setLogs(logsRes.logs || [])
    } catch (err) {
      showToast((err as Error).message || '加载失败', 'error')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    loadAccounts()
    loadNodes()
  }, [])

  // 账号变更时重新加载节点列表
  useEffect(() => {
    loadNodes(filters.account_id || undefined)
    // 清空已选节点（因为可能不在新账号下）
    if (filters.node_id) {
      setFilters((prev) => ({ ...prev, node_id: '' }))
    }
  }, [filters.account_id])

  useEffect(() => {
    loadData()
  }, [filters])

  const formatCost = (cost: number) => {
    if (cost < 0.01) return `$${cost.toFixed(6)}`
    if (cost < 1) return `$${cost.toFixed(4)}`
    return `$${cost.toFixed(2)}`
  }

  const formatTokens = (tokens: number) => {
    if (tokens >= 1000000) return `${(tokens / 1000000).toFixed(2)}M`
    if (tokens >= 1000) return `${(tokens / 1000).toFixed(1)}K`
    return tokens.toString()
  }

  const formatDate = (dateStr: string) => {
    const date = new Date(dateStr)
    return date.toLocaleString('zh-CN', {
      month: '2-digit',
      day: '2-digit',
      hour: '2-digit',
      minute: '2-digit',
    })
  }

  const getNodeName = (nodeId: string) => {
    const node = nodes.find((n) => n.id === nodeId)
    return node?.name || nodeId.slice(0, 8)
  }

  return (
    <div className="usage-page">
      <div className="usage-header">
        <h1>使用统计</h1>
        <p className="sub">查看 API 调用量和费用统计。</p>
      </div>

      {/* 筛选器 */}
      <Card className="filter-card">
        <div className="filters">
          <label>
            <span>账号</span>
            <select
              value={filters.account_id || ''}
              onChange={(e) => setFilters({ ...filters, account_id: e.target.value })}
            >
              <option value="">全部账号</option>
              {accounts.map((a) => (
                <option key={a.id} value={a.id}>
                  {a.name}
                </option>
              ))}
            </select>
          </label>
          <label>
            <span>节点</span>
            <select
              value={filters.node_id || ''}
              onChange={(e) => setFilters({ ...filters, node_id: e.target.value })}
            >
              <option value="">全部节点</option>
              {nodes.map((n) => (
                <option key={n.id} value={n.id}>
                  {n.name}
                </option>
              ))}
            </select>
          </label>
          <label>
            <span>开始时间</span>
            <input
              type="datetime-local"
              value={filters.from || ''}
              onChange={(e) => setFilters({ ...filters, from: e.target.value ? new Date(e.target.value).toISOString() : '' })}
            />
          </label>
          <label>
            <span>结束时间</span>
            <input
              type="datetime-local"
              value={filters.to || ''}
              onChange={(e) => setFilters({ ...filters, to: e.target.value ? new Date(e.target.value).toISOString() : '' })}
            />
          </label>
          <button className="btn ghost" onClick={() => setFilters({ limit: 50 })}>
            重置
          </button>
          <button className="btn primary" onClick={loadData} disabled={loading}>
            刷新
          </button>
        </div>
      </Card>

      {/* 统计概览 */}
      {summary && (
        <div className="stats-grid">
          <div className="stat-card">
            <div className="stat-value">{formatCost(summary.total_cost_usd)}</div>
            <div className="stat-label">总费用</div>
          </div>
          <div className="stat-card">
            <div className="stat-value">{summary.total_requests.toLocaleString()}</div>
            <div className="stat-label">总请求数</div>
          </div>
          <div className="stat-card">
            <div className="stat-value">{formatTokens(summary.total_input_tokens)}</div>
            <div className="stat-label">输入 Token</div>
          </div>
          <div className="stat-card">
            <div className="stat-value">{formatTokens(summary.total_output_tokens)}</div>
            <div className="stat-label">输出 Token</div>
          </div>
          <div className="stat-card">
            <div className="stat-value">
              {summary.total_requests > 0
                ? ((summary.success_requests / summary.total_requests) * 100).toFixed(1)
                : 0}%
            </div>
            <div className="stat-label">成功率</div>
          </div>
        </div>
      )}

      {/* 标签页切换 */}
      <div className="tabs">
        <button
          className={`tab ${activeTab === 'model' ? 'active' : ''}`}
          onClick={() => setActiveTab('model')}
        >
          按模型统计
        </button>
        <button
          className={`tab ${activeTab === 'node' ? 'active' : ''}`}
          onClick={() => setActiveTab('node')}
        >
          按节点统计
        </button>
        <button
          className={`tab ${activeTab === 'logs' ? 'active' : ''}`}
          onClick={() => setActiveTab('logs')}
        >
          详细记录
        </button>
      </div>

      <Card>
        {loading ? (
          <div className="loading-text">加载中...</div>
        ) : activeTab === 'model' ? (
          /* 按模型统计 */
          modelSummaries.length === 0 ? (
            <div className="empty-text">暂无使用数据</div>
          ) : (
            <div className="table-wrapper">
              <table className="usage-table">
                <thead>
                  <tr>
                    <th>模型</th>
                    <th>请求数</th>
                    <th>输入 Token</th>
                    <th>输出 Token</th>
                    <th>费用</th>
                  </tr>
                </thead>
                <tbody>
                  {modelSummaries.map((ms, idx) => (
                    <tr key={idx}>
                      <td className="model-cell">{ms.model_id || '未知'}</td>
                      <td>{ms.total_requests.toLocaleString()}</td>
                      <td>{formatTokens(ms.total_input_tokens)}</td>
                      <td>{formatTokens(ms.total_output_tokens)}</td>
                      <td className="cost-cell">{formatCost(ms.total_cost_usd)}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )
        ) : activeTab === 'node' ? (
          /* 按节点统计 */
          nodeSummaries.length === 0 ? (
            <div className="empty-text">暂无使用数据</div>
          ) : (
            <div className="table-wrapper">
              <table className="usage-table">
                <thead>
                  <tr>
                    <th>节点</th>
                    <th>请求数</th>
                    <th>输入 Token</th>
                    <th>输出 Token</th>
                    <th>费用</th>
                  </tr>
                </thead>
                <tbody>
                  {nodeSummaries.map((ns, idx) => (
                    <tr key={idx}>
                      <td className="node-cell">{getNodeName(ns.node_id || '')}</td>
                      <td>{ns.total_requests.toLocaleString()}</td>
                      <td>{formatTokens(ns.total_input_tokens)}</td>
                      <td>{formatTokens(ns.total_output_tokens)}</td>
                      <td className="cost-cell">{formatCost(ns.total_cost_usd)}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )
        ) : /* 详细记录 */
        logs.length === 0 ? (
          <div className="empty-text">暂无使用记录</div>
        ) : (
          <div className="table-wrapper">
            <table className="usage-table">
              <thead>
                <tr>
                  <th>时间</th>
                  <th>模型</th>
                  <th>节点</th>
                  <th>输入</th>
                  <th>输出</th>
                  <th>费用</th>
                  <th>状态</th>
                </tr>
              </thead>
              <tbody>
                {logs.map((log) => (
                  <tr key={log.id} className={!log.success ? 'failed' : ''}>
                    <td className="time-cell">{formatDate(log.created_at)}</td>
                    <td className="model-cell">{log.model_id}</td>
                    <td>{getNodeName(log.node_id)}</td>
                    <td>{formatTokens(log.input_tokens)}</td>
                    <td>{formatTokens(log.output_tokens)}</td>
                    <td className="cost-cell">{formatCost(log.cost_usd)}</td>
                    <td>
                      <span className={`status-dot ${log.success ? 'success' : 'failed'}`} />
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </Card>

      <Toast message={toast?.message} type={toast?.type} />
    </div>
  )
}
