import { useCallback, useEffect, useRef, useState } from 'react'

export type ChatSource = {
  chunk_id: string
  document_id: string
  title: string
  content: string
  score: number
  page?: number
}

export type WSMessage = {
  type: string
  content?: string
  sources?: ChatSource[]
  done?: boolean
  error?: string
  conversation_id?: string
  message_id?: string
  job?: unknown
  document_id?: string
  status?: string
  message?: string
}

type Options = {
  url: string
  onMessage?: (msg: WSMessage) => void
  onOpen?: () => void
  onClose?: () => void
  reconnect?: boolean
}

export function useWebSocket({ url, onMessage, onOpen, onClose, reconnect = true }: Options) {
  const wsRef = useRef<WebSocket | null>(null)
  const [connected, setConnected] = useState(false)
  const callbacks = useRef({ onMessage, onOpen, onClose })
  callbacks.current = { onMessage, onOpen, onClose }

  const connect = useCallback(() => {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    const wsUrl = url.startsWith('ws') ? url : `${protocol}//${window.location.host}${url}`
    const ws = new WebSocket(wsUrl)

    ws.onopen = () => {
      setConnected(true)
      callbacks.current.onOpen?.()
    }
    ws.onclose = () => {
      setConnected(false)
      callbacks.current.onClose?.()
      if (reconnect) {
        setTimeout(connect, 3000)
      }
    }
    ws.onmessage = (ev) => {
      try {
        const msg = JSON.parse(ev.data) as WSMessage
        callbacks.current.onMessage?.(msg)
      } catch {
        /* ignore */
      }
    }
    wsRef.current = ws
  }, [url, reconnect])

  useEffect(() => {
    connect()
    return () => {
      wsRef.current?.close()
    }
  }, [connect])

  const send = useCallback((data: unknown) => {
    if (wsRef.current?.readyState === WebSocket.OPEN) {
      wsRef.current.send(JSON.stringify(data))
    }
  }, [])

  return { connected, send }
}
