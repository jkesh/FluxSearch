import { useCallback, useEffect, useRef, useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import ChatComposer from '../components/chat/ChatComposer'
import HistorySidebar from '../components/chat/HistorySidebar'
import {
  ChatMessage,
  ConversationListItem,
  ChatSource,
  deleteConversation,
  fetchSettings,
  getConversation,
  listConversations,
} from '../lib/api'
import { useWebSocket } from '../hooks/useWebSocket'

type ChatItem = {
  role: 'user' | 'assistant'
  content: string
  sources?: ChatSource[]
}

const SUGGESTIONS = [
  '解释一下混合检索的原理',
  '如何部署 Monitor 服务？',
  'Milvus 连接失败怎么排查？',
]

function FileIcon() {
  return (
    <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" className="h-3 w-3 shrink-0 opacity-50">
      <path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z" />
      <polyline points="14 2 14 8 20 8" />
    </svg>
  )
}

function ModelIcon() {
  return (
    <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" className="h-5 w-5 text-primary">
      <path d="M12 2a4 4 0 0 1 4 4v1h1a3 3 0 0 1 3 3v2a3 3 0 0 1-3 3h-1v1a4 4 0 0 1-8 0v-1H7a3 3 0 0 1-3-3v-2a3 3 0 0 1 3-3h1V6a4 4 0 0 1 4-4z" />
      <circle cx="9" cy="13" r="1" /><circle cx="15" cy="13" r="1" />
    </svg>
  )
}

function SourceList({ sources }: { sources: ChatSource[] }) {
  if (sources.length === 0) return null
  return (
    <div className="mb-3 flex flex-wrap gap-1.5">
      {sources.map((src) => (
        <Link
          key={src.chunk_id}
          to={`/documents/${src.document_id}`}
          className="inline-flex max-w-full items-center gap-1.5 rounded-md border border-base-300/80 bg-base-100 px-2 py-1 text-xs text-base-content/60 transition-colors hover:border-primary/30 hover:text-primary"
          title={src.content.slice(0, 200)}
        >
          <FileIcon />
          <span className="truncate">{src.title || '未命名文档'}</span>
          {src.page != null && src.page > 0 && (
            <span className="shrink-0 text-base-content/35">p.{src.page}</span>
          )}
        </Link>
      ))}
    </div>
  )
}

function toChatItems(messages: ChatMessage[]): ChatItem[] {
  return messages
    .filter((m) => m.role === 'user' || m.role === 'assistant')
    .map((m) => ({
      role: m.role as 'user' | 'assistant',
      content: m.content,
      sources: m.sources,
    }))
}

export default function ChatPage() {
  const { id: routeId } = useParams()
  const navigate = useNavigate()
  const [conversationId, setConversationId] = useState<string | undefined>(routeId)
  const [conversations, setConversations] = useState<ConversationListItem[]>([])
  const [messages, setMessages] = useState<ChatItem[]>([])
  const [input, setInput] = useState('')
  const [streaming, setStreaming] = useState('')
  const [streamingSources, setStreamingSources] = useState<ChatSource[]>([])
  const [busy, setBusy] = useState(false)
  const [loading, setLoading] = useState(false)
  const [modelName, setModelName] = useState('')
  const [bannerDismissed, setBannerDismissed] = useState(false)
  const [historyOpen, setHistoryOpen] = useState(true)
  const scrollRef = useRef<HTMLDivElement>(null)
  const streamingRef = useRef('')
  const streamingSourcesRef = useRef<ChatSource[]>([])
  const conversationIdRef = useRef<string | undefined>(routeId)

  useEffect(() => {
    fetchSettings()
      .then((s) => {
        const name = s.llm_model || s.embedding_model
        if (name) setModelName(name)
      })
      .catch(() => {})
  }, [])

  const refreshList = useCallback(async () => {
    try {
      const data = await listConversations()
      setConversations(data.conversations ?? [])
    } catch {
      /* ignore */
    }
  }, [])

  useEffect(() => {
    refreshList()
  }, [refreshList])

  useEffect(() => {
    conversationIdRef.current = conversationId
  }, [conversationId])

  useEffect(() => {
    setConversationId(routeId)
    if (!routeId) {
      setMessages([])
      return
    }

    let cancelled = false
    setLoading(true)
    getConversation(routeId)
      .then((data) => {
        if (cancelled) return
        setMessages(toChatItems(data.messages ?? []))
      })
      .catch(() => {
        if (!cancelled) navigate('/', { replace: true })
      })
      .finally(() => {
        if (!cancelled) setLoading(false)
      })

    return () => {
      cancelled = true
    }
  }, [routeId, navigate])

  const { connected, send } = useWebSocket({
    url: '/api/v1/ws/chat',
    onMessage: (msg) => {
      if (msg.type === 'conversation' && msg.conversation_id) {
        setConversationId(msg.conversation_id)
        conversationIdRef.current = msg.conversation_id
        navigate(`/c/${msg.conversation_id}`, { replace: true })
        refreshList()
      }
      if (msg.type === 'sources' && msg.sources) {
        streamingSourcesRef.current = msg.sources
        setStreamingSources(msg.sources)
      }
      if (msg.type === 'token' && msg.content) {
        streamingRef.current += msg.content
        setStreaming(streamingRef.current)
      }
      if (msg.type === 'done') {
        const content = streamingRef.current
        const sources = streamingSourcesRef.current
        if (content || sources.length > 0) {
          setMessages((m) => [
            ...m,
            { role: 'assistant', content, sources: sources.length > 0 ? sources : undefined },
          ])
        }
        streamingRef.current = ''
        streamingSourcesRef.current = []
        setStreaming('')
        setStreamingSources([])
        setBusy(false)
        refreshList()
      }
      if (msg.type === 'error') {
        setMessages((m) => [...m, { role: 'assistant', content: msg.error ?? '发生错误' }])
        streamingRef.current = ''
        streamingSourcesRef.current = []
        setStreaming('')
        setStreamingSources([])
        setBusy(false)
      }
    },
  })

  useEffect(() => {
    const el = scrollRef.current
    if (el) el.scrollTop = el.scrollHeight
  }, [messages, streaming, streamingSources])

  const onNewChat = () => {
    setConversationId(undefined)
    conversationIdRef.current = undefined
    setMessages([])
    navigate('/')
  }

  const onDelete = async (id: string, e: React.MouseEvent) => {
    e.preventDefault()
    e.stopPropagation()
    if (!confirm('确定删除此对话？')) return
    try {
      await deleteConversation(id)
      if (conversationId === id) onNewChat()
      refreshList()
    } catch {
      /* ignore */
    }
  }

  const handleSend = () => {
    const text = input.trim()
    if (!text || !connected || busy) return
    setMessages((m) => [...m, { role: 'user', content: text }])
    streamingRef.current = ''
    streamingSourcesRef.current = []
    setStreaming('')
    setStreamingSources([])
    setBusy(true)
    send({ type: 'chat', content: text, conversation_id: conversationIdRef.current })
    setInput('')
  }

  const empty = messages.length === 0 && !streaming && !busy && !loading
  const displayModel = modelName || 'RAG 知识库助手'

  return (
    <section className="flex h-screen bg-base-200">
      {historyOpen && (
        <HistorySidebar
          conversations={conversations}
          activeId={conversationId}
          onNewChat={onNewChat}
          onDelete={onDelete}
        />
      )}

      <div className="flex min-w-0 flex-1 flex-col">
        {/* 顶栏 */}
        <header className="flex shrink-0 items-center justify-between border-b border-base-300/60 bg-base-100 px-4 py-2.5">
          <div className="flex items-center gap-2">
            <button
              type="button"
              onClick={() => setHistoryOpen((v) => !v)}
              className="rounded-lg px-2.5 py-1.5 text-xs text-base-content/55 transition-colors hover:bg-base-200"
            >
              {historyOpen ? '收起记录' : '对话记录'}
            </button>
          </div>
          <Link
            to="/settings"
            className="flex items-center gap-1.5 rounded-lg border border-base-300/80 bg-base-100 px-3 py-1.5 text-xs text-base-content/60 transition-colors hover:border-primary/30 hover:text-primary"
          >
            <ModelIcon />
            <span className="max-w-[160px] truncate font-medium">{displayModel}</span>
          </Link>
        </header>

        {/* 提示条 */}
        {!bannerDismissed && (
          <div className="flex shrink-0 items-center justify-between gap-3 border-b border-info/15 bg-info/[0.06] px-4 py-2 text-xs text-base-content/65">
            <span>FluxSearch 基于知识库 RAG 检索回答问题，引用来源将显示在回答上方。</span>
            <button
              type="button"
              onClick={() => setBannerDismissed(true)}
              className="shrink-0 rounded px-1.5 py-0.5 text-base-content/40 hover:bg-base-300/50 hover:text-base-content/70"
            >
              ✕
            </button>
          </div>
        )}

        {/* 主内容 */}
        <div ref={scrollRef} className="flex-1 overflow-y-auto">
          {loading ? (
            <div className="flex h-full items-center justify-center">
              <span className="loading loading-spinner loading-md text-primary" />
            </div>
          ) : empty ? (
            /* 空态：百炼风格居中欢迎 + 大输入框 */
            <div className="flex min-h-full flex-col items-center justify-center px-4 py-10">
              <div className="w-full max-w-2xl animate-fade-in">
                <div className="mb-8 text-center">
                  <h1 className="flex flex-wrap items-center justify-center gap-2 text-2xl font-semibold tracking-tight text-base-content/90 md:text-[28px]">
                    <span>欢迎使用</span>
                    <span className="inline-flex items-center gap-1.5 text-primary">
                      <ModelIcon />
                      FluxSearch
                    </span>
                    <span>知识库助手</span>
                  </h1>
                  <p className="mt-3 text-sm text-base-content/45">
                    当前模型：
                    <Link to="/settings" className="ml-1 text-primary hover:underline">
                      {displayModel}
                    </Link>
                  </p>
                </div>

                <ChatComposer
                  value={input}
                  onChange={setInput}
                  onSubmit={handleSend}
                  disabled={busy}
                  busy={busy}
                  connected={connected}
                  modelName={displayModel}
                />

                <div className="mt-6 flex flex-wrap items-center justify-center gap-2">
                  {SUGGESTIONS.map((s) => (
                    <button
                      key={s}
                      type="button"
                      onClick={() => setInput(s)}
                      className="rounded-full border border-base-300/80 bg-base-100 px-4 py-1.5 text-xs text-base-content/55 transition-all hover:border-primary/30 hover:text-primary"
                    >
                      {s}
                    </button>
                  ))}
                </div>
              </div>
            </div>
          ) : (
            /* 对话态 */
            <div className="mx-auto w-full max-w-3xl space-y-8 px-4 py-8 pb-4">
              {messages.map((m, i) =>
                m.role === 'user' ? (
                  <div key={i} className="flex animate-fade-up justify-end">
                    <div className="max-w-[85%] whitespace-pre-wrap break-words rounded-2xl rounded-br-md bg-primary px-4 py-3 text-[15px] leading-relaxed text-primary-content">
                      {m.content}
                    </div>
                  </div>
                ) : (
                  <div key={i} className="animate-fade-up">
                    {m.sources && m.sources.length > 0 && <SourceList sources={m.sources} />}
                    <div className="whitespace-pre-wrap break-words text-[15px] leading-7 text-base-content/85">
                      {m.content}
                    </div>
                  </div>
                ),
              )}
              {(streaming || busy) && (
                <div className="animate-fade-up">
                  {streamingSources.length > 0 && <SourceList sources={streamingSources} />}
                  <div className="whitespace-pre-wrap break-words text-[15px] leading-7 text-base-content/85">
                    {streaming || (busy && !streaming ? '正在检索知识库并生成回答…' : '')}
                    {busy && (
                      <span className="ml-1 inline-block h-4 w-1 animate-pulse rounded-sm bg-primary/60 align-middle" />
                    )}
                  </div>
                </div>
              )}
            </div>
          )}
        </div>

        {/* 对话态底部输入框 */}
        {!empty && !loading && (
          <div className="shrink-0 border-t border-base-300/60 bg-base-200/80 px-4 py-4 backdrop-blur-sm">
            <div className="mx-auto w-full max-w-3xl">
              <ChatComposer
                value={input}
                onChange={setInput}
                onSubmit={handleSend}
                disabled={busy}
                busy={busy}
                connected={connected}
                modelName={displayModel}
              />
            </div>
          </div>
        )}
      </div>
    </section>
  )
}
