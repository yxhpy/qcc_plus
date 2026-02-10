import { useCallback, useEffect, useMemo, useState } from 'react'
import { Bar, Doughnut } from 'react-chartjs-2'
import {
  ArcElement,
  BarElement,
  CategoryScale,
  Chart as ChartJS,
  Legend,
  LinearScale,
  Tooltip,
} from 'chart.js'
import Card from '../components/Card'
import RecoveryBadge from '../components/RecoveryBadge'
import Toast from '../components/Toast'
import api from '../services/api'
import type { Account, Node } from '../types'
import { formatBeijingTime } from '../utils/date'
import { useChartColors } from '../hooks/useChartColors'
import './Dashboard.css'

ChartJS.register(CategoryScale, LinearScale, BarElement, ArcElement, Tooltip, Legend)

function formatBytes(bytes?: number) {
  const b = Number(bytes || 0)
  if (!b) return '-'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let i = 0
  let val = b
  while (val >= 1024 && i < units.length - 1) {
    val /= 1024
    i++
  }
  const fixed = val >= 10 ? 1 : 2
  return `${val.toFixed(fixed)} ${units[i]}`
}

function formatNumber(num?: number) {
  const n = Number(num || 0)
  if (n >= 1e9) return `${(n / 1e9).toFixed(1)}B`
  if (n >= 1e6) return `${(n / 1e6).toFixed(1)}M`
  if (n >= 1e3) return `${(n / 1e3).toFixed(1)}K`
  return n.toLocaleString('en-US')
}

function calculateSuccessRate(node: Node) {
  const req = Number(node.requests || 0)
  const fail = Number(node.fail_count || 0)
  if (req <= 0) return 100
  return Math.max(0, ((req - fail) / req) * 100)
}

function bytesPerSecond(node: Node) {
  const total = Number(node.total_bytes || 0)
  const dur = Number(node.stream_dur_ms || 0)
  if (total <= 0 || dur <= 0) return 0
  return total / (dur / 1000)
}

function formatBps(bps: number) {
  if (bps <= 0) return '-'
  if (bps >= 1e6) return `${(bps / 1e6).toFixed(1)} MB/s`
  if (bps >= 1e3) return `${(bps / 1e3).toFixed(1)} KB/s`
  return `${bps.toFixed(0)} B/s`
}

function nodeHealthTone(node: Node) {
  if (node.disabled) return 'off'
  if (node.failed || Number(node.fail_streak || 0) > 3) return 'fail'
  if (Number(node.fail_streak || 0) > 0) return 'warn'
  return 'ok'
}

interface AlertItem {
  type: 'failed' | 'streak' | 'slow'
  node: string
  message: string
  raw?: string
}

export default function Dashboard() {
  const [accounts, setAccounts] = useState<Account[]>([])
  const [accountId, setAccountId] = useState('')
  const [nodes, setNodes] = useState<Node[]>([])
  const [loading, setLoading] = useState(true)
  const [toast, setToast] = useState<{ message: string; type: 'success' | 'error' } | null>(null)
  const [autoRefresh, setAutoRefresh] = useState(true)
  const [lastUpdated, setLastUpdated] = useState<Date | null>(null)
  const chartColors = useChartColors()

  const showToast = (message: string, type: 'success' | 'error' = 'success') => {
    setToast({ message, type })
    setTimeout(() => setToast(null), 2200)
  }

  const fetchNodes = useCallback(
    async (showSkeleton = true) => {
      if (!accountId) return
      if (showSkeleton) setLoading(true)
      try {
        const data = await api.getNodes(accountId)
        setNodes(data)
        setLastUpdated(new Date())
      } catch (err) {
        showToast((err as Error).message || '加载失败', 'error')
      } finally {
        setLoading(false)
      }
    },
    [accountId],
  )

  const fetchAccounts = useCallback(async () => {
    setLoading(true)
    try {
      const list = await api.getAccounts()
      setAccounts(list)
      setAccountId((prev) => (prev || (list[0]?.id ?? '')))
    } catch (err) {
      showToast('加载账号失败', 'error')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    fetchAccounts()
  }, [fetchAccounts])

  useEffect(() => {
    if (accountId) {
      fetchNodes()
    }
  }, [accountId, fetchNodes])

  useEffect(() => {
    if (!autoRefresh || !accountId) return undefined
    const id = setInterval(() => fetchNodes(false), 6000)
    return () => clearInterval(id)
  }, [autoRefresh, accountId, fetchNodes])

  const stats = useMemo(() => {
    let totalReq = 0
    let totalFail = 0
    let active = 0
    let totalBytes = 0
    let totalTokens = 0
    let throughputSum = 0
    let throughputCount = 0
    nodes.forEach((n) => {
      const req = Number(n.requests || 0)
      const fail = Number(n.fail_count || 0)
      totalReq += req
      totalFail += fail
      if (!n.disabled && !n.failed) active += 1
      const bps = bytesPerSecond(n)
      if (bps > 0) {
        throughputSum += bps
        throughputCount += 1
      }
      totalBytes += Number(n.total_bytes || 0)
      totalTokens += Number(n.input_tokens || 0) + Number(n.output_tokens || 0)
    })
    const successRate = totalReq > 0 ? ((totalReq - totalFail) / totalReq) * 100 : 100
    const avgThroughput = throughputCount > 0 ? throughputSum / throughputCount : 0
    const failRate = totalReq > 0 ? (totalFail / totalReq) * 100 : 0
    return {
      totalReq,
      totalFail,
      active,
      totalBytes,
      totalTokens,
      avgThroughput,
      successRate,
      failRate,
      throughputCount,
    }
  }, [nodes])

  const alerts: AlertItem[] = useMemo(() => {
    const list: AlertItem[] = []
    nodes.forEach((n) => {
      const bps = bytesPerSecond(n)
      if (n.failed) {
        list.push({ type: 'failed', node: n.name || '未命名', message: n.last_error || '未知原因', raw: n.last_error })
      }
      if (Number(n.fail_streak || 0) > 3) {
        list.push({ type: 'streak', node: n.name || '未命名', message: `连续失败 ${n.fail_streak} 次` })
      }
      if (bps > 0 && bps < 1000) {
        list.push({ type: 'slow', node: n.name || '未命名', message: `输出速率过低（${formatBps(bps)}）` })
      }
    })
    return list
  }, [nodes])

  const displayedAlerts = alerts.slice(0, 3)

  const throughputChart = useMemo(() => {
    const throughputColor = (bps: number) => {
      if (bps >= 3000) return chartColors.success
      if (bps >= 1000) return chartColors.warning
      return chartColors.danger
    }

    const data = nodes
      .map((n) => ({ label: n.name || '未命名', value: bytesPerSecond(n) }))
      .sort((a, b) => b.value - a.value)
    const labels = data.length ? data.map((d) => d.label) : ['暂无数据']
    const values = data.length ? data.map((d) => d.value) : [0]
    const colors = values.map((v) => throughputColor(v))
    return {
      data: {
        labels,
        datasets: [
          {
            data: values,
            backgroundColor: colors,
            borderRadius: 6,
          },
        ],
      },
      options: {
        indexAxis: 'y' as const,
        scales: {
          x: { grid: { color: chartColors.gridColor }, title: { display: true, text: '字节/秒' } },
          y: { ticks: { color: chartColors.textSecondary } },
        },
        plugins: {
          legend: { display: false },
          tooltip: {
            callbacks: {
              label(ctx: any) {
                return formatBps(Number(ctx.raw))
              },
            },
          },
        },
        maintainAspectRatio: false,
      },
    }
  }, [nodes, chartColors])

  const requestChart = useMemo(() => {
    const sorted = nodes.slice().sort((a, b) => Number(b.requests || 0) - Number(a.requests || 0))
    const top = sorted.slice(0, 5)
    const others = sorted.slice(5)
    const dataArr = top.map((n) => Number(n.requests || 0))
    const labelsArr = top.map((n) => n.name || '未命名')
    const otherSum = others.reduce((acc, n) => acc + Number(n.requests || 0), 0)
    if (otherSum > 0) {
      dataArr.push(otherSum)
      labelsArr.push('其他')
    }
    if (!dataArr.length) {
      dataArr.push(0)
      labelsArr.push('暂无数据')
    }
    const palette = chartColors.palette
    const colors = labelsArr.map((_, i) => palette[i % palette.length])
    return {
      data: {
        labels: labelsArr,
        datasets: [
          {
            data: dataArr,
            backgroundColor: colors,
            borderWidth: 1,
          },
        ],
      },
      options: {
        plugins: {
          legend: { position: 'bottom' as const },
          tooltip: {
            callbacks: {
              label(ctx: any) {
                const dataset = (ctx.dataset.data as Array<number | string>) || []
                const total = dataset.reduce<number>((acc, item) => acc + Number(item), 0) || 1
                const val = Number(ctx.raw)
                const percent = ((val / total) * 100).toFixed(1)
                return `${ctx.label}: ${val.toLocaleString('en-US')} (${percent}%)`
              },
            },
          },
        },
        maintainAspectRatio: false,
      },
    }
  }, [nodes, chartColors])

  const handleActivate = async (id: string) => {
    try {
      await api.activateNode(id)
      showToast('已设为活跃')
      fetchNodes(false)
    } catch (err) {
      showToast((err as Error).message || '切换失败', 'error')
    }
  }

  const successBadgeTone = stats.successRate < 90 ? 'warn' : 'green'
  const failBadgeTone = stats.failRate > 5 ? 'warn' : 'gray'
  const throughputBadgeTone = stats.avgThroughput >= 3000 ? 'green' : stats.avgThroughput >= 1000 ? 'gray' : 'warn'

  return (
    <div>
      <h1>仪表盘</h1>
      <p className="sub">全景洞察节点健康、流量与性能，支持 6 秒自动刷新。</p>

      <Card>
        <div className="toolbar">
          <label style={{ minWidth: 220 }}>
            选择账号
            <select value={accountId} onChange={(e) => setAccountId(e.target.value)}>
              {accounts.map((a) => (
                <option key={a.id} value={a.id}>
                  {a.name}
                  {a.is_admin ? ' [管理员]' : ''}
                </option>
              ))}
            </select>
          </label>
          <div className="spacer" />
          <label className="auto-refresh">
            <input
              type="checkbox"
              checked={autoRefresh}
              onChange={(e) => setAutoRefresh(e.target.checked)}
            />
            自动刷新 6s
          </label>
          <button className="btn ghost" type="button" onClick={() => fetchNodes()} disabled={loading}>
            立即刷新
          </button>
        </div>
        <div className="kpi-grid">
          <div className="stat-card">
            <span className="muted-title">总请求数</span>
            <div className={`kpi-main ${loading ? 'skeleton' : ''}`}>{stats.totalReq.toLocaleString('en-US')}</div>
            <div className={`badge ${successBadgeTone}`}>成功率 {stats.successRate.toFixed(1)}%</div>
            <small className="muted">失败 {stats.totalFail.toLocaleString('en-US')} ({stats.failRate.toFixed(1)}%)</small>
          </div>
          <div className="stat-card">
            <span className="muted-title">活跃节点数</span>
            <div className={`kpi-main ${loading ? 'skeleton' : ''}`}>{stats.active} / {nodes.length}</div>
            <div className="badge gray">运行中</div>
            <small className="muted">活跃 / 总数</small>
          </div>
          <div className="stat-card">
            <span className="muted-title">平均输出速率</span>
            <div className={`kpi-main ${loading ? 'skeleton' : ''}`}>{formatBps(stats.avgThroughput)}</div>
            <div className={`badge ${throughputBadgeTone}`}>输出速率</div>
            <small className="muted">{stats.throughputCount > 0 ? `基于 ${stats.throughputCount} 个节点` : '等待更多数据'}</small>
          </div>
          <div className="stat-card">
            <span className="muted-title">Token 消耗</span>
            <div className={`kpi-main ${loading ? 'skeleton' : ''}`}>{formatNumber(stats.totalTokens)}</div>
            <div className="badge warn">输入 + 输出</div>
            <small className="muted">累计 Tokens</small>
          </div>
          <div className="stat-card">
            <span className="muted-title">总流量</span>
            <div className={`kpi-main ${loading ? 'skeleton' : ''}`}>{formatBytes(stats.totalBytes)}</div>
            <div className="badge gray">带宽</div>
            <small className="muted">累计字节</small>
          </div>
          <div className="stat-card">
            <span className="muted-title">失败请求数</span>
            <div className={`kpi-main ${loading ? 'skeleton' : ''}`}>{stats.totalFail.toLocaleString('en-US')}</div>
            <div className={`badge ${failBadgeTone}`}>失败率 {stats.failRate.toFixed(1)}%</div>
            <small className="muted">失败次数 / 请求数</small>
          </div>
        </div>
      </Card>

      <Card
        title="节点性能与请求分布"
        extra={<div className="badge gray">{lastUpdated ? `更新于 ${formatBeijingTime(lastUpdated)}` : '--'}</div>}
      >
        <div className="chart-grid">
          <Card
            className="chart-card"
            title="节点性能对比"
            extra={<div className="chart-legend">🟢 &gt;3KB/s · 🟡 1-3KB/s · 🔴 &lt;1KB/s</div>}
          >
            <div className={loading ? 'chart-skeleton' : 'chart-body'}>
              {loading ? <div className="skeleton" style={{ height: 14 }} /> : <Bar data={throughputChart.data} options={throughputChart.options} />}
            </div>
          </Card>
          <Card className="chart-card" title="节点请求分布" extra={<div className="badge gray">总请求 {stats.totalReq.toLocaleString('en-US')}</div>}>
            <div className={loading ? 'chart-skeleton' : 'chart-body'}>
              {loading ? <div className="skeleton" style={{ height: 14 }} /> : <Doughnut data={requestChart.data} options={requestChart.options} />}
            </div>
          </Card>
        </div>
      </Card>

      {alerts.length > 0 && (
        <Card title="告警" extra={<span className="pill-small" style={{ background: '#fee2e2', color: '#b91c1c' }}>{alerts.length}</span>}>
          <div className="alert-list">
            {displayedAlerts.map((alert, idx) => (
              <div className="alert-card" key={idx}>
                <div className="alert-title">
                  <strong>{alert.node}</strong>: {alert.message}
                </div>
                {alert.type === 'failed' && alert.raw && String(alert.raw).length > 50 && (
                  <details>
                    <summary>查看完整错误</summary>
                    <pre>{alert.raw}</pre>
                  </details>
                )}
              </div>
            ))}
          </div>
        </Card>
      )}

      <Card title="节点健康列表" extra={<small className="muted">按请求数排序，成功率低于 90% 标红。</small>}>
        <div className="table-wrapper">
          <table className="health-table">
            <thead>
              <tr>
                <th>节点名</th>
                <th>状态</th>
                <th>请求数</th>
                <th>成功率</th>
                <th>输出速率</th>
                <th>Tokens</th>
                <th>操作</th>
              </tr>
            </thead>
            <tbody>
              {loading ? (
                <tr>
                  <td colSpan={7}>
                    <div className="skeleton" style={{ height: 18 }} />
                  </td>
                </tr>
              ) : nodes.length === 0 ? (
                <tr>
                  <td colSpan={7}>暂无节点</td>
                </tr>
              ) : (
                nodes
                  .slice()
                  .sort((a, b) => Number(b.requests || 0) - Number(a.requests || 0))
                  .map((n) => {
                    const success = calculateSuccessRate(n)
                    const bps = bytesPerSecond(n)
                    const tokens = Number(n.input_tokens || 0) + Number(n.output_tokens || 0)
                    const tone = nodeHealthTone(n)
                    const statusLabel = tone === 'ok' ? '正常' : tone === 'fail' ? '失败' : tone === 'off' ? '已禁用' : '警告'
                    return (
                      <tr key={n.id}>
                        <td>
                          {n.name || '未命名'}<RecoveryBadge nodeId={n.id} />
                          <div className="muted" style={{ fontSize: 12 }}>权重 {n.weight || 1}</div>
                        </td>
                        <td>
                          <span className={`tag ${tone}`}>{statusLabel}</span>
                        </td>
                        <td>{Number(n.requests || 0).toLocaleString('en-US')}</td>
                        <td>
                          <span className={success < 90 ? 'danger-text' : ''}>{success.toFixed(1)}%</span>
                        </td>
                        <td>{formatBps(bps)}</td>
                        <td>{formatNumber(tokens)}</td>
                        <td>
                          <div className="table-actions">
                            <button className="btn ghost" type="button" onClick={() => handleActivate(n.id)} disabled={n.disabled}>
                              设为活跃
                            </button>
                          </div>
                        </td>
                      </tr>
                    )
                  })
              )}
            </tbody>
          </table>
        </div>
      </Card>

      <Toast message={toast?.message} type={toast?.type} />
    </div>
  )
}
