import type { HealthCheckRecord, MonitorNode } from '../types'
import { formatBeijingTime } from '../utils/date'
import HealthTimeline from './HealthTimeline'
import './NodeCard.css'

interface NodeCardProps {
	node: MonitorNode
	historyRefreshKey: number
	healthEvent?: HealthCheckRecord | null
	shareToken?: string
}

const statusLabel: Record<string, string> = {
  online: '在线',
  degraded: '注意',
  offline: '离线',
  disabled: '停用',
  unknown: '未知',
}

export default function NodeCard({ node, historyRefreshKey, healthEvent, shareToken }: NodeCardProps) {
	const resolvedStatus = node.disabled ? 'disabled' : node.status || 'unknown'
	const trafficStatus = resolvedStatus
	const successRate = Number(node.traffic?.success_rate ?? 0)
	const avgTime = Number(node.traffic?.avg_response_time ?? 0)
	const totalReq = Number(node.traffic?.total_requests ?? 0)
	const failedReq = Number(node.traffic?.failed_requests ?? 0)
	const lastCheck = node.health?.last_check_at ? formatBeijingTime(node.health.last_check_at) : '暂无'
	const lastError = (node.last_error || node.health?.last_ping_err || '').trim()
	const healthLabel: Record<string, string> = { up: '正常', down: '失败', stale: '过期' }
	const healthStatus = node.health?.status || 'stale'
	const checkMethod = (node.health?.check_method || 'api').toUpperCase()
	const lastPing = node.health?.last_ping_ms ?? 0

	return (
		<div className="node-card">
			<div className="node-card__header">
				<div className="node-card__title-wrap">
					<div className="node-card__title">{node.name || '未命名节点'}</div>
					<div className="node-card__url">{node.url || '-'}</div>
				</div>
				<div className="node-card__status-badges">
					<span className={`status-badge traffic ${trafficStatus}`}>
						流量 · {statusLabel[trafficStatus] || trafficStatus}
					</span>
					<span className={`status-badge health ${healthStatus}`}>
						探活 · {healthLabel[healthStatus] || healthStatus}
					</span>
				</div>
			</div>

			<div className="node-card__metrics-grid">
				<div className="node-card__section traffic">
					<div className="section-title">代理流量</div>
					<div className="section-content">
						<div className="metric-item">
							<span className="label">成功率</span>
							<strong>{successRate.toFixed(1)}%</strong>
						</div>
						<div className="metric-item">
							<span className="label">平均延时</span>
							<strong>{avgTime ? `${avgTime} ms` : '--'}</strong>
						</div>
						<div className="metric-item">
							<span className="label">请求数</span>
							<strong>{totalReq.toLocaleString()}</strong>
						</div>
						<div className="metric-item">
							<span className="label">失败数</span>
							<strong className={failedReq > 0 ? 'danger' : ''}>
								{failedReq.toLocaleString()}
							</strong>
						</div>
					</div>
				</div>

				<div className="node-card__section health">
					<div className="section-title">健康检查</div>
					<div className="section-content">
						<div className="metric-item">
							<span className="label">状态</span>
							<strong className={`health-${healthStatus}`}>
								{healthLabel[healthStatus] || '未知'}
							</strong>
						</div>
						<div className="metric-item">
							<span className="label">最近检查</span>
							<strong>{lastCheck}</strong>
						</div>
						<div className="metric-item">
							<span className="label">探活延时</span>
							<strong>{lastPing ? `${lastPing} ms` : '--'}</strong>
						</div>
						<div className="metric-item">
							<span className="label">检查方式</span>
							<strong>{checkMethod}</strong>
						</div>
					</div>
				</div>
			</div>

			<HealthTimeline
				nodeId={node.id}
				refreshKey={historyRefreshKey}
				latest={healthEvent}
				shareToken={shareToken}
			/>

			<div className="node-card__footer">
				<div className="node-card__badges">
					{node.is_active && <span className="badge badge-primary">当前主用</span>}
					{node.disabled && <span className="badge badge-muted">已停用</span>}
				</div>
				{lastError && <div className="node-card__error">最后错误：{lastError}</div>}
			</div>
		</div>
	)
}
