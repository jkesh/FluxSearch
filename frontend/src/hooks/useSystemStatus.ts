import { useCallback, useEffect, useState } from 'react'
import { fetchSystemStatus, type SystemStatusReport } from '../lib/api'

const REFRESH_INTERVAL = 15_000

export function useSystemStatus() {
  const [report, setReport] = useState<SystemStatusReport | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const refresh = useCallback(async () => {
    try {
      const data = await fetchSystemStatus()
      setReport(data)
      setError(null)
    } catch (e) {
      setError(String(e))
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    refresh()
    const timer = setInterval(refresh, REFRESH_INTERVAL)
    return () => clearInterval(timer)
  }, [refresh])

  return { report, loading, error, refresh, intervalSec: REFRESH_INTERVAL / 1000 }
}
