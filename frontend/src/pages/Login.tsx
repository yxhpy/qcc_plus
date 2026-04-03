import type { FormEvent } from 'react'
import { useState } from 'react'
import { useNavigate } from 'react-router-dom'

import Toast from '../components/Toast'
import { useAuth } from '../hooks/useAuth'
import { useVersion } from '../hooks/useVersion'

import './Login.css'

import loginIcon from '../assets/qcc-plus-logo.png'

function UserInputIcon() {
  return (
    <svg viewBox="0 0 24 24" aria-hidden="true">
      <path
        d="M12 12.5C9.65279 12.5 7.75 10.5972 7.75 8.25C7.75 5.90279 9.65279 4 12 4C14.3472 4 16.25 5.90279 16.25 8.25C16.25 10.5972 14.3472 12.5 12 12.5Z"
        fill="currentColor"
      />
      <path
        d="M12 14C8.27208 14 5.16779 16.6684 4.49247 20.1991C4.41508 20.6038 4.74896 21 5.16098 21H18.839C19.251 21 19.5849 20.6038 19.5075 20.1991C18.8322 16.6684 15.7279 14 12 14Z"
        fill="currentColor"
      />
    </svg>
  )
}

function PasswordInputIcon() {
  return (
    <svg viewBox="0 0 24 24" aria-hidden="true">
      <path
        d="M17 10H7C5.89543 10 5 10.8954 5 12V18C5 19.1046 5.89543 20 7 20H17C18.1046 20 19 19.1046 19 18V12C19 10.8954 18.1046 10 17 10Z"
        fill="currentColor"
      />
      <path
        d="M8 10V8.5C8 6.01472 10.0147 4 12.5 4C14.9853 4 17 6.01472 17 8.5V10H15.5V8.5C15.5 6.84315 14.1569 5.5 12.5 5.5C10.8431 5.5 9.5 6.84315 9.5 8.5V10H8Z"
        fill="currentColor"
      />
    </svg>
  )
}

export default function Login() {
  const navigate = useNavigate()
  const { login } = useAuth()
  const { version, loading: versionLoading, error: versionError } = useVersion()
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

  const versionLabel = version
    ? (version.version.startsWith('v') ? version.version : `v${version.version}`)
    : versionLoading
      ? 'v...'
      : 'v-'
  const buildDateBeijing = version?.build_date_beijing || '--'
  const environmentLabel = version?.environment?.trim().toLowerCase() || 'dev'
  const versionTitle = version
    ? `commit: ${version.git_commit}\n环境: ${environmentLabel}\nbuild (BJ): ${buildDateBeijing}\nbuild (UTC): ${version.build_date || '--'}\ngo: ${version.go_version}`
    : versionError
      ? `版本获取失败：${versionError.message}`
      : '正在加载版本信息...'

  return (
    <div className="login-page">
      <div className="login-shell">
        <aside className="login-brand">
          <div className="login-brand-orb login-brand-orb-primary" />
          <div className="login-brand-orb login-brand-orb-secondary" />
          <div className="login-brand-content">
            <div className="login-brand-badge">QCC Plus</div>
            <img src={loginIcon} alt="QCC Plus Logo" className="login-icon" />
            <div className="login-brand-copy">
              <p className="login-brand-eyebrow">AI Coding Gateway</p>
              <h2>统一管理你的 AI Coding 代理</h2>
              <p className="login-brand-slogan">
                支持 Claude Code、Codex、Gemini 等 AI 工具的统一代理网关，多租户隔离、用量可观测。
              </p>
            </div>
            <div className="login-brand-highlights">
              <div className="login-brand-highlight">
                <strong>多租户</strong>
                <span>账号与会话分区管理</span>
              </div>
              <div className="login-brand-highlight">
                <strong>多引擎</strong>
                <span>Claude Code / Codex / Gemini 统一接入</span>
              </div>
            </div>
          </div>
        </aside>

        <section className="login-panel">
          <div className="login-container">
            <div className="login-header">
              <div className="login-title">
                <p className="login-panel-kicker">管理后台登录</p>
                <h1>欢迎主人回家</h1>
                <p className="sub">使用账号名称和密码进入 QCC Plus 管理后台</p>
              </div>
            </div>

            {error && <div className="error-message">{error}</div>}

            <form className="login-form" onSubmit={handleSubmit}>
              <div className="form-group">
                <label className="form-label">账号名称</label>
                <div className="form-input-wrap">
                  <span className="form-input-icon">
                    <UserInputIcon />
                  </span>
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
              </div>

              <div className="form-group">
                <label className="form-label">密码</label>
                <div className="form-input-wrap">
                  <span className="form-input-icon">
                    <PasswordInputIcon />
                  </span>
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
              </div>

              <button className="btn-submit" type="submit" disabled={loading}>
                {loading ? '登录中...' : '继续'}
              </button>
            </form>

            <div className="login-footer">
              <span>登录后 24 小时内保持会话，记得使用退出按钮主动登出。</span>
              <div className="login-version" title={versionTitle}>
                {versionLabel}
                {version && <span className={`login-env-badge login-env-${environmentLabel}`}>{environmentLabel}</span>}
              </div>
            </div>
          </div>
        </section>
      </div>
      <div className="login-mobile-version" title={versionTitle}>
        {versionLabel}
        {version && <span className={`login-env-badge login-env-${environmentLabel}`}>{environmentLabel}</span>}
      </div>
      <Toast message={toast?.message} type={toast?.type} />
    </div>
  )
}
