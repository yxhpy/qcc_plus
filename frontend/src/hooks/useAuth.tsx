import type { ReactNode } from 'react'
import { createContext, useCallback, useContext, useEffect, useMemo, useState } from 'react'
import api from '../services/api'
import type { Account } from '../types'

interface AuthContextValue {
  user: Account | null
  loading: boolean
  isAuthenticated: boolean
  isAdmin: boolean
  login: (username: string, password: string) => Promise<void>
  logout: () => Promise<void>
  refresh: () => Promise<void>
}

const AuthContext = createContext<AuthContextValue | undefined>(undefined)
const AUTH_KEY = 'claude-proxy-auth'

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<Account | null>(null)
  const [loading, setLoading] = useState(true)

  const refresh = useCallback(async () => {
    setLoading(true)
    try {
      const accounts = await api.getAccounts()
      const self = accounts[0] || null
      setUser(self)
      if (self) {
        localStorage.setItem(AUTH_KEY, 'true')
      } else {
        localStorage.removeItem(AUTH_KEY)
      }
    } catch (err) {
      setUser(null)
      localStorage.removeItem(AUTH_KEY)
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    // Attempt bootstrap only when previous auth exists or first load
    refresh()
  }, [refresh])

  const login = useCallback(
    async (username: string, password: string) => {
      await api.login(username, password)
      await refresh()
    },
    [refresh],
  )

  const logout = useCallback(async () => {
    await api.logout()
    localStorage.removeItem(AUTH_KEY)
    setUser(null)
  }, [])

  const value = useMemo<AuthContextValue>(
    () => ({
      user,
      loading,
      isAuthenticated: !!user,
      isAdmin: !!user?.is_admin,
      login,
      logout,
      refresh,
    }),
    [user, loading, login, logout, refresh],
  )

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>
}

export function useAuth() {
  const ctx = useContext(AuthContext)
  if (!ctx) {
    throw new Error('useAuth must be used within AuthProvider')
  }
  return ctx
}
