import type { FormEvent } from 'react'
import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import Card from '../components/Card'
import Toast from '../components/Toast'
import { useAuth } from '../hooks/useAuth'

export default function Login() {
  const navigate = useNavigate()
  const { login } = useAuth()
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')
  const [toast, setToast] = useState<{ message: string; type: 'success' | 'error' } | null>(null)
  const [loading, setLoading] = useState(false)

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault()
    setError('')
    setToast(null)
    if (!username.trim() || !password.trim()) {
      setError('账号名称和密码不能为空')
      return
    }
    try {
      setLoading(true)
      await login(username.trim(), password.trim())
      setToast({ message: '登录成功，正在跳转...', type: 'success' })
      navigate('/admin/dashboard', { replace: true })
    } catch (err) {
      setError((err as Error).message || '登录失败')
      setToast({ message: (err as Error).message || '登录失败', type: 'error' })
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="login-page">
      <Card>
        <div className="login-title">
          <h1>登录 Claude Proxy</h1>
          <p className="sub">使用账号名称和密码登录管理后台</p>
        </div>
        {error && <div className="error-box">{error}</div>}
        <form className="login-form" onSubmit={handleSubmit}>
          <label>
            账号名称
            <input
              name="username"
              value={username}
              onChange={(e) => setUsername(e.target.value)}
              placeholder="输入账号名称"
              autoComplete="username"
              required
            />
          </label>
          <label>
            密码
            <input
              name="password"
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              placeholder="输入密码"
              autoComplete="current-password"
              required
            />
          </label>
          <button className="btn primary" type="submit" disabled={loading}>
            {loading ? '登录中...' : '登录'}
          </button>
        </form>
        <small className="muted">登录后 24 小时内保持会话，记得使用退出按钮主动登出。</small>
      </Card>
      <Toast message={toast?.message} type={toast?.type} />
    </div>
  )
}
