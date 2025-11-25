import type { ReactNode } from 'react'
import { Link, NavLink, useNavigate } from 'react-router-dom'
import { useAuth } from '../hooks/useAuth'
import { useVersion } from '../hooks/useVersion'
import { formatBeijingTime } from '../utils/date'
import './Layout.css'

interface LayoutProps {
  children: ReactNode
}

export default function Layout({ children }: LayoutProps) {
  const { logout, user } = useAuth()
  const { version, loading: versionLoading, error: versionError } = useVersion()
  const navigate = useNavigate()

  const handleLogout = async () => {
    try {
      await logout()
      navigate('/login', { replace: true })
    } catch (err) {
      navigate('/login', { replace: true })
    }
  }

  const linkClass = ({ isActive }: { isActive: boolean }) =>
    `layout-link ${isActive ? 'active' : ''}`

  const versionLabel = version
    ? (version.version.startsWith('v') ? version.version : `v${version.version}`)
    : versionLoading
      ? 'v...'
      : 'v-'
  const versionTitle = version
    ? `commit: ${version.git_commit}\nbuild: ${formatBeijingTime(version.build_date)}\ngo: ${version.go_version}`
    : versionError
      ? `版本获取失败：${versionError.message}`
      : '正在加载版本信息...'

  return (
    <>
      <nav className="layout-nav">
        <div className="layout-brand">QCC Plus</div>
        <div className="layout-links">
          <NavLink to="/admin/dashboard" className={linkClass}>
            仪表盘
          </NavLink>
          <NavLink to="/admin/accounts" className={linkClass}>
            账号管理
          </NavLink>
          <NavLink to="/admin/nodes" className={linkClass}>
            节点管理
          </NavLink>
          <NavLink to="/admin/settings" className={linkClass}>
            系统配置
          </NavLink>
          <NavLink to="/admin/notifications" className={linkClass}>
            通知管理
          </NavLink>
          {user?.is_admin && (
            <NavLink to="/admin/tunnel" className={linkClass}>
              隧道设置
            </NavLink>
          )}
        </div>
        <div className="layout-actions">
          {user && (
            <small className="user-info">
              {user.name}{user.is_admin ? ' · 管理员' : ''}
            </small>
          )}
          <button className="btn-logout" type="button" onClick={handleLogout}>
            退出登录
          </button>
          <div className="layout-version-wrap">
            <Link to="/changelog" className="layout-changelog">更新日志</Link>
            <div className="layout-version" title={versionTitle}>
              {versionLabel}
            </div>
          </div>
        </div>
      </nav>
      <main className="layout-main">{children}</main>
    </>
  )
}
