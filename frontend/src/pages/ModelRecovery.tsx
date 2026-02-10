import { useCallback, useEffect, useRef, useState } from 'react'
import Card from '../components/Card'
import Toast from '../components/Toast'
import { useAuth } from '../hooks/useAuth'
import api from '../services/api'
import type { Account, ModelRecoveryItem } from '../types'
import './ModelRecovery.css'

const AUTO_REFRESH_INTERVAL = 15000 // 15s

export default function ModelRecovery() {
  const { isAdmin } = useAuth()
  const [items, setItems] = useState<ModelRecoveryItem[]>([])
  const [total, setTotal] = useState(0)
  const [loading, setLoading] = useState(true)
  const [accounts, setAccounts] = useState<Account[]>([])
  const [selectedAccount, setSelectedAccount] = useState('')
  const [autoRefresh, setAutoRefresh] = useState(true)
  const [toast, setToast] = useState<{ message: string; type: 'success' | 'error' } | null>(null)
  const toastTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  const refreshTimerRef = useRef<ReturnType<typeof setInterval> | null>(null)

  const showToast = useCallback((message: string, type: 'success' | 'error' = 'success') => {
    if (toastTimerRef.current) clearTimeout(toastTimerRef.current)
    setToast({ message, type })
    toastTimerRef.current = setTimeout(() => setToast(null), 2200)
  }, [])

  const fetchData = useCallback(async () => {
    try {
      const data = await api.getModelRecovery(selectedAccount || undefined)
      setItems(data.items || [])
      setTotal(data.total || 0)
    } catch (err) {
      console.error('Failed to load model recovery data:', err)
    } finally {
      setLoading(false)
    }
  }, [selectedAccount])

  const loadAccounts = useCallback(async () => {
    if (!isAdmin) return
    try {
      setAccounts(await api.getAccounts())
    } catch (err) {
      console.error('Failed to load accounts:', err)
    }
  }, [isAdmin])

  useEffect(() => {
    loadAccounts()
  }, [loadAccounts])

  useEffect(() => {
    setLoading(true)
    fetchData()
  }, [fetchData])

  useEffect(() => {
    if (refreshTimerRef.current) clearInterval(refreshTimerRef.current)
    if (autoRefresh) {
      refreshTimerRef.current = setInterval(fetchData, AUTO_REFRESH_INTERVAL)
    }
    return () => {
      if (refreshTimerRef.current) clearInterval(refreshTimerRef.current)
    }
  }, [autoRefresh, fetchData])

  useEffect(() => {
    return () => {
      if (toastTimerRef.current) clearTimeout(toastTimerRef.current)
      if (refreshTimerRef.current) clearInterval(refreshTimerRef.current)
    }
  }, [])

  const handleDismiss = async (nodeId: string, modelId: string) => {
    try {
      await api.dismissModelRecovery(nodeId, modelId)
      showToast('已移除恢复跟踪')
      fetchData()
    } catch (err) {
      showToast('操作失败', 'error')
    }
  }

  // 统计信息
  const uniqueNodes = new Set(items.map(i => i.node_id)).size
  const uniqueModels = new Set(items.map(i => i.model_id)).size
  const maxOffline = items.length > 0 ? Math.max(...items.map(i => i.offline_sec)) : 0

  const formatMaxOffline = (sec: number): string => {
    if (sec < 60) return '< 1 分钟'
    if (sec < 3600) return `${Math.floor(sec / 60)} 分钟`
    if (sec < 86400) {
      const h = Math.floor(sec / 3600)
      const m = Math.floor((sec % 3600) / 60)
      return m > 0 ? `${h} 小时 ${m} 分钟` : `${h} 小时`
    }
    const d = Math.floor(sec / 86400)
    const h = Math.floor((sec % 86400) / 3600)
    return h > 0 ? `${d} 天 ${h} 小时` : `${d} 天`
  }

  return (
    <div className="model-recovery-page">
      {toast && <Toast message={toast.message} type={toast.type} />}

      <div className="model-recovery-header">
        <div style={{ display: 'flex', alignItems: 'center', gap: '10px' }}>
          <h1>模型恢复</h1>
          <span className={`model-recovery-count${total === 0 ? ' zero' : ''}`}>
            {total === 0 ? '全部正常' : `${total} 个恢复中`}
          </span>
        </div>
        <div className="model-recovery-toolbar">
          {isAdmin && accounts.length > 1 && (
            <select
              value={selectedAccount}
              onChange={e => setSelectedAccount(e.target.value)}
              style={{ height: '28px', fontSize: '12px' }}
            >
              <option value="">全部账号</option>
              {accounts.map(a => (
                <option key={a.id} value={a.id}>{a.name}</option>
              ))}
            </select>
          )}
          <label style={{ fontSize: '12px', display: 'flex', alignItems: 'center', gap: '4px', cursor: 'pointer' }}>
            <input
              type="checkbox"
              checked={autoRefresh}
              onChange={e => setAutoRefresh(e.target.checked)}
            />
            自动刷新
          </label>
          <button onClick={() => { setLoading(true); fetchData() }} style={{ fontSize: '12px', padding: '2px 10px' }}>
            刷新
          </button>
        </div>
      </div>

      {total > 0 && (
        <div className="model-recovery-summary">
          <div className="summary-card">
            <div className="label">恢复中模型</div>
            <div className={`value${total > 0 ? ' danger' : ''}`}>{total}</div>
          </div>
          <div className="summary-card">
            <div className="label">受影响节点</div>
            <div className="value">{uniqueNodes}</div>
          </div>
          <div className="summary-card">
            <div className="label">受影响模型</div>
            <div className="value">{uniqueModels}</div>
          </div>
          <div className="summary-card">
            <div className="label">最长离线</div>
            <div className={`value${maxOffline > 3600 ? ' danger' : ''}`}>
              {formatMaxOffline(maxOffline)}
            </div>
          </div>
        </div>
      )}

      <Card>
        {loading && items.length === 0 ? (
          <div className="model-recovery-empty">加载中...</div>
        ) : total === 0 ? (
          <div className="model-recovery-empty">
            <div className="empty-icon">&#10003;</div>
            <div>所有节点的所有模型运行正常</div>
            <div style={{ marginTop: '8px', fontSize: '12px' }}>
              当请求某个模型失败时，该模型会自动进入恢复跟踪
            </div>
          </div>
        ) : (
          <table className="model-recovery-table">
            <thead>
              <tr>
                <th>模型</th>
                <th>节点</th>
                <th>离线时长</th>
                <th>错误信息</th>
                <th>失败时间</th>
                <th>检查次数</th>
                <th>最后检查</th>
                <th>操作</th>
              </tr>
            </thead>
            <tbody>
              {items.map((item, idx) => (
                <tr key={`${item.node_id}-${item.model_id}-${idx}`}>
                  <td><span className="model-tag">{item.model_id}</span></td>
                  <td><span className="node-tag">{item.node_name}</span></td>
                  <td>
                    <span className={`offline-duration${item.offline_sec < 600 ? ' short' : ''}`}>
                      {item.offline_human}
                    </span>
                  </td>
                  <td>
                    <span className="error-text" title={item.error}>
                      {item.error}
                    </span>
                  </td>
                  <td style={{ fontSize: '12px', color: 'var(--color-text-muted)', whiteSpace: 'nowrap' }}>
                    {item.failed_at}
                  </td>
                  <td>
                    <span className="check-count">{item.check_count}</span>
                  </td>
                  <td style={{ fontSize: '12px', color: 'var(--color-text-muted)', whiteSpace: 'nowrap' }}>
                    {item.last_check || '--'}
                  </td>
                  <td>
                    <button
                      className="dismiss-btn"
                      onClick={() => handleDismiss(item.node_id, item.model_id)}
                      title="手动移除此恢复跟踪记录"
                    >
                      移除
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </Card>
    </div>
  )
}
