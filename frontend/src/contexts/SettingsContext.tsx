import { createContext, useContext, useState, useEffect, useCallback } from 'react'
import type { ReactNode } from 'react'
import { settingsApi, type Setting } from '../services/settingsApi'
import { useAuth } from '../hooks/useAuth'

interface SettingsContextType {
  settings: Record<string, Setting>
  loading: boolean
  error: string | null
  refresh: () => Promise<void>
  updateSetting: (key: string, value: any) => Promise<boolean>
  getSetting: <T>(key: string, defaultValue: T) => T
}

const SettingsContext = createContext<SettingsContextType | undefined>(undefined)

export function SettingsProvider({ children }: { children: ReactNode }) {
  const { isAuthenticated, loading: authLoading } = useAuth()
  const [settings, setSettings] = useState<Record<string, Setting>>({})
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [version, setVersion] = useState(0)

  const refresh = useCallback(async () => {
    if (!isAuthenticated) {
      setSettings({})
      setVersion(0)
      setError(null)
      setLoading(false)
      return
    }

    setLoading(true)
    try {
      const res = await settingsApi.list({ scope: 'system' })
      const map: Record<string, Setting> = {}
      res.data.forEach(s => {
        map[s.key] = s
      })
      setSettings(map)
      setVersion(res.version)
      setError(null)
    } catch (e: any) {
      setError(e?.message || '加载配置失败')
    } finally {
      setLoading(false)
    }
  }, [isAuthenticated])

  useEffect(() => {
    // 登录页不预拉系统设置，避免把 unauthenticated 缓存在全局上下文中。
    if (authLoading) return
    refresh()
  }, [authLoading, refresh])

  useEffect(() => {
    if (!isAuthenticated) return

    const timer = setInterval(async () => {
      try {
        const newVersion = await settingsApi.getVersion()
        if (newVersion > version) {
          refresh()
        }
      } catch {
        // ignore poll errors
      }
    }, 30000)
    return () => clearInterval(timer)
  }, [isAuthenticated, version, refresh])

  const updateSetting = async (key: string, value: any): Promise<boolean> => {
    const setting = settings[key]
    if (!setting) return false

    try {
      const res = await settingsApi.update(key, value, setting.version)
      if ((res as any).success) {
        setSettings(prev => ({
          ...prev,
          [key]: { ...prev[key], value, version: res.new_version },
        }))
        setVersion(prev => (res.new_version > prev ? res.new_version : prev))
        return true
      }
    } catch (e: any) {
      if (e?.message === 'version_conflict') {
        refresh()
      }
      throw e
    }
    return false
  }

  const getSetting = <T,>(key: string, defaultValue: T): T => {
    const s = settings[key]
    return s ? (s.value as T) : defaultValue
  }

  return (
    <SettingsContext.Provider value={{ settings, loading, error, refresh, updateSetting, getSetting }}>
      {children}
    </SettingsContext.Provider>
  )
}

export function useSettings() {
  const ctx = useContext(SettingsContext)
  if (!ctx) throw new Error('useSettings must be used within SettingsProvider')
  return ctx
}
