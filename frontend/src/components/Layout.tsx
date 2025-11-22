import type { ReactNode } from 'react'
import { NavLink, useNavigate } from 'react-router-dom'
import { useAuth } from '../hooks/useAuth'

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

  const linkClass = ({ isActive }: { isActive: boolean }) => (isActive ? 'active' : '')

  return (
    <>
      <nav className="top">
        <div className="brand">Claude Proxy</div>
        <div className="links">
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
        <div className="actions">
          {user && <small className="muted">{user.name}{user.is_admin ? ' · 管理员' : ''}</small>}
          <button className="btn ghost" type="button" onClick={handleLogout}>
            退出登录
          </button>
        </div>
      </nav>
      <main className="page">{children}</main>
    </>
  )
}
