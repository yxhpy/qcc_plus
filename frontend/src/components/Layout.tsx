import type { ReactNode } from 'react'
import { useState, useEffect, useCallback } from 'react'
import { Link, NavLink, useNavigate } from 'react-router-dom'
import { useAuth } from '../hooks/useAuth'
import { useVersion } from '../hooks/useVersion'
import { useTheme } from '../themes'
import './Layout.css'

// 简洁的 20x20 图标（使用 currentColor）
const icons = {
  dashboard: (
    <svg viewBox="0 0 20 20" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
      <rect x="3" y="3" width="6" height="6" rx="1.2" />
      <rect x="11" y="3" width="6" height="4.5" rx="1.2" />
      <rect x="3" y="11" width="6" height="6" rx="1.2" />
      <rect x="11" y="9.5" width="6" height="7.5" rx="1.2" />
    </svg>
  ),
  monitor: (
    <svg viewBox="0 0 20 20" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
      <rect x="2.75" y="3.5" width="14.5" height="9.5" rx="1.3" />
      <path d="M5.5 11.5 7.3 8l1.9 4 1.6-4.7 1.8 4.6 1.4-3" />
      <path d="M8 16.5h4" />
      <path d="M10 13v3.5" />
    </svg>
  ),
  magic: (
    <svg viewBox="0 0 20 20" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
      <path d="m11 2.5-4.8 7.7h3.2L7.8 17.5l6.2-9.7H11l.8-5.3Z" />
      <path d="m4 15.6 2-2" />
      <path d="m14.5 5.5 1.5-1.5" />
      <path d="m5.2 5.8.6-2.3" />
      <path d="m15 14.2 1.8.3" />
    </svg>
  ),
  accounts: (
    <svg viewBox="0 0 20 20" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
      <path d="M10 10.25a3.25 3.25 0 1 0-3.25-3.25A3.25 3.25 0 0 0 10 10.25Z" />
      <path d="M4.5 15.75c.8-2.05 2.85-3.25 5.5-3.25s4.7 1.2 5.5 3.25" />
    </svg>
  ),
  nodes: (
    <svg viewBox="0 0 20 20" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
      <circle cx="5" cy="5" r="2.25" />
      <circle cx="15" cy="6.5" r="2.25" />
      <circle cx="10" cy="15" r="2.25" />
      <path d="M6.9 6.1 13.2 6.4" />
      <path d="M6.2 6.9 8.8 13.4" />
      <path d="M13.3 8.4 10.9 13" />
    </svg>
  ),
  share: (
    <svg viewBox="0 0 20 20" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
      <path d="m15.5 5-5.2 3-5.3-3" />
      <circle cx="15.5" cy="5" r="2" />
      <circle cx="4.5" cy="10" r="2" />
      <circle cx="15.5" cy="15" r="2" />
      <path d="m6.4 11.1 3.9 2.3" />
      <path d="m11 7.9 3.6 1.9" />
    </svg>
  ),
  settings: (
    <svg viewBox="0 0 20 20" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
      <circle cx="10" cy="10" r="3" />
      <path d="M10 3.5v1.2" />
      <path d="M10 15.3v1.2" />
      <path d="m4.3 5.2.9.7" />
      <path d="m14.8 14.1.9.7" />
      <path d="M3.5 10h1.2" />
      <path d="M15.3 10h1.2" />
      <path d="m4.3 14.8.9-.7" />
      <path d="m14.8 5.9.9-.7" />
    </svg>
  ),
  notifications: (
    <svg viewBox="0 0 20 20" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
      <path d="M6.5 8.5a3.5 3.5 0 0 1 7 0c0 3.25 1.15 4.5 2 5H4.5c.85-.5 2-1.75 2-5Z" />
      <path d="M9 15.5c.25.6.85 1 1.5 1s1.25-.4 1.5-1" />
    </svg>
  ),
  tunnel: (
    <svg viewBox="0 0 20 20" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
      <path d="M6.2 11.5A3.5 3.5 0 0 1 6.5 4a3.7 3.7 0 0 1 1.9.52 4 4 0 0 1 6 3.48v.5a3 3 0 0 1-1.2 5.8H11" />
      <path d="M11.2 9.5 8.8 12l2.4 2.5" />
    </svg>
  ),
  changelog: (
    <svg viewBox="0 0 20 20" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
      <path d="M6.5 3.5h5.8a1.7 1.7 0 0 1 1.2.5l2 2a1.7 1.7 0 0 1 .5 1.2v7.1a1.7 1.7 0 0 1-1.7 1.7H6.5A1.7 1.7 0 0 1 4.8 14V5.2A1.7 1.7 0 0 1 6.5 3.5Z" />
      <path d="M13 3.6V6a1 1 0 0 0 1 1h2.3" />
      <path d="M7.5 9.5h5" />
      <path d="M7.5 12.5h3.5" />
    </svg>
  ),
  pricing: (
    <svg viewBox="0 0 20 20" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
      <path d="M10 3v14" />
      <path d="M13.5 6H8.25a2.25 2.25 0 0 0 0 4.5h3.5a2.25 2.25 0 0 1 0 4.5H6" />
      <path d="M8 3h4" />
      <path d="M8 17h4" />
    </svg>
  ),
  usage: (
    <svg viewBox="0 0 20 20" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
      <path d="M3 17V7l4-4 6 6 4-4v12" />
      <path d="M3 13l4-4 6 6 4-4" />
    </svg>
  ),
  collapse: (
    <svg viewBox="0 0 20 20" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
      <path d="m11.5 5.5-4 4 4 4" />
    </svg>
  ),
  expand: (
    <svg viewBox="0 0 20 20" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
      <path d="m8.5 14.5 4-4-4-4" />
    </svg>
  ),
  logout: (
    <svg viewBox="0 0 20 20" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
      <path d="M11.5 5.5V4.2A1.7 1.7 0 0 0 9.8 2.5H5.7A1.7 1.7 0 0 0 4 4.2v11.6a1.7 1.7 0 0 0 1.7 1.7h4.1a1.7 1.7 0 0 0 1.7-1.7v-1.3" />
      <path d="M8.5 10h7" />
      <path d="m13.5 7 3 3-3 3" />
    </svg>
  ),
  sun: (
    <svg viewBox="0 0 20 20" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
      <circle cx="10" cy="10" r="3.25" />
      <path d="M10 3.5v1.5M10 15v1.5M4.5 10H3M17 10h-1.5M5.5 5.5 4.4 4.4M15.6 15.6 14.5 14.5M5.5 14.5 4.4 15.6M15.6 4.4 14.5 5.5" />
    </svg>
  ),
  moon: (
    <svg viewBox="0 0 20 20" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
      <path d="M15.5 11.6a5.8 5.8 0 1 1-7.1-7.1 5.2 5.2 0 0 0 7.1 7.1Z" />
    </svg>
  ),
  system: (
    <svg viewBox="0 0 20 20" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
      <circle cx="10" cy="10" r="5.5" />
      <path d="M4.5 10h11" />
      <path d="M10 4.5c1.6 2.6 1.6 8.4 0 11" />
    </svg>
  ),
} as const

interface NavItem {
  path: string
  label: string
  icon: keyof typeof icons
  adminOnly?: boolean
  group: 'core' | 'system'
}

const navItems: NavItem[] = [
	{ path: '/admin/dashboard', label: '仪表盘', icon: 'dashboard', group: 'core' },
	{ path: '/admin/claude-config', label: '快速配置', icon: 'magic', group: 'core' },
	{ path: '/admin/monitor', label: '监控大屏', icon: 'monitor', group: 'core' },
	{ path: '/admin/nodes', label: '节点管理', icon: 'nodes', group: 'core' },
	{ path: '/admin/usage', label: '使用统计', icon: 'usage', group: 'core' },
	{ path: '/admin/accounts', label: '账号管理', icon: 'accounts', adminOnly: true, group: 'system' },
	{ path: '/admin/pricing', label: '模型定价', icon: 'pricing', adminOnly: true, group: 'system' },
	{ path: '/admin/monitor-shares', label: '分享链接', icon: 'share', group: 'system' },
	{ path: '/admin/notifications', label: '通知管理', icon: 'notifications', group: 'system' },
	{ path: '/settings', label: '系统设置', icon: 'settings', adminOnly: true, group: 'system' },
	{ path: '/admin/tunnel', label: '隧道设置', icon: 'tunnel', adminOnly: true, group: 'system' },
]

const SIDEBAR_STORAGE_KEY = 'qcc-sidebar-collapsed'

export default function Layout({ children }: { children: ReactNode }) {
  const { logout, user } = useAuth()
  const { version } = useVersion()
  const { theme, setTheme, resolvedTheme } = useTheme()
  const navigate = useNavigate()

  const [collapsed, setCollapsed] = useState(() => {
    if (typeof window === 'undefined') return false
    const stored = localStorage.getItem(SIDEBAR_STORAGE_KEY)
    return stored === 'true'
  })

  // 移动端自动收起
  useEffect(() => {
    const mediaQuery = window.matchMedia('(max-width: 768px)')
    const handler = (e: MediaQueryListEvent) => {
      if (e.matches) setCollapsed(true)
    }

    if (mediaQuery.matches) setCollapsed(true)

    if (mediaQuery.addEventListener) {
      mediaQuery.addEventListener('change', handler)
    } else {
      mediaQuery.addListener(handler)
    }

    return () => {
      if (mediaQuery.removeEventListener) {
        mediaQuery.removeEventListener('change', handler)
      } else {
        mediaQuery.removeListener(handler)
      }
    }
  }, [])

  const toggleCollapsed = useCallback(() => {
    setCollapsed(prev => {
      const next = !prev
      localStorage.setItem(SIDEBAR_STORAGE_KEY, String(next))
      return next
    })
  }, [])

  const handleLogout = async () => {
    try {
      await logout()
      navigate('/login', { replace: true })
    } catch {
      navigate('/login', { replace: true })
    }
  }

  const cycleTheme = () => {
    const modes = ['light', 'dark', 'system'] as const
    const idx = modes.indexOf(theme)
    setTheme(modes[(idx + 1) % modes.length])
  }

  const coreItems = navItems.filter(item => item.group === 'core')
  const systemItems = navItems.filter(item => item.group === 'system')

  const renderNavItem = (item: NavItem) => {
    if (item.adminOnly && !user?.is_admin) return null
    return (
      <NavLink
        key={item.path}
        to={item.path}
        className={({ isActive }) => `sidebar-link ${isActive ? 'active' : ''}`}
        title={collapsed ? item.label : undefined}
      >
        <span className="sidebar-icon">{icons[item.icon]}</span>
        {!collapsed && <span className="sidebar-label">{item.label}</span>}
      </NavLink>
    )
  }

  const themeIcon = theme === 'system' ? icons.system : resolvedTheme === 'dark' ? icons.moon : icons.sun
  const themeLabel = theme === 'system' ? '跟随系统' : resolvedTheme === 'dark' ? '深色' : '浅色'

  return (
    <div className={`layout ${collapsed ? 'sidebar-collapsed' : ''}`}>
      <aside className="sidebar">
        <div className="sidebar-header">
          <div className="sidebar-brand" title="QCC Plus">
            {collapsed ? 'Q' : 'QCC Plus'}
          </div>
          <button className="sidebar-toggle" onClick={toggleCollapsed} title={collapsed ? '展开' : '收起'}>
            {collapsed ? icons.expand : icons.collapse}
          </button>
        </div>

        <nav className="sidebar-nav">
          <div className="sidebar-group">
            {!collapsed && <div className="sidebar-group-title">核心功能</div>}
            {coreItems.map(renderNavItem)}
          </div>
          <div className="sidebar-group">
            {!collapsed && <div className="sidebar-group-title">系统管理</div>}
            {systemItems.map(renderNavItem)}
          </div>
        </nav>

        <div className="sidebar-footer">
          <button className="sidebar-theme-btn" onClick={cycleTheme} title={`当前: ${theme}`}>
            <span className="sidebar-icon">{themeIcon}</span>
            {!collapsed && <span className="sidebar-label">{themeLabel}</span>}
          </button>

          <Link to="/changelog" className="sidebar-link" title={collapsed ? '更新日志' : undefined}>
            <span className="sidebar-icon">{icons.changelog}</span>
            {!collapsed && <span className="sidebar-label">更新日志</span>}
          </Link>

          {user && !collapsed && (
            <div className="sidebar-user">
              <span className="sidebar-user-name">{user.name}</span>
              {user.is_admin && <span className="sidebar-user-badge">管理员</span>}
            </div>
          )}

          <button
            className="sidebar-link sidebar-logout"
            onClick={handleLogout}
            title={collapsed ? '退出登录' : undefined}
          >
            <span className="sidebar-icon">{icons.logout}</span>
            {!collapsed && <span className="sidebar-label">退出登录</span>}
          </button>

          {!collapsed && version && (
            <div className="sidebar-version">
              {version.version.startsWith('v') ? version.version : `v${version.version}`}
            </div>
          )}
        </div>
      </aside>

      <main className="layout-main">{children}</main>
    </div>
  )
}
