import type { FormEvent } from 'react'
import { useEffect, useState } from 'react'
import Card from '../components/Card'
import Toast from '../components/Toast'
import api from '../services/api'
import type { Account } from '../types'
import './Settings.css'

export default function Settings() {
  const [accounts, setAccounts] = useState<Account[]>([])
  const [accountId, setAccountId] = useState('')
  const [retries, setRetries] = useState(1)
  const [failLimit, setFailLimit] = useState(1)
  const [health, setHealth] = useState(30)
  const [loading, setLoading] = useState(false)
  const [toast, setToast] = useState<{ message: string; type: 'success' | 'error' } | null>(null)

  const showToast = (message: string, type: 'success' | 'error' = 'success') => {
    setToast({ message, type })
    setTimeout(() => setToast(null), 2200)
  }

  const loadAccounts = async () => {
    try {
      const list = await api.getAccounts()
      setAccounts(list)
      setAccountId((prev) => prev || (list[0]?.id ?? ''))
    } catch (err) {
      showToast('加载账号失败', 'error')
    }
  }

  const loadConfig = async () => {
    if (!accountId) return
    setLoading(true)
    try {
      const cfg = await api.getConfig(accountId)
      setRetries(cfg.retries || 1)
      setFailLimit(cfg.fail_limit || 1)
      setHealth(cfg.health_interval_sec || 30)
    } catch (err) {
      showToast((err as Error).message || '加载失败', 'error')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    loadAccounts()
  }, [])

  useEffect(() => {
    if (accountId) {
      loadConfig()
    }
  }, [accountId])

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault()
    try {
      await api.updateConfig({
        retries: Math.min(10, Math.max(1, retries)),
        fail_limit: Math.min(10, Math.max(1, failLimit)),
        health_interval_sec: Math.min(300, Math.max(5, health)),
      }, accountId)
      showToast('配置已保存')
      loadConfig()
    } catch (err) {
      showToast((err as Error).message || '保存失败', 'error')
    }
  }

  return (
    <div className="settings-page">
      <div className="settings-header">
        <h1>系统配置</h1>
        <p className="sub">调整重试、失败阈值和健康检查频率。</p>
      </div>

      <Card>
        <div className="toolbar">
          <label style={{ minWidth: 220 }}>
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
          <div className="spacer" />
          <button className="btn ghost" type="button" onClick={loadConfig}>
            刷新
          </button>
        </div>
        <form className="settings-form" onSubmit={handleSubmit}>
          <label>
            重试次数
            <input
              type="number"
              name="retries"
              min={1}
              max={10}
              value={retries}
              onChange={(e) => setRetries(Number(e.target.value))}
              required
            />
          </label>
          <label>
            失败阈值
            <input
              type="number"
              name="fail"
              min={1}
              max={10}
              value={failLimit}
              onChange={(e) => setFailLimit(Number(e.target.value))}
              required
            />
          </label>
          <label>
            健康检查间隔（秒）
            <input
              type="number"
              name="health"
              min={5}
              max={300}
              value={health}
              onChange={(e) => setHealth(Number(e.target.value))}
              required
            />
          </label>
          <button className="btn primary" type="submit" disabled={loading}>
            保存配置
          </button>
        </form>
        <div className="notice" style={{ marginTop: 24 }}>
          范围提示：重试 1-10，失败阈值 1-10，健康检查 5-300 秒。
        </div>
      </Card>
      <Toast message={toast?.message} type={toast?.type} />
    </div>
  )
}
