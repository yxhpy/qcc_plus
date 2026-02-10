import { createContext, useCallback, useContext, useEffect, useRef, useState, type ReactNode } from 'react'
import api from '../services/api'
import type { ModelRecoveryItem } from '../types'

const REFRESH_INTERVAL = 30000 // 30s

interface NodeRecoveryInfo {
  count: number
  models: string[] // model_id list
}

interface ModelRecoveryContextType {
  /** per-node recovery map: nodeId -> { count, models } */
  byNode: Record<string, NodeRecoveryInfo>
  /** total recovering models across all nodes */
  total: number
  /** force refresh */
  refresh: () => void
}

const ModelRecoveryContext = createContext<ModelRecoveryContextType | undefined>(undefined)

export function ModelRecoveryProvider({ children }: { children: ReactNode }) {
  const [byNode, setByNode] = useState<Record<string, NodeRecoveryInfo>>({})
  const [total, setTotal] = useState(0)
  const timerRef = useRef<ReturnType<typeof setInterval> | null>(null)

  const fetchData = useCallback(async () => {
    try {
      const data = await api.getModelRecovery()
      const items: ModelRecoveryItem[] = data.items || []
      setTotal(data.total || 0)

      const map: Record<string, NodeRecoveryInfo> = {}
      for (const item of items) {
        if (!map[item.node_id]) {
          map[item.node_id] = { count: 0, models: [] }
        }
        map[item.node_id].count++
        map[item.node_id].models.push(item.model_id)
      }
      setByNode(map)
    } catch {
      // silent fail - recovery info is supplementary
    }
  }, [])

  useEffect(() => {
    fetchData()
    timerRef.current = setInterval(fetchData, REFRESH_INTERVAL)
    return () => {
      if (timerRef.current) clearInterval(timerRef.current)
    }
  }, [fetchData])

  return (
    <ModelRecoveryContext.Provider value={{ byNode, total, refresh: fetchData }}>
      {children}
    </ModelRecoveryContext.Provider>
  )
}

export function useModelRecovery() {
  const ctx = useContext(ModelRecoveryContext)
  if (!ctx) {
    // Return safe defaults when used outside provider (e.g. SharedMonitor)
    return { byNode: {} as Record<string, NodeRecoveryInfo>, total: 0, refresh: () => {} }
  }
  return ctx
}
