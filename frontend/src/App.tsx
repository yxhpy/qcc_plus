import type { ReactElement } from 'react'
import { Navigate, Route, Routes, BrowserRouter } from 'react-router-dom'
import Layout from './components/Layout'
import Login from './pages/Login'
import Dashboard from './pages/Dashboard'
import Accounts from './pages/Accounts'
import Nodes from './pages/Nodes'
import Settings from './pages/Settings'
import TunnelSettings from './pages/TunnelSettings'
import { useAuth } from './hooks/useAuth'

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
          path="/admin/tunnel"
          element={
            <ProtectedRoute adminOnly>
              <Layout>
                <TunnelSettings />
              </Layout>
            </ProtectedRoute>
          }
        />
        <Route path="*" element={<Navigate to="/" replace />} />
      </Routes>
    </BrowserRouter>
  )
}
