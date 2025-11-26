import type { HealthCheckRecord, MonitorNode } from '../types'
import HealthTimeline from './HealthTimeline'
import Tooltip from './Tooltip'
import { useNodeMetrics } from '../contexts/NodeMetricsContext'
import './NodeCard.css'

interface NodeCardProps {
  node: MonitorNode
  historyRefreshKey: number
  healthEvent?: HealthCheckRecord | null
  shareToken?: string
}

export default function NodeCard({ node, historyRefreshKey, healthEvent, shareToken }: NodeCardProps) {
  const { preference } = useNodeMetrics()
  const successRate = Number(node.traffic?.success_rate ?? 0)
  const avgTime = Number(node.traffic?.avg_response_time ?? 0)
  const totalReq = Number(node.traffic?.total_requests ?? 0)
  const failedReq = Number(node.traffic?.failed_requests ?? 0)
  const lastError = (node.last_error || node.health?.last_ping_err || '').trim()
  const healthLabel: Record<string, string> = { up: '在线', down: '离线' }
  const rawHealthStatus = node.health?.status || 'down'
  const healthStatus = rawHealthStatus === 'up' ? 'up' : 'down'
  const checkMethod = (node.health?.check_method || 'api').toUpperCase()
  const lastPing = Number(node.health?.last_ping_ms ?? 0)
  const lastCheckShort = node.health?.last_check_at
    ? node.health.last_check_at.replace(/^\d{4}年\d{2}月\d{2}日\s*/, '')
    : '--'

  return (
    <div className={`node-card ${node.is_active ? 'node-card--active' : ''}`}>
      {lastError && (
        <div className="node-card__error-badge">
          <Tooltip content={lastError} trigger="both" maxWidth="300px">
            <span className="node-card__error-icon" role="img" aria-label="错误">⚠️</span>
          </Tooltip>
        </div>
      )}
      {/* Header: 节点名 + 双状态徽章 */}
      <div className="node-card__header">
        <div className="node-card__title-wrap">
          <div className="node-card__title">{node.name || '未命名节点'}</div>
          <div className="node-card__url">{node.url || '-'}</div>
        </div>
      </div>

      {/* 紧凑指标区 - 两行 */}
      <div className="node-card__metrics-compact">
        {preference.showProxy && (
          <div className="metrics-row traffic">
            <span className="row-label">代理</span>
            <span className="metric">成功率 <strong>{successRate.toFixed(1)}%</strong></span>
            <span className="sep">|</span>
            <span className="metric">延时 <strong>{avgTime ? avgTime : '--'}ms</strong></span>
            <span className="sep">|</span>
            <span className="metric">请求 <strong>{totalReq.toLocaleString()}</strong></span>
            <span className="metric secondary">/失败 <strong className={failedReq > 0 ? 'danger' : ''}>{failedReq.toLocaleString()}</strong></span>
          </div>
        )}
        {preference.showHealth && (
          <div className="metrics-row health">
            <span className="row-label">探活</span>
            <span className={`metric health-${healthStatus}`}><strong>{healthLabel[healthStatus]}</strong></span>
            <span className="sep">|</span>
            <span className="metric"><strong>{lastPing || '--'}ms</strong></span>
            <span className="metric secondary">({checkMethod})</span>
            <span className="sep">|</span>
            <span className="metric secondary">检查于 {lastCheckShort}</span>
          </div>
        )}
      </div>

      {/* 健康检查历史 - 保持原样但更紧凑 */}
      <HealthTimeline
        nodeId={node.id}
        refreshKey={historyRefreshKey}
        latest={healthEvent}
        shareToken={shareToken}
      />

      {/* Footer - 单行 */}
      <div className="node-card__footer">
        <div className="node-card__badges">
          {node.disabled && <span className="badge badge-muted">已停用</span>}
        </div>
      </div>
    </div>
  )
}
