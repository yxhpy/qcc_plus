import type { FormEvent } from 'react'
import { useState } from 'react'
import { useNavigate } from 'react-router-dom'

import Toast from '../components/Toast'
import { useAuth } from '../hooks/useAuth'

import './Login.css'

import loginIcon from '../assets/login-icon.png'

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
      <div className="login-container">
        <div className="login-header">
          <img src={loginIcon} alt="Logo" className="login-icon" />
          <div className="login-title">
            <h1>Welcome back</h1>
            <p className="sub">登录 Claude Proxy 管理后台</p>
          </div>
        </div>

        {error && <div className="error-message">{error}</div>}

        <form className="login-form" onSubmit={handleSubmit}>
          <div className="form-group">
            <label className="form-label">账号名称</label>
            <input
              className="form-input"
              name="username"
              value={username}
              onChange={(e) => setUsername(e.target.value)}
              placeholder="输入账号名称"
              autoComplete="username"
              required
            />
          </div>

          <div className="form-group">
            <label className="form-label">密码</label>
            <input
              className="form-input"
              name="password"
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              placeholder="输入密码"
              autoComplete="current-password"
              required
            />
          </div>

          <button className="btn-submit" type="submit" disabled={loading}>
            {loading ? '登录中...' : '继续'}
          </button>
        </form>

        <div className="login-footer">
          登录后 24 小时内保持会话，记得使用退出按钮主动登出。
        </div>
      </div>
      <Toast message={toast?.message} type={toast?.type} />
    </div>
  )
}
