import { useEffect, useState } from 'react'
import { useParams } from 'react-router-dom'
import NodeCard from '../components/NodeCard'
import type { HealthCheckRecord, MonitorDashboard } from '../types'
import api from '../services/api'
import { useMonitorWebSocket } from '../hooks/useMonitorWebSocket'
import { formatBeijingTime, parseToDate } from '../utils/date'
import './SharedMonitor.css'

export default function SharedMonitor() {
  const { token } = useParams<{ token: string }>()
  const [dashboard, setDashboard] = useState<MonitorDashboard | null>(null)
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(true)
  const [historyRefreshKey, setHistoryRefreshKey] = useState(0)
  const [healthEvents, setHealthEvents] = useState<Record<string, HealthCheckRecord>>({})

  const { connected, lastMessage } = useMonitorWebSocket(undefined, token)

  useEffect(() => {
    async function fetchData() {
      if (!token) {
        setError('无效的分享链接')
        setLoading(false)
        return
      }
      try {
        const data = await api.getSharedMonitor(token)
        setDashboard(data)
        setHistoryRefreshKey((v) => v + 1)
        document.title = `${data.account_name} - 监控大屏`
      } catch (err) {
        setError('分享链接无效或已过期')
      } finally {
        setLoading(false)
      }
    }
    fetchData()
  }, [token])

  useEffect(() => {
    setHealthEvents({})
  }, [token])

  // 处理 WebSocket 实时更新
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

    const payload = lastMessage.payload
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
        status: (payload.health?.status ?? prevNode.health?.status ?? 'up') as typeof prevNode.health.status,
        last_check_at: payload.health?.last_check_at ?? payload.timestamp ?? prevNode.health?.last_check_at ?? null,
        last_ping_ms: payload.health?.last_ping_ms ?? payload.last_ping_ms ?? prevNode.health?.last_ping_ms ?? 0,
        last_ping_err: payload.health?.last_ping_err ?? prevNode.health?.last_ping_err ?? '',
        check_method: (payload.health?.check_method ?? prevNode.health?.check_method ?? 'api').toLowerCase(),
      }

      const nextNode = {
        ...prevNode,
        status: (payload.status as typeof prevNode.status | undefined) || prevNode.status,
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

  if (loading) {
    return <div className="shared-monitor-loading">加载中...</div>
  }

  if (error) {
    return (
      <div className="shared-monitor-error">
        <h2>{error}</h2>
        <p>请检查分享链接是否正确，或联系分享者获取新的链接。</p>
      </div>
    )
  }

  return (
    <div className="shared-monitor-page">
      <header>
        <h1>{dashboard?.account_name} · 监控大屏</h1>
        <div className="header-info">
          <span className={`ws-status ${connected ? 'connected' : 'disconnected'}`}>
            {connected ? '● 实时更新' : '○ 未连接'}
          </span>
          <p className="readonly-notice">
            只读模式 · 最近更新：{formatBeijingTime(dashboard?.updated_at)}
          </p>
        </div>
      </header>

      <div className="nodes-grid">
        {dashboard?.nodes.map((node) => (
          <NodeCard
            key={node.id}
            node={node}
            historyRefreshKey={historyRefreshKey}
            healthEvent={healthEvents[node.id]}
            shareToken={token}
          />
        ))}
      </div>

      <footer>
        <p>Powered by qcc_plus</p>
      </footer>
    </div>
  )
}
