import { createContext, useContext, useState, type ReactNode } from 'react'

const STORAGE_KEY = 'qcc_node_metrics_preference'

interface NodeMetricsPreference {
  showProxy: boolean
  showHealth: boolean
}

const DEFAULT_PREF: NodeMetricsPreference = {
  showProxy: true,
  showHealth: true,
}

interface NodeMetricsContextType {
  preference: NodeMetricsPreference
  setPreference: (pref: Partial<NodeMetricsPreference>) => void
  resetToDefault: () => void
}

const NodeMetricsContext = createContext<NodeMetricsContextType | undefined>(undefined)

export function NodeMetricsProvider({ children }: { children: ReactNode }) {
  const [preference, setPreferenceState] = useState<NodeMetricsPreference>(() => {
    try {
      const saved = localStorage.getItem(STORAGE_KEY)
      return saved ? { ...DEFAULT_PREF, ...JSON.parse(saved) } : DEFAULT_PREF
    } catch {
      return DEFAULT_PREF
    }
  })

  const setPreference = (pref: Partial<NodeMetricsPreference>) => {
    setPreferenceState((prev) => {
      const next = { ...prev, ...pref }
      localStorage.setItem(STORAGE_KEY, JSON.stringify(next))
      return next
    })
  }

  const resetToDefault = () => {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(DEFAULT_PREF))
    setPreferenceState(DEFAULT_PREF)
  }

  return (
    <NodeMetricsContext.Provider value={{ preference, setPreference, resetToDefault }}>
      {children}
    </NodeMetricsContext.Provider>
  )
}

export function useNodeMetrics(): NodeMetricsContextType {
  const ctx = useContext(NodeMetricsContext)
  if (!ctx) {
    // 返回默认值，用于 SharedMonitor 等不包裹 Provider 的场景
    return {
      preference: DEFAULT_PREF,
      setPreference: () => {},
      resetToDefault: () => {},
    }
  }
  return ctx
}
