import { useCallback, useEffect, useState } from 'react'

export type ThemeMode = 'light' | 'dark'

const STORAGE_KEY = 'fluxsearch-theme'

function resolveInitialMode(): ThemeMode {
  try {
    const saved = window.localStorage.getItem(STORAGE_KEY)
    if (saved === 'light' || saved === 'dark') return saved
  } catch {
    /* ignore */
  }
  return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light'
}

function applyTheme(mode: ThemeMode) {
  const root = document.documentElement
  root.dataset.theme = mode === 'dark' ? 'fluxdark' : 'fluxlight'
  root.style.colorScheme = mode
}

export function useTheme() {
  const [mode, setMode] = useState<ThemeMode>(resolveInitialMode)

  useEffect(() => {
    applyTheme(mode)
    try {
      window.localStorage.setItem(STORAGE_KEY, mode)
    } catch {
      /* ignore */
    }
  }, [mode])

  const toggle = useCallback(() => {
    setMode((m) => (m === 'dark' ? 'light' : 'dark'))
  }, [])

  return { mode, toggle }
}
