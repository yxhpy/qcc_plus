import type { ReactNode } from 'react'
import { NavLink, useNavigate } from 'react-router-dom'
import { useAuth } from '../hooks/useAuth'
import './Layout.css'

interface LayoutProps {
  children: ReactNode
}

export default function Layout({ children }: LayoutProps) {
  const { logout, user } = useAuth()
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

  return (
    <>
      <nav className="layout-nav">
        <div className="layout-brand">Claude Proxy</div>
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
        </div>
      </nav>
      <main className="layout-main">{children}</main>
    </>
  )
}
