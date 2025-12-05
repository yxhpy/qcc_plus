import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { createPortal } from 'react-dom'
import { request } from '../services/api'
import type { HealthCheckRecord, HealthHistory } from '../types'
import { formatBeijingTime, parseToDate } from '../utils/date'
import './HealthTimeline.css'

type RangeKey = '24h'

const RANGE_WINDOWS: Record<RangeKey, number> = {
	'24h': 24 * 60 * 60 * 1000,
}

const CACHE_TTL = 60 * 1000 // keep recent results for 1 minute to smooth bursts
const historyCache = new Map<string, { data: HealthHistory; ts: number }>()
const inFlightCache = new Map<string, { controller: AbortController; promise: Promise<HealthHistory> }>()

function buildCacheKey(nodeId: string, range: RangeKey, shareToken?: string, source?: string) {
	return [nodeId, range, shareToken || '', source || ''].join('__')
}

async function fetchHistoryWithAbort(
	nodeId: string,
	range: RangeKey,
	shareToken: string | undefined,
	source: string | undefined,
	signal: AbortSignal,
): Promise<HealthHistory> {
	const now = new Date()
	const to = now.toISOString()
	const from = new Date(now.getTime() - RANGE_WINDOWS[range]).toISOString()
	const params = new URLSearchParams()
	params.set('from', from)
	params.set('to', to)
	if (shareToken) params.set('share_token', shareToken)
	if (source) params.set('source', source)
	const qs = params.toString()
	const url = qs
		? `/api/nodes/${encodeURIComponent(nodeId)}/health-history?${qs}`
		: `/api/nodes/${encodeURIComponent(nodeId)}/health-history`
	return request<HealthHistory>(url, { signal })
}

interface HealthTimelineProps {
	nodeId: string
	refreshKey?: number
	latest?: HealthCheckRecord | null
	shareToken?: string
	/** 数据来源过滤：scheduled(周期检查)/recovery(故障恢复)/proxy_fail(代理失败)，空字符串表示所有来源 */
	source?: string
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

export default function HealthTimeline({ nodeId, refreshKey = 0, latest, shareToken, source = '' }: HealthTimelineProps) {
	const range: RangeKey = '24h' // 固定使用 24h
	const [history, setHistory] = useState<HealthHistory | null>(null)
	const [loading, setLoading] = useState(false)
	const [error, setError] = useState<string | null>(null)
	const [hover, setHover] = useState<{
		rec: HealthCheckRecord
		x: number
		y: number
	} | null>(null)

	const abortRef = useRef<AbortController | null>(null)
	const lastRefreshKeyRef = useRef(refreshKey)
	const mountedRef = useRef(true)

	const cacheKey = useMemo(() => buildCacheKey(nodeId, range, shareToken, source), [nodeId, range, shareToken, source])

	const fetchHistory = useCallback(
		async (force = false) => {
			const now = Date.now()
			const cached = !force ? historyCache.get(cacheKey) : undefined
			if (cached && now - cached.ts < CACHE_TTL) {
				setHistory(cached.data)
				setLoading(false)
				return
			}

			if (!force) {
				const inflight = inFlightCache.get(cacheKey)
				if (inflight) {
					setLoading(true)
					try {
						const data = await inflight.promise
						if (!mountedRef.current || inflight.controller.signal.aborted) return
						setHistory(data)
					} catch (err) {
						if (inflight.controller.signal.aborted) return
						setError((err as Error).message || '加载失败')
					} finally {
						if (!inflight.controller.signal.aborted) setLoading(false)
					}
					return
				}
			}

			if (abortRef.current) {
				abortRef.current.abort()
			}

			const controller = new AbortController()
			abortRef.current = controller
			setLoading(true)
			setError(null)

			const promise = fetchHistoryWithAbort(nodeId, range, shareToken, source, controller.signal)
				.then((res) => {
					historyCache.set(cacheKey, { data: res, ts: Date.now() })
					return res
				})
				.finally(() => {
					const inflight = inFlightCache.get(cacheKey)
					if (inflight?.controller === controller) inFlightCache.delete(cacheKey)
				})

			inFlightCache.set(cacheKey, { controller, promise })

			try {
				const data = await promise
				if (!controller.signal.aborted && mountedRef.current) {
					setHistory(data)
				}
			} catch (err) {
				if (controller.signal.aborted) return
				setError((err as Error).message || '加载失败')
			} finally {
				if (!controller.signal.aborted) setLoading(false)
			}
		},
		[cacheKey, nodeId, range, shareToken, source],
	)

	useEffect(() => {
		mountedRef.current = true
		return () => {
			mountedRef.current = false
			if (abortRef.current) abortRef.current.abort()
		}
	}, [])

	useEffect(() => {
		const force = refreshKey !== lastRefreshKeyRef.current
		lastRefreshKeyRef.current = refreshKey
		fetchHistory(force)
	}, [fetchHistory, refreshKey])

	useEffect(() => {
		if (!latest || latest.node_id !== nodeId) return
		setHistory((prev) => {
			const normalized = normalizeRecord(nodeId, latest)
			const checkedAt = parseToDate(normalized.check_time)?.getTime() || Date.now()
			const earliest = Date.now() - RANGE_WINDOWS[range]

			// 如果 history 还未加载，创建一个初始结构
			if (!prev) {
				return {
					node_id: nodeId,
					from: new Date(earliest).toISOString(),
					to: new Date(checkedAt).toISOString(),
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
		const result = checks.reduce(
			(acc, cur) => {
				if (!cur.success) acc.fail += 1
				else acc.ok += 1
				return acc
			},
			{ ok: 0, fail: 0 },
		)
		const total = result.ok + result.fail
		const healthRate = total > 0 ? ((result.ok / total) * 100).toFixed(1) : null
		return { ...result, healthRate }
	}, [history])

	const renderTooltip = () => {
		if (!hover) return null
		const { rec, x, y } = hover
		const status = rec.success ? '在线' : '离线'
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
	// 判断最新状态：使用最后一次检查的 success 字段
	const latestCheck = checks.length > 0 ? checks[checks.length - 1] : null
	const isOnline = latestCheck ? latestCheck.success : false

	return (
		<div className="health-timeline health-timeline--compact">
			<div className="health-timeline__header">
				<div className="health-timeline__status">
					<span className={`status-dot ${isOnline ? 'online' : 'offline'}`} />
					<span className={`status-text ${isOnline ? 'online' : 'offline'}`}>{isOnline ? '在线' : '离线'}</span>
					{stats.healthRate !== null && (
						<>
							<span className="sep">·</span>
							<span className="health-rate">24h健康率 <strong>{stats.healthRate}%</strong></span>
						</>
					)}
				</div>
			</div>

			<div className={`health-track ${loading ? 'loading' : ''}`}>
				{loading && <div className="health-track__skeleton" />}
				{!loading && checks.length === 0 && <div className="health-empty">暂无数据</div>}
				{!loading &&
					checks.map((rec, idx) => {
						const color = rec.success ? 'ok' : 'fail'
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
