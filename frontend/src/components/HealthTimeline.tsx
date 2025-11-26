import { useCallback, useEffect, useMemo, useState } from 'react'
import { createPortal } from 'react-dom'
import api from '../services/api'
import type { HealthCheckRecord, HealthHistory } from '../types'
import { formatBeijingTime, parseToDate } from '../utils/date'
import './HealthTimeline.css'

type RangeKey = '24h' | '7d' | '30d'

const RANGE_WINDOWS: Record<RangeKey, number> = {
	'24h': 24 * 60 * 60 * 1000,
	'7d': 7 * 24 * 60 * 60 * 1000,
	'30d': 30 * 24 * 60 * 60 * 1000,
}

interface HealthTimelineProps {
	nodeId: string
	refreshKey?: number
	latest?: HealthCheckRecord | null
	shareToken?: string
}

function normalizeRecord(nodeId: string, rec: HealthCheckRecord): HealthCheckRecord {
	return {
		node_id: rec.node_id || nodeId,
		check_time: rec.check_time,
		success: rec.success,
		response_time_ms: typeof rec.response_time_ms === 'number' ? rec.response_time_ms : 0,
		error_message: rec.error_message || '',
		check_method: rec.check_method || 'api',
	}
}

export default function HealthTimeline({ nodeId, refreshKey = 0, latest, shareToken }: HealthTimelineProps) {
	const [range, setRange] = useState<RangeKey>('24h')
	const [history, setHistory] = useState<HealthHistory | null>(null)
	const [loading, setLoading] = useState(false)
	const [error, setError] = useState<string | null>(null)
	const [hover, setHover] = useState<{
		rec: HealthCheckRecord
		x: number
		y: number
	} | null>(null)

	const fetchHistory = useCallback(async () => {
		setLoading(true)
		setError(null)
		const now = new Date()
		const to = now.toISOString()
		const from = new Date(now.getTime() - RANGE_WINDOWS[range]).toISOString()
		try {
			const res = await api.getHealthHistory(nodeId, from, to, shareToken)
			setHistory(res)
		} catch (err) {
			setError((err as Error).message || '加载失败')
		} finally {
			setLoading(false)
		}
	}, [nodeId, range, shareToken])

	useEffect(() => {
		fetchHistory()
		// eslint-disable-next-line react-hooks/exhaustive-deps
	}, [fetchHistory, refreshKey])

	useEffect(() => {
		if (!latest || latest.node_id !== nodeId) return
		setHistory((prev) => {
			const normalized = normalizeRecord(nodeId, latest)
			const checkedAt = parseToDate(normalized.check_time)?.getTime() || Date.now()
			const earliest = Date.now() - RANGE_WINDOWS[range]
			if (!prev) {
				return {
					node_id: nodeId,
					from: new Date(earliest).toISOString(),
					to: normalized.check_time,
					total: 1,
					checks: [normalized],
				}
			}

			const exists = prev.checks.some((c) => c.check_time === normalized.check_time)
			if (exists) return prev

			const merged = [...prev.checks, normalized]
			merged.sort((a, b) => {
				const ta = parseToDate(a.check_time)?.getTime() || 0
				const tb = parseToDate(b.check_time)?.getTime() || 0
				return ta - tb
			})
			const filtered = merged.filter((c) => (parseToDate(c.check_time)?.getTime() || 0) >= earliest)
			return {
				...prev,
				to: new Date(Math.max(parseToDate(prev.to)?.getTime() || 0, checkedAt)).toISOString(),
				total: prev.total + 1,
				checks: filtered,
			}
		})
	}, [latest, nodeId, range])

	const stats = useMemo(() => {
		const checks = history?.checks || []
		return checks.reduce(
			(acc, cur) => {
				if (!cur.success) acc.fail += 1
				else if ((cur.response_time_ms || 0) >= 1000) acc.slow += 1
				else acc.ok += 1
				return acc
			},
			{ ok: 0, slow: 0, fail: 0 },
		)
	}, [history])

	const renderTooltip = () => {
		if (!hover) return null
		const { rec, x, y } = hover
		const status = !rec.success ? '失败' : rec.response_time_ms >= 1000 ? '较慢' : '正常'
		const tooltip = (
			<div className="health-tooltip" style={{ left: x + 12, top: y - 10 }}>
				<div className="health-tooltip__time">{formatBeijingTime(rec.check_time)}</div>
				<div className="health-tooltip__row">状态：{status}</div>
				<div className="health-tooltip__row">耗时：{rec.response_time_ms || 0} ms</div>
				<div className="health-tooltip__row">方式：{rec.check_method || 'api'}</div>
				{rec.error_message && <div className="health-tooltip__row">错误：{rec.error_message}</div>}
			</div>
		)

		if (typeof document === 'undefined') return tooltip
		return createPortal(tooltip, document.body)
	}

	const checks = history?.checks || []

	return (
		<div className="health-timeline">
			<div className="health-timeline__header">
				<div className="health-timeline__title">健康检查历史</div>
				<div className="health-timeline__actions">
					{(['24h', '7d', '30d'] as RangeKey[]).map((key) => (
						<button
							key={key}
							type="button"
							className={`chip ${range === key ? 'active' : ''}`}
							onClick={() => setRange(key)}
						>
							{key === '24h' ? '24 小时' : key === '7d' ? '7 天' : '30 天'}
						</button>
					))}
				</div>
			</div>

			<div className="health-timeline__summary">
				<span>总数 {history?.total ?? 0}</span>
				<span className="pill ok">正常 {stats.ok}</span>
				<span className="pill slow">较慢 {stats.slow}</span>
				<span className="pill fail">失败 {stats.fail}</span>
			</div>

			<div className={`health-track ${loading ? 'loading' : ''}`}>
				{loading && <div className="health-track__skeleton" />}
				{!loading && checks.length === 0 && <div className="health-empty">暂无数据</div>}
				{!loading &&
					checks.map((rec, idx) => {
						const color = !rec.success ? 'fail' : rec.response_time_ms >= 1000 ? 'slow' : 'ok'
						return (
							<div
								key={`${rec.check_time}-${idx}`}
								className={`health-dot ${color}`}
								onMouseEnter={(e) => setHover({ rec, x: e.clientX, y: e.clientY })}
								onMouseLeave={() => setHover(null)}
								title={formatBeijingTime(rec.check_time)}
							/>
						)
					})}
			</div>
			{error && <div className="health-error">{error}</div>}
			{renderTooltip()}
		</div>
	)
}
