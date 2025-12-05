import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { useParams } from 'react-router-dom'
import Card from '../components/Card'
import NodeCard from '../components/NodeCard'
import Toast from '../components/Toast'
import { useAuth } from '../hooks/useAuth'
import { useMonitorWebSocket } from '../hooks/useMonitorWebSocket'
import { useNodeMetrics } from '../contexts/NodeMetricsContext'
import { useSettings } from '../contexts/SettingsContext'
import api from '../services/api'
import type {
  Account,
  CreateMonitorShareRequest,
  HealthCheckRecord,
  MonitorDashboard,
  MonitorNode,
  MonitorShare,
} from '../types'
import { formatBeijingTime, parseToDate } from '../utils/date'
import './Monitor.css'

interface MonitorProps {
  shared?: boolean
}

export default function Monitor({ shared = false }: MonitorProps) {
  const params = useParams<{ token?: string }>()
  const shareToken = shared ? params.token : undefined
  const { isAdmin } = useAuth()
  const { preference, setPreference, resetToDefault } = useNodeMetrics()
  const { getSetting } = useSettings()
  const refreshInterval = getSetting<number>('monitor.refresh_interval_ms', 30000)

  const [accounts, setAccounts] = useState<Account[]>([])
  const [accountId, setAccountId] = useState('')
  const [dashboard, setDashboard] = useState<MonitorDashboard | null>(null)
  const [loading, setLoading] = useState(true)
  const [refreshing, setRefreshing] = useState(false)
  const [autoRefresh, setAutoRefresh] = useState(true)
  const [toast, setToast] = useState<{ message: string; type: 'success' | 'error' } | null>(null)
  const [shares, setShares] = useState<MonitorShare[]>([])
  const [shareLoading, setShareLoading] = useState(false)
  const [expireIn, setExpireIn] = useState<CreateMonitorShareRequest['expire_in']>('24h')
  const [historyRefreshKey, setHistoryRefreshKey] = useState(0)
  const [healthEvents, setHealthEvents] = useState<Record<string, HealthCheckRecord>>({})
  const lastNodeIdsRef = useRef('')

  const wsAccountId = shared ? undefined : accountId || undefined
  const { connected, lastMessage } = useMonitorWebSocket(wsAccountId, shareToken)

  const showToast = (message: string, type: 'success' | 'error' = 'success') => {
    setToast({ message, type })
    setTimeout(() => setToast(null), 2200)
  }

  const fetchAccounts = useCallback(async () => {
    setLoading(true)
    try {
      const list = await api.getAccounts()
      setAccounts(list)
      setAccountId((prev) => prev || (list[0]?.id ?? ''))
    } catch (err) {
      showToast('加载账号失败', 'error')
    } finally {
      setLoading(false)
    }
  }, [])

  const fetchDashboard = useCallback(
    async (withSkeleton = true) => {
      if (shared && !shareToken) return
      if (!shared && !accountId) return
      if (withSkeleton) setLoading(true)
      setRefreshing(true)
      try {
        const data = shared && shareToken
          ? await api.getSharedMonitor(shareToken)
          : await api.getMonitorDashboard(accountId)
        const nodeIdsSignature = (data.nodes || []).map((n) => n.id).sort().join('|')
        const nextSignature = `${shared ? 'shared' : accountId || 'default'}|${shareToken || ''}|${nodeIdsSignature}`

        setDashboard(data)
        setHistoryRefreshKey((v) => {
          if (nextSignature !== lastNodeIdsRef.current) {
            lastNodeIdsRef.current = nextSignature
            return v + 1
          }
          return v
        })
      } catch (err) {
        showToast((err as Error).message || '加载失败', 'error')
      } finally {
        setLoading(false)
        setRefreshing(false)
      }
    },
    [accountId, shareToken, shared],
  )

  const loadShares = useCallback(async () => {
    if (shared || !isAdmin || !accountId) return
    setShareLoading(true)
    try {
      const res = await api.getMonitorShares(accountId)
      setShares(res.shares || [])
    } catch (err) {
      showToast('加载分享列表失败', 'error')
    } finally {
      setShareLoading(false)
    }
  }, [accountId, isAdmin, shared])

  useEffect(() => {
    if (!shared) {
      fetchAccounts()
    }
  }, [fetchAccounts, shared])

  useEffect(() => {
    if (shared) {
      if (shareToken) fetchDashboard()
    } else if (accountId) {
      fetchDashboard()
    }
  }, [accountId, fetchDashboard, shared, shareToken])

  useEffect(() => {
    if (!autoRefresh) return undefined
    const id = setInterval(() => fetchDashboard(false), refreshInterval)
    return () => clearInterval(id)
  }, [autoRefresh, fetchDashboard, refreshInterval])

  useEffect(() => {
    loadShares()
  }, [loadShares])

  useEffect(() => {
    setHealthEvents({})
  }, [accountId, shareToken])

  useEffect(() => {
    if (!lastMessage) return

    if (lastMessage.type === 'health_check') {
      const payload = lastMessage.payload
      setHealthEvents((prev) => ({
        ...prev,
        [payload.node_id]: {
          node_id: payload.node_id,
          check_time: payload.check_time,
          success: payload.success,
          response_time_ms: payload.response_time_ms ?? 0,
          error_message: payload.error_message || '',
          check_method: payload.check_method || 'api',
        },
      }))
      return
    }

    if (lastMessage.type !== 'node_status' && lastMessage.type !== 'node_metrics') return

    const payload = lastMessage.payload as typeof lastMessage.payload
    setDashboard((prev) => {
      if (!prev) return prev
      const idx = prev.nodes.findIndex((n) => n.id === payload.node_id)
      if (idx === -1) return prev
      const prevNode = prev.nodes[idx]

      const mergedTraffic = {
        success_rate: payload.traffic?.success_rate ?? payload.success_rate ?? prevNode.traffic?.success_rate ?? 0,
        avg_response_time:
          payload.traffic?.avg_response_time ?? payload.avg_response_time ?? prevNode.traffic?.avg_response_time ?? 0,
        total_requests: payload.traffic?.total_requests ?? payload.total_requests ?? prevNode.traffic?.total_requests ?? 0,
        failed_requests:
          payload.traffic?.failed_requests ?? payload.failed_requests ?? prevNode.traffic?.failed_requests ?? 0,
      }

      const mergedHealth = {
        status: (payload.health?.status ?? prevNode.health?.status ?? 'up') as MonitorNode['health']['status'],
        last_check_at:
          payload.health?.last_check_at ?? payload.timestamp ?? prevNode.health?.last_check_at ?? null,
        last_ping_ms: payload.health?.last_ping_ms ?? payload.last_ping_ms ?? prevNode.health?.last_ping_ms ?? 0,
        last_ping_err: payload.health?.last_ping_err ?? prevNode.health?.last_ping_err ?? '',
        check_method: (payload.health?.check_method ?? prevNode.health?.check_method ?? 'api').toLowerCase(),
      }

      const nextNode: MonitorNode = {
        ...prevNode,
        status: (payload.status as MonitorNode['status'] | undefined) || prevNode.status,
        last_error: payload.error ?? prevNode.last_error,
        traffic: mergedTraffic,
        health: mergedHealth,
      }

      const hasTrafficUpdate =
        payload.traffic?.success_rate !== undefined || payload.success_rate !== undefined
      const trendTs = payload.timestamp || mergedHealth.last_check_at || undefined

      if (hasTrafficUpdate && trendTs) {
        const merged = (prevNode.trend_24h || [])
          .filter((p) => p.timestamp !== trendTs)
          .concat({
            timestamp: trendTs,
            success_rate: mergedTraffic.success_rate,
            avg_time: mergedTraffic.avg_response_time,
          })
          .sort((a, b) => {
            const ta = parseToDate(a.timestamp)?.getTime() || 0
            const tb = parseToDate(b.timestamp)?.getTime() || 0
            return ta - tb
          })
          .slice(-96)
        nextNode.trend_24h = merged
      }
      const nextNodes = prev.nodes.slice()
      nextNodes[idx] = nextNode
      return {
        ...prev,
        nodes: nextNodes,
        updated_at: trendTs || prev.updated_at,
      }
    })
  }, [lastMessage])

  const aggregated = useMemo(() => {
    const list = dashboard?.nodes || []
    const totalRequests = list.reduce((acc, n) => acc + Number(n.traffic?.total_requests || 0), 0)
    const failedRequests = list.reduce((acc, n) => acc + Number(n.traffic?.failed_requests || 0), 0)
    const successRate = totalRequests > 0 ? ((totalRequests - failedRequests) / totalRequests) * 100 : 100
    const avgResponse =
      list.length > 0
        ? list.reduce((acc, n) => acc + Number(n.traffic?.avg_response_time || 0), 0) / list.length
        : 0
    const online = list.filter((n) => n.status === 'online').length
    const offline = list.filter((n) => n.status === 'offline' || n.status === 'degraded').length
    const disabled = list.filter((n) => n.status === 'disabled').length
    return { totalRequests, failedRequests, successRate, avgResponse, online, offline, disabled }
  }, [dashboard])

  const handleCreateShare = async () => {
    if (!accountId) {
      showToast('请选择账号', 'error')
      return
    }
    setShareLoading(true)
    try {
      const res = await api.createMonitorShare({ account_id: accountId, expire_in: expireIn })
      setShares((prev) => [res, ...prev])
      showToast('已生成分享链接')
    } catch (err) {
      showToast((err as Error).message || '创建失败', 'error')
    } finally {
      setShareLoading(false)
    }
  }

  const handleRevokeShare = async (id: string) => {
    try {
      await api.revokeMonitorShare(id)
      showToast('已撤销')
      loadShares()
    } catch (err) {
      showToast((err as Error).message || '撤销失败', 'error')
    }
  }

  const handleCopy = async (text: string) => {
    try {
      await navigator.clipboard.writeText(text)
      showToast('已复制链接')
    } catch (err) {
      showToast('复制失败', 'error')
    }
  }

  const resolvedShareUrl = (item: MonitorShare) =>
    item.share_url || `${window.location.origin}/monitor/share/${item.token}`

  const pageTitle = shared ? '共享监控大屏' : '监控大屏'
  const subTitle = shared ? '只读模式 · 通过分享链接查看实时状态' : '实时洞察节点状态与 24 小时趋势'

  return (
    <div className="monitor-page">
      <div className="monitor-header">
        <div>
          <h1>{pageTitle}</h1>
          <p className="sub">{subTitle}</p>
        </div>
        <div className="monitor-header-actions">
          <div className={`ws-pill ${connected ? 'on' : 'off'}`}>
            <span className="status-dot" />
            WebSocket {connected ? '已连接' : '未连接'}
          </div>
          <div className="monitor-settings">
            <label className="setting-toggle">
              <span>代理</span>
              <span className="toggle-mini">
                <input
                  type="checkbox"
                  checked={preference.showProxy}
                  onChange={(e) => setPreference({ showProxy: e.target.checked })}
                />
                <span className="slider" />
              </span>
            </label>
            <label className="setting-toggle">
              <span>探活</span>
              <span className="toggle-mini">
                <input
                  type="checkbox"
                  checked={preference.showHealth}
                  onChange={(e) => setPreference({ showHealth: e.target.checked })}
                />
                <span className="slider" />
              </span>
            </label>
            <button className="reset-btn" onClick={resetToDefault}>重置</button>
          </div>
          {!shared && (
            <label className="auto-refresh">
              <span className="toggle-mini">
                <input
                  type="checkbox"
                  checked={autoRefresh}
                  onChange={(e) => setAutoRefresh(e.target.checked)}
                />
                <span className="slider" />
              </span>
              自动刷新 {Math.round(refreshInterval / 1000)}s
            </label>
          )}
          <button className="btn ghost" type="button" onClick={() => fetchDashboard()} disabled={refreshing}>
            {refreshing ? '刷新中...' : '立即刷新'}
          </button>
        </div>
      </div>

      {!shared && (
        <Card>
          <div className="monitor-toolbar">
            {isAdmin && (
              <label>
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
            )}
            <div className="toolbar-spacer" />
            <div className="muted">
              更新于：{dashboard?.updated_at ? formatBeijingTime(dashboard.updated_at) : '--'}
            </div>
          </div>
        </Card>
      )}

      <Card
        title="全局指标"
        extra={
          loading || !dashboard ? null : (
            <div className="badge gray">
              总节点 {dashboard.nodes.length} · 在线 {aggregated.online} · 离线 {aggregated.offline}
            </div>
          )
        }
      >
        {loading || !dashboard ? (
          <div className="kpi-grid">
            {Array.from({ length: 4 }).map((_, i) => (
              <div className="stat-card glass skeleton" key={i}>
                <div className="skeleton-block" style={{ height: '80px' }} />
              </div>
            ))}
          </div>
        ) : (
          <div className="kpi-grid">
            <div className="stat-card glass">
              <span className="muted-title">成功率</span>
              <div className="kpi-main">{aggregated.successRate.toFixed(1)}%</div>
              <div className={`badge ${aggregated.successRate < 90 ? 'warn' : 'green'}`}>
                {aggregated.failedRequests.toLocaleString('en-US')} 失败
              </div>
            </div>
            <div className="stat-card glass">
              <span className="muted-title">平均响应</span>
              <div className="kpi-main">{Math.round(aggregated.avgResponse)} ms</div>
              <div className="badge gray">近 24h</div>
            </div>
            <div className="stat-card glass">
              <span className="muted-title">请求总数</span>
              <div className="kpi-main">{aggregated.totalRequests.toLocaleString('en-US')}</div>
              <div className="badge gray">累计</div>
            </div>
            <div className="stat-card glass">
              <span className="muted-title">状态分布</span>
              <div className="kpi-main">
                🟢 {aggregated.online} / 🔴 {aggregated.offline} / ⏸ {aggregated.disabled}
              </div>
              <div className="badge gray">在线 / 离线 / 停用</div>
            </div>
          </div>
        )}
      </Card>

      <Card title="节点实时状态" extra={<div className="badge gray">每 30 秒自动刷新 · WebSocket 增量更新</div>}>
        {loading ? (
          <div className="nodes-grid">
            {Array.from({ length: 3 }).map((_, i) => (
              <div className="monitor-card skeleton" key={i}>
                <div className="skeleton-block" />
              </div>
            ))}
          </div>
        ) : dashboard && dashboard.nodes.length > 0 ? (
          <div className="nodes-grid">
            {dashboard.nodes.map((node) => (
			<NodeCard
				key={node.id}
				node={node}
				historyRefreshKey={historyRefreshKey}
				healthEvent={healthEvents[node.id]}
				shareToken={shareToken}
			/>
            ))}
          </div>
        ) : (
          <div className="empty">暂无节点数据</div>
        )}
      </Card>

      {!shared && isAdmin && (
        <Card title="分享大屏" extra={<small className="muted">生成只读链接，便于团队查看</small>}>
          <div className="share-toolbar">
            <label>
              过期时间
              <select value={expireIn} onChange={(e) => setExpireIn(e.target.value as CreateMonitorShareRequest['expire_in'])}>
                <option value="1h">1 小时</option>
                <option value="24h">24 小时</option>
                <option value="168h">7 天</option>
                <option value="permanent">永久</option>
              </select>
            </label>
            <button className="btn primary" type="button" onClick={handleCreateShare} disabled={shareLoading || !accountId}>
              {shareLoading ? '生成中...' : '生成分享链接'}
            </button>
          </div>

          <div className="share-list">
            <div className="share-row share-head">
              <span>链接</span>
              <span>状态</span>
              <span>截止</span>
              <span>操作</span>
            </div>
            {shareLoading ? (
              <div className="share-row">
                <div className="skeleton-block" style={{ width: '100%' }} />
              </div>
            ) : shares.length === 0 ? (
              <div className="share-row">暂无分享</div>
            ) : (
              shares.map((s) => {
                const expired = s.expire_at ? (parseToDate(s.expire_at)?.getTime() || 0) < Date.now() : false
                const url = resolvedShareUrl(s)
                return (
                  <div className="share-row" key={s.id}>
                    <div className="share-link" title={url}>{url}</div>
                    <div className="share-status">
                      {s.revoked ? (
                        <span className="pill danger">已撤销</span>
                      ) : expired ? (
                        <span className="pill warn">已过期</span>
                      ) : (
                        <span className="pill ok">有效</span>
                      )}
                    </div>
                    <div>{s.expire_at ? formatBeijingTime(s.expire_at) : '永久'}</div>
                    <div className="share-actions">
                      <button className="btn ghost" type="button" onClick={() => handleCopy(url)}>
                        复制
                      </button>
                      <button className="btn danger" type="button" onClick={() => handleRevokeShare(s.id)} disabled={s.revoked}>
                        撤销
                      </button>
                    </div>
                  </div>
                )
              })
            )}
          </div>
        </Card>
      )}

      <Toast message={toast?.message} type={toast?.type} />
    </div>
  )
}
