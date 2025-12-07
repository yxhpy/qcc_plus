import { Suspense, lazy, type ReactElement } from 'react'
import { Navigate, Route, Routes, BrowserRouter } from 'react-router-dom'
import Layout from './components/Layout'
import Loading from './components/Loading'
import { useAuth } from './hooks/useAuth'
import { NodeMetricsProvider } from './contexts/NodeMetricsContext'
import { SettingsProvider } from './contexts/SettingsContext'

// Route-level code splitting to keep initial bundle light
const Login = lazy(() => import('./pages/Login'))
const Dashboard = lazy(() => import('./pages/Dashboard'))
const Accounts = lazy(() => import('./pages/Accounts'))
const Nodes = lazy(() => import('./pages/Nodes'))
const Monitor = lazy(() => import('./pages/Monitor'))
const MonitorShares = lazy(() => import('./pages/MonitorShares'))
const Settings = lazy(() => import('./pages/Settings'))
const SystemSettings = lazy(() => import('./pages/SystemSettings'))
const TunnelSettings = lazy(() => import('./pages/TunnelSettings'))
const Notifications = lazy(() => import('./pages/Notifications'))
const ChangelogPage = lazy(() => import('./pages/ChangelogPage'))
const SharedMonitor = lazy(() => import('./pages/SharedMonitor'))
const ClaudeConfig = lazy(() => import('./pages/ClaudeConfig'))
const Pricing = lazy(() => import('./pages/Pricing'))
const Usage = lazy(() => import('./pages/Usage'))

function ProtectedRoute({ children, adminOnly = false }: { children: ReactElement; adminOnly?: boolean }) {
  const { isAuthenticated, loading, isAdmin } = useAuth()

  if (loading) {
    return <Loading message="加载中..." />
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
    <SettingsProvider>
      <NodeMetricsProvider>
        <BrowserRouter>
          <Suspense fallback={<Loading />}>
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
                path="/admin/claude-config"
                element={
                  <ProtectedRoute>
                    <Layout>
                      <ClaudeConfig />
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
                path="/settings"
                element={
                  <ProtectedRoute adminOnly>
                    <Layout>
                      <SystemSettings />
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
              <Route
                path="/admin/pricing"
                element={
                  <ProtectedRoute adminOnly>
                    <Layout>
                      <Pricing />
                    </Layout>
                  </ProtectedRoute>
                }
              />
              <Route
                path="/admin/usage"
                element={
                  <ProtectedRoute>
                    <Layout>
                      <Usage />
                    </Layout>
                  </ProtectedRoute>
                }
              />
              <Route path="/monitor/share/:token" element={<SharedMonitor />} />
              <Route path="*" element={<Navigate to="/" replace />} />
            </Routes>
          </Suspense>
        </BrowserRouter>
      </NodeMetricsProvider>
    </SettingsProvider>
  )
}
