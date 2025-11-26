import type { ReactElement } from 'react'
import { Navigate, Route, Routes, BrowserRouter } from 'react-router-dom'
import Layout from './components/Layout'
import Login from './pages/Login'
import Dashboard from './pages/Dashboard'
import Accounts from './pages/Accounts'
import Nodes from './pages/Nodes'
import Monitor from './pages/Monitor'
import MonitorShares from './pages/MonitorShares'
import Settings from './pages/Settings'
import TunnelSettings from './pages/TunnelSettings'
import Notifications from './pages/Notifications'
import ChangelogPage from './pages/ChangelogPage'
import SharedMonitor from './pages/SharedMonitor'
import { useAuth } from './hooks/useAuth'
import { NodeMetricsProvider } from './contexts/NodeMetricsContext'

function ProtectedRoute({ children, adminOnly = false }: { children: ReactElement; adminOnly?: boolean }) {
  const { isAuthenticated, loading, isAdmin } = useAuth()

  if (loading) {
    return <div className="page-loading">加载中...</div>
  }
  if (!isAuthenticated) {
    return <Navigate to="/login" replace />
  }
  if (adminOnly && !isAdmin) {
    return <Navigate to="/admin/dashboard" replace />
  }
  return children
}

function HomeRedirect() {
  const { isAuthenticated, loading } = useAuth()
  if (loading) return <div className="page-loading">加载中...</div>
  return <Navigate to={isAuthenticated ? '/admin/dashboard' : '/login'} replace />
}

export default function App() {
  return (
    <NodeMetricsProvider>
      <BrowserRouter>
        <Routes>
          <Route path="/" element={<HomeRedirect />} />
          <Route path="/login" element={<Login />} />
          <Route
            path="/admin/dashboard"
            element={
              <ProtectedRoute>
                <Layout>
                  <Dashboard />
                </Layout>
              </ProtectedRoute>
            }
          />
          <Route
            path="/admin/accounts"
            element={
              <ProtectedRoute adminOnly>
                <Layout>
                  <Accounts />
                </Layout>
              </ProtectedRoute>
            }
          />
          <Route
            path="/admin/nodes"
            element={
              <ProtectedRoute>
                <Layout>
                  <Nodes />
                </Layout>
              </ProtectedRoute>
            }
          />
          <Route
            path="/admin/monitor"
            element={
              <ProtectedRoute>
                <Layout>
                  <Monitor />
                </Layout>
              </ProtectedRoute>
            }
          />
          <Route
            path="/admin/monitor-shares"
            element={
              <ProtectedRoute>
                <Layout>
                  <MonitorShares />
                </Layout>
              </ProtectedRoute>
            }
          />
          <Route
            path="/admin/settings"
            element={
              <ProtectedRoute>
                <Layout>
                  <Settings />
                </Layout>
              </ProtectedRoute>
            }
          />
          <Route
            path="/admin/notifications"
            element={
              <ProtectedRoute>
                <Layout>
                  <Notifications />
                </Layout>
              </ProtectedRoute>
            }
          />
          <Route
            path="/changelog"
            element={
              <ProtectedRoute>
                <Layout>
                  <ChangelogPage />
                </Layout>
              </ProtectedRoute>
            }
          />
          <Route
            path="/admin/tunnel"
            element={
              <ProtectedRoute adminOnly>
                <Layout>
                  <TunnelSettings />
                </Layout>
              </ProtectedRoute>
            }
          />
          <Route path="/monitor/share/:token" element={<SharedMonitor />} />
          <Route path="*" element={<Navigate to="/" replace />} />
        </Routes>
      </BrowserRouter>
    </NodeMetricsProvider>
  )
}
