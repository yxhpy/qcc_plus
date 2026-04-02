import { useCallback, useEffect, useState } from 'react'
import Card from '../components/Card'
import Toast from '../components/Toast'
import api from '../services/api'
import type { ErrorPolicySnapshot } from '../types'
import './ErrorPolicies.css'

export default function ErrorPolicies() {
  const [data, setData] = useState<ErrorPolicySnapshot>({ builtin_rules: [], custom_rules: [], observed: [] })
  const [loading, setLoading] = useState(true)
  const [toast, setToast] = useState<{ message: string; type: 'success' | 'error' } | null>(null)

  const showToast = (message: string, type: 'success' | 'error' = 'success') => {
    setToast({ message, type })
    setTimeout(() => setToast(null), 2200)
  }

  const load = useCallback(async () => {
    setLoading(true)
    try {
      const res = await api.getErrorPolicies()
      setData(res.data)
    } catch (e) {
      showToast('加载异常策略失败', 'error')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    load()
  }, [load])

  const toggleBuiltin = async (id: string, enabled: boolean) => {
    try {
      await api.toggleErrorPolicy('builtin', id, enabled)
      showToast('内置规则已更新')
      await load()
    } catch {
      showToast('更新失败', 'error')
    }
  }

  const toggleObserved = async (id: string, enabled: boolean) => {
    try {
      await api.toggleErrorPolicy('observed', id, enabled)
      showToast('异常策略已更新')
      await load()
    } catch {
      showToast('更新失败', 'error')
    }
  }

  return (
    <div className="error-policies-page">
      <div className="error-policies-header">
        <h1>异常策略</h1>
        <p className="sub">自动采集线上异常，勾选后该异常会自动触发切换，不再直接透传给客户端。</p>
      </div>

      <Card>
        <div className="error-policies-toolbar">
          <button className="btn ghost" onClick={load}>刷新</button>
        </div>

        {loading ? (
          <div className="empty">加载中...</div>
        ) : (
          <>
            <h3>内置策略（可关闭）</h3>
            <table className="error-policy-table">
              <thead>
                <tr>
                  <th>启用</th>
                  <th>名称</th>
                  <th>状态码</th>
                  <th>错误码</th>
                  <th>关键词</th>
                </tr>
              </thead>
              <tbody>
                {data.builtin_rules.map((r) => (
                  <tr key={r.id}>
                    <td><input type="checkbox" checked={r.enabled} onChange={(e) => toggleBuiltin(r.id, e.target.checked)} /></td>
                    <td>{r.name}</td>
                    <td>{r.status_code || '-'}</td>
                    <td>{r.error_code || '-'}</td>
                    <td>{r.message_contains || '-'}</td>
                  </tr>
                ))}
              </tbody>
            </table>

            <h3 style={{ marginTop: 20 }}>观察到的异常（勾选后自动切换）</h3>
            <table className="error-policy-table">
              <thead>
                <tr>
                  <th>自动切换</th>
                  <th>状态码</th>
                  <th>错误码</th>
                  <th>错误信息</th>
                  <th>出现次数</th>
                  <th>最后出现</th>
                </tr>
              </thead>
              <tbody>
                {data.observed.length === 0 ? (
                  <tr>
                    <td colSpan={6} className="empty">暂无观察异常</td>
                  </tr>
                ) : data.observed.map((o) => (
                  <tr key={o.id}>
                    <td><input type="checkbox" checked={o.auto_switch} onChange={(e) => toggleObserved(o.id, e.target.checked)} /></td>
                    <td>{o.status_code}</td>
                    <td>{o.error_code || '-'}</td>
                    <td className="msg" title={o.message}>{o.message}</td>
                    <td>{o.count}</td>
                    <td>{new Date(o.last_seen_at).toLocaleString()}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </>
        )}
      </Card>

      <Toast message={toast?.message} type={toast?.type} />
    </div>
  )
}
