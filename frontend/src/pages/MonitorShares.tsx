import { useCallback, useEffect, useMemo, useState } from 'react'
import Card from '../components/Card'
import Modal from '../components/Modal'
import Toast from '../components/Toast'
import useDialog from '../hooks/useDialog'
import { useAuth } from '../hooks/useAuth'
import api from '../services/api'
import type { Account, CreateMonitorShareRequest, MonitorShare } from '../types'
import { formatBeijingTime } from '../utils/date'
import './MonitorShares.css'

const PAGE_SIZE = 20

export default function MonitorShares() {
  const { isAdmin, user, loading: authLoading } = useAuth()
  const dialog = useDialog()
  const [accounts, setAccounts] = useState<Account[]>([])
  const [accountId, setAccountId] = useState('')
  const [shares, setShares] = useState<MonitorShare[]>([])
  const [total, setTotal] = useState(0)
  const [hasNext, setHasNext] = useState(false)
  const [page, setPage] = useState(1)
  const [loading, setLoading] = useState(true)
  const [showCreateModal, setShowCreateModal] = useState(false)
  const [creating, setCreating] = useState(false)
  const [toast, setToast] = useState<{ message: string; type: 'success' | 'error' } | null>(null)

  const showToast = (message: string, type: 'success' | 'error' = 'success') => {
    setToast({ message, type })
    setTimeout(() => setToast(null), 2200)
  }

  const loadAccounts = useCallback(async () => {
    if (!isAdmin) return
    try {
      const list = await api.getAccounts()
      setAccounts(list)
      setAccountId((prev) => prev || (list[0]?.id ?? ''))
    } catch (err) {
      showToast('加载账号列表失败', 'error')
    }
  }, [isAdmin])

  useEffect(() => {
    if (authLoading) return
    if (isAdmin) {
      loadAccounts()
    } else if (user?.id) {
      setAccountId(user.id)
    }
  }, [authLoading, isAdmin, user, loadAccounts])

  useEffect(() => {
    setPage(1)
  }, [accountId])

  const fetchShares = useCallback(async () => {
    if (authLoading) return
    if (isAdmin && !accountId) return
    setLoading(true)
    try {
      const { shares: list, total: resTotal } = await api.getMonitorShares(
        isAdmin ? accountId : undefined,
        PAGE_SIZE,
        (page - 1) * PAGE_SIZE,
      )
      setShares(list)
      const estimatedTotal =
        typeof resTotal === 'number'
          ? resTotal
          : (page - 1) * PAGE_SIZE + list.length + (list.length === PAGE_SIZE ? 1 : 0)
      setTotal(estimatedTotal)
      setHasNext(list.length === PAGE_SIZE)
    } catch (err) {
      showToast((err as Error).message || '加载分享链接失败', 'error')
    } finally {
      setLoading(false)
    }
  }, [accountId, authLoading, isAdmin, page])

  useEffect(() => {
    fetchShares()
  }, [fetchShares])

  const totalPages = useMemo(() => {
    const pages = total > 0 ? Math.ceil(total / PAGE_SIZE) : shares.length > 0 ? 1 : 1
    return pages
  }, [total, shares.length])

  const handleCreate = async (expireIn: CreateMonitorShareRequest['expire_in']) => {
    if (creating) return
    setCreating(true)
    try {
      const payload: CreateMonitorShareRequest = {
        expire_in: expireIn,
      }
      if (isAdmin && accountId) {
        payload.account_id = accountId
      }
      const share = await api.createMonitorShare(payload)
      showToast('分享链接创建成功')
      setShowCreateModal(false)
      await fetchShares()

      const url = share.share_url || `${window.location.origin}/monitor/share/${share.token}`
      try {
        await navigator.clipboard.writeText(url)
        showToast('分享链接已复制到剪贴板')
      } catch (err) {
        showToast('已生成链接，请手动复制', 'error')
      }
    } catch (error) {
      showToast((error as Error).message || '创建失败', 'error')
    } finally {
      setCreating(false)
    }
  }

  const handleRevoke = async (id: string) => {
    const ok = await dialog.confirm({
      title: '确认撤销',
      message: '撤销后链接将不可访问，确定继续？',
    })
    if (!ok) return
    try {
      await api.revokeMonitorShare(id)
      showToast('分享链接已撤销')
      fetchShares()
    } catch (error) {
      showToast((error as Error).message || '撤销失败', 'error')
    }
  }

  const handleCopy = async (share: MonitorShare) => {
    const url = share.share_url || `${window.location.origin}/monitor/share/${share.token}`
    try {
      await navigator.clipboard.writeText(url)
      showToast('分享链接已复制')
    } catch (err) {
      showToast('复制失败，请手动复制', 'error')
    }
  }

  const isExpired = (share: MonitorShare) => {
    if (!share.expire_at) return false
    const date = new Date(share.expire_at)
    return Number.isNaN(date.getTime()) ? false : date < new Date()
  }

  const isInactive = (share: MonitorShare) => share.revoked || isExpired(share)

  const shortToken = (token?: string) => {
    if (!token) return '--'
    return token.length > 16 ? `${token.slice(0, 16)}...` : token
  }

  const shortSharePath = (token?: string) => {
    if (!token) return '--'
    const short = token.length > 8 ? `${token.slice(0, 8)}...` : token
    return `/monitor/share/${short}`
  }

  const disableNext = !hasNext && page >= totalPages

  return (
    <div className="monitor-shares-page">
      <div className="monitor-shares-header">
        <div>
          <h1>监控分享链接</h1>
          <p className="sub">创建只读分享链接，让外部用户也能查看监控大屏。</p>
        </div>
        <div className="monitor-shares-actions">
          {isAdmin && (
            <select value={accountId} onChange={(e) => setAccountId(e.target.value)}>
              {accounts.map((acc) => (
                <option key={acc.id} value={acc.id}>
                  {acc.name}
                </option>
              ))}
            </select>
          )}
          <button className="btn primary" type="button" onClick={() => setShowCreateModal(true)}>
            ➕ 创建分享链接
          </button>
        </div>
      </div>

      <Card title="分享链接列表">
        <table className="shares-table">
          <thead>
            <tr>
              <th>Token</th>
              <th>分享 URL</th>
              <th>创建时间</th>
              <th>过期时间</th>
              <th>状态</th>
              <th style={{ minWidth: 160 }}>操作</th>
            </tr>
          </thead>
          <tbody>
            {loading ? (
              <tr>
                <td colSpan={6}>
                  <div className="table-skeleton" />
                </td>
              </tr>
            ) : shares.length === 0 ? (
              <tr>
                <td colSpan={6} style={{ textAlign: 'center', color: 'var(--color-text-muted)' }}>
                  暂无分享链接
                </td>
              </tr>
            ) : (
              shares.map((share) => {
                const expired = isExpired(share)
                const inactive = isInactive(share)
                return (
                  <tr key={share.id} className={inactive ? 'inactive' : ''}>
                    <td>
                      <code>{shortToken(share.token)}</code>
                    </td>
                    <td>
                      <code className="share-url">{shortSharePath(share.token)}</code>
                    </td>
                    <td>{formatBeijingTime(share.created_at)}</td>
                    <td>{share.expire_at ? formatBeijingTime(share.expire_at) : '永久'}</td>
                    <td>
                      {share.revoked ? (
                        <span className="status-badge revoked">已撤销</span>
                      ) : expired ? (
                        <span className="status-badge expired">已过期</span>
                      ) : (
                        <span className="status-badge active">有效</span>
                      )}
                    </td>
                    <td>
                      <div className="table-actions">
                        <button className="btn ghost" onClick={() => handleCopy(share)} disabled={inactive}>
                          复制
                        </button>
                        {!share.revoked && (
                          <button className="btn danger" onClick={() => handleRevoke(share.id)}>
                            撤销
                          </button>
                        )}
                      </div>
                    </td>
                  </tr>
                )
              })
            )}
          </tbody>
        </table>

        <div className="pagination">
          <button className="btn ghost" onClick={() => setPage((p) => Math.max(1, p - 1))} disabled={page === 1 || loading}>
            上一页
          </button>
          <span>
            第 {page} / 共 {totalPages} 页
          </span>
          <button
            className="btn ghost"
            onClick={() => setPage((p) => p + 1)}
            disabled={disableNext || loading}
          >
            下一页
          </button>
        </div>
      </Card>

      <Modal open={showCreateModal} title="创建分享链接" onClose={() => setShowCreateModal(false)}>
        <CreateShareForm onSubmit={handleCreate} submitting={creating} onCancel={() => setShowCreateModal(false)} />
      </Modal>

      {toast && <Toast message={toast.message} type={toast.type} />}
    </div>
  )
}

function CreateShareForm({
  onSubmit,
  submitting,
  onCancel,
}: {
  onSubmit: (expireIn: CreateMonitorShareRequest['expire_in']) => void
  submitting: boolean
  onCancel: () => void
}) {
  const [expireIn, setExpireIn] = useState<CreateMonitorShareRequest['expire_in']>('24h')

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    onSubmit(expireIn)
  }

  return (
    <form className="share-form" onSubmit={handleSubmit}>
      <label>
        有效期
        <select value={expireIn} onChange={(e) => setExpireIn(e.target.value as CreateMonitorShareRequest['expire_in'])}>
          <option value="1h">1 小时</option>
          <option value="24h">24 小时</option>
          <option value="168h">7 天</option>
          <option value="permanent">永久</option>
        </select>
      </label>

      <div className="form-actions">
        <button className="btn ghost" type="button" onClick={onCancel} disabled={submitting}>
          取消
        </button>
        <button className="btn primary" type="submit" disabled={submitting}>
          {submitting ? '创建中...' : '创建'}
        </button>
      </div>
    </form>
  )
}
