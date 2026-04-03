import { useEffect, useState } from 'react'
import api from '../services/api'

export interface VersionInfo {
  version: string
  git_commit: string
  build_date: string
  build_date_beijing: string
  go_version: string
  environment: string
}

interface UseVersionResult {
  version: VersionInfo | null
  loading: boolean
  error: Error | null
}

export function useVersion(): UseVersionResult {
  const [version, setVersion] = useState<VersionInfo | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<Error | null>(null)

  useEffect(() => {
    let cancelled = false

    const fetchVersion = async () => {
      setLoading(true)
      setError(null)
      try {
        const data = await api.getVersion()
        if (!cancelled) {
          setVersion(data)
        }
      } catch (err) {
        if (!cancelled) {
          setError(err instanceof Error ? err : new Error('加载版本信息失败'))
        }
      } finally {
        if (!cancelled) {
          setLoading(false)
        }
      }
    }

    fetchVersion()

    return () => {
      cancelled = true
    }
  }, [])

  return { version, loading, error }
}

export default useVersion
