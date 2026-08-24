import { useCallback, useEffect, useRef, useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import {
  collectionLabel,
  DEFAULT_COLLECTION_ID,
  deleteDocument,
  getDocument,
  listCollections,
  listDocuments,
  rechunkDocument,
  reimportDocument,
  type Chunk,
  type Collection,
  type Document,
  type DocumentListItem,
} from '../lib/api'
import { useWebSocket } from '../hooks/useWebSocket'

const STATUS_LABEL: Record<string, string> = {
  pending: '待处理',
  processing: '处理中',
  indexed: '已分块',
  failed: '失败',
}

const STATUS_BADGE: Record<string, string> = {
  pending: 'badge-ghost',
  processing: 'badge-info',
  indexed: 'badge-success',
  failed: 'badge-error',
}

function EmptyDocIcon() {
  return (
    <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" className="h-10 w-10 text-base-content/25">
      <path d="M4 19.5A2.5 2.5 0 0 1 6.5 17H20" />
      <path d="M6.5 2H20v20H6.5A2.5 2.5 0 0 1 4 19.5v-15A2.5 2.5 0 0 1 6.5 2z" />
    </svg>
  )
}

function formatTime(iso: string) {
  try {
    return new Date(iso).toLocaleString()
  } catch {
    return iso
  }
}

const COLLECTION_STORAGE_KEY = 'fluxsearch:documents:collection_id'

function loadStoredCollectionId(): string {
  try {
    return localStorage.getItem(COLLECTION_STORAGE_KEY) || DEFAULT_COLLECTION_ID
  } catch {
    return DEFAULT_COLLECTION_ID
  }
}

export default function DocumentsPage() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()

  const [collections, setCollections] = useState<Collection[]>([])
  const [collectionId, setCollectionId] = useState(loadStoredCollectionId)
  const [documents, setDocuments] = useState<DocumentListItem[]>([])
  const [listLoading, setListLoading] = useState(false)
  const [listError, setListError] = useState<string | null>(null)

  const [document, setDocument] = useState<Document | null>(null)
  const [chunks, setChunks] = useState<Chunk[]>([])
  const [detailLoading, setDetailLoading] = useState(false)
  const [detailTab, setDetailTab] = useState<'source' | 'chunks'>('source')
  const [rechunking, setRechunking] = useState(false)
  const [reimporting, setReimporting] = useState(false)
  const [deleting, setDeleting] = useState(false)
  const [actionMsg, setActionMsg] = useState<string | null>(null)
  const fileInputRef = useRef<HTMLInputElement>(null)
  const detailIdRef = useRef<string | undefined>(id)
  detailIdRef.current = id

  const loadDocuments = useCallback(async () => {
    setListLoading(true)
    setListError(null)
    try {
      const data = await listDocuments(200, 0, collectionId)
      setDocuments(data.documents ?? [])
    } catch (err) {
      setListError(String(err))
    } finally {
      setListLoading(false)
    }
  }, [collectionId])

  const loadCollections = useCallback(async () => {
    try {
      const data = await listCollections()
      const items = data.collections ?? []
      setCollections(items)
      if (items.length > 0 && !items.some((c) => c.id === collectionId)) {
        const fallback = items.find((c) => c.id === DEFAULT_COLLECTION_ID) ?? items[0]
        setCollectionId(fallback.id)
      }
    } catch {
      // keep current selection; list still works with collection_id param
    }
  }, [collectionId])

  const loadDetail = useCallback(async (docId: string) => {
    setDetailLoading(true)
    setActionMsg(null)
    try {
      const data = await getDocument(docId)
      setDocument(data.document)
      setChunks(data.chunks ?? [])
    } catch (err) {
      setDocument(null)
      setChunks([])
      setActionMsg(String(err))
    } finally {
      setDetailLoading(false)
    }
  }, [])

  useWebSocket({
    url: '/api/v1/ws/events',
    onMessage: (ev) => {
      const docId = detailIdRef.current
      if (!docId || ev.document_id !== docId) return
      if (ev.type === 'document.reindex' && ev.status === 'processing') {
        setActionMsg('正在后台重新索引…')
      }
      if (ev.type === 'document.updated' || (ev.type === 'document.reindex' && ev.status === 'failed')) {
        if (ev.type === 'document.updated') {
          setActionMsg('重新索引完成')
          void loadDocuments()
          void loadDetail(docId)
        } else if (ev.message || ev.error) {
          setActionMsg(ev.message ?? ev.error ?? '重新索引失败')
        }
        setRechunking(false)
        setReimporting(false)
      }
    },
  })

  useEffect(() => {
    void loadCollections()
  }, [loadCollections])

  useEffect(() => {
    void loadDocuments()
  }, [loadDocuments])

  useEffect(() => {
    try {
      localStorage.setItem(COLLECTION_STORAGE_KEY, collectionId)
    } catch {
      // ignore
    }
  }, [collectionId])

  const onCollectionChange = (nextId: string) => {
    if (nextId === collectionId) return
    setCollectionId(nextId)
    if (id) navigate('/documents')
  }

  const activeCollection = collections.find((c) => c.id === collectionId)

  useEffect(() => {
    if (!id) {
      setDocument(null)
      setChunks([])
      return
    }
    void loadDetail(id)
  }, [id, loadDetail])

  const onDelete = async () => {
    if (!id || !document) return
    if (!window.confirm(`确定删除「${document.title}」？此操作不可恢复。`)) return
    setDeleting(true)
    setActionMsg(null)
    try {
      await deleteDocument(id)
      setActionMsg('文档已删除')
      await loadDocuments()
      navigate('/documents')
    } catch (err) {
      setActionMsg(String(err))
    } finally {
      setDeleting(false)
    }
  }

  const onRechunk = async () => {
    if (!id) return
    setRechunking(true)
    setActionMsg(null)
    try {
      await rechunkDocument(id, true)
      setActionMsg('重新分块已加入队列，请稍候…')
    } catch (err) {
      setActionMsg(String(err))
      setRechunking(false)
    }
  }

  const onReimportFile = async (file: File) => {
    if (!id) return
    setReimporting(true)
    setActionMsg(null)
    try {
      await reimportDocument(id, file)
      setActionMsg('重新导入已加入队列，请稍候…')
    } catch (err) {
      setActionMsg(String(err))
      setReimporting(false)
    }
  }

  const onPickReimport = () => {
    fileInputRef.current?.click()
  }

  const renderSourceContent = () => {
    if (!document) return null
    if (document.content_pages && document.content_pages.length > 0) {
      return (
        <div className="space-y-4">
          {document.content_pages.map((p) => (
            <div key={p.page} className="surface p-4">
              <span className="badge badge-sm badge-ghost font-medium">第 {p.page} 页</span>
              <pre className="mt-2.5 whitespace-pre-wrap break-words font-mono text-sm leading-relaxed">{p.text}</pre>
            </div>
          ))}
        </div>
      )
    }
    if (document.content) {
      return (
        <div className="surface p-4">
          <pre className="whitespace-pre-wrap break-words font-mono text-sm leading-relaxed">{document.content}</pre>
        </div>
      )
    }
    return <p className="text-sm text-base-content/50">无存储原文（请重新导入）</p>
  }

  return (
    <div className="flex h-[calc(100vh)] flex-col">
      <header className="shrink-0 border-b border-base-300 bg-base-100 px-4 py-3 md:px-6">
        <div className="flex flex-wrap items-center justify-between gap-2">
          <div>
            <h1 className="text-lg font-semibold">文档库</h1>
            <p className="text-xs text-base-content/50">浏览原文 · 分块预览 · 异步重新导入/分块</p>
          </div>
          <div className="flex flex-wrap items-center gap-2">
            <label className="flex items-center gap-2 text-sm">
              <span className="text-base-content/50">知识库</span>
              <select
                className="select select-sm select-bordered min-w-[12rem]"
                value={collectionId}
                onChange={(e) => onCollectionChange(e.target.value)}
                disabled={listLoading && collections.length === 0}
              >
                {collections.length === 0 ? (
                  <option value={collectionId}>加载中…</option>
                ) : (
                  collections.map((c) => (
                    <option key={c.id} value={c.id}>
                      {collectionLabel(c)}
                    </option>
                  ))
                )}
              </select>
            </label>
            <button type="button" className="btn btn-sm btn-outline" onClick={() => void loadDocuments()} disabled={listLoading}>刷新</button>
            <Link to="/import" className="btn btn-sm btn-primary">去导入</Link>
          </div>
        </div>
      </header>

      <div className="flex min-h-0 flex-1 flex-col md:flex-row">
        <aside className="flex w-full shrink-0 flex-col border-b border-base-300 bg-base-100 md:w-80 md:border-b-0 md:border-r">
          <div className="border-b border-base-300 px-4 py-2.5 text-xs font-medium text-base-content/50">
            {activeCollection ? collectionLabel(activeCollection) : '文档列表'}
            {documents.length > 0 && <span className="tabular-nums"> ({documents.length})</span>}
          </div>
          <div className="min-h-0 flex-1 overflow-y-auto">
            {listError && <p className="px-4 py-4 text-sm text-error">{listError}</p>}
            {!listError && documents.length === 0 && !listLoading && (
              <div className="flex flex-col items-center gap-2 px-4 py-12 text-center">
                <EmptyDocIcon />
                <p className="text-sm text-base-content/50">暂无文档</p>
              </div>
            )}
            <ul className="divide-y divide-base-300">
              {documents.map((doc) => (
                <li key={doc.id}>
                  <button
                    type="button"
                    onClick={() => navigate(`/documents/${doc.id}`)}
                    className={`relative w-full px-4 py-3 text-left transition-colors hover:bg-base-200/70 ${
                      id === doc.id ? 'bg-primary/5' : ''
                    }`}
                  >
                    {id === doc.id && (
                      <span className="absolute left-0 top-0 h-full w-0.5 bg-primary" />
                    )}
                    <div className="flex items-start justify-between gap-2">
                      <span className={`line-clamp-1 text-sm font-medium ${id === doc.id ? 'text-primary' : ''}`}>
                        {doc.title}
                      </span>
                      <span className={`badge badge-xs shrink-0 ${STATUS_BADGE[doc.status] ?? 'badge-ghost'}`}>
                        {STATUS_LABEL[doc.status] ?? doc.status}
                      </span>
                    </div>
                    <p className="mt-1 line-clamp-2 text-xs leading-relaxed text-base-content/55">{doc.content_preview || '（无摘要）'}</p>
                    <div className="mt-1.5 flex gap-x-2 text-[11px] text-base-content/40">
                      <span>{doc.source_type}</span><span>·</span><span className="tabular-nums">{doc.chunk_count} 块</span><span>·</span><span>v{doc.version}</span>
                    </div>
                  </button>
                </li>
              ))}
            </ul>
          </div>
        </aside>

        <section className="flex min-h-0 min-w-0 flex-1 flex-col bg-base-200/30">
          {!id ? (
            <div className="flex flex-1 flex-col items-center justify-center gap-3 text-sm text-base-content/40">
              <EmptyDocIcon />
              <p>从左侧选择文档</p>
              <Link to="/import" className="btn btn-sm btn-outline btn-primary mt-1">导入新文档</Link>
            </div>
          ) : detailLoading ? (
            <div className="flex flex-1 items-center justify-center"><span className="loading loading-spinner loading-md" /></div>
          ) : document ? (
            <>
              <div className="shrink-0 border-b border-base-300 bg-base-100 px-4 py-3">
                <div className="flex flex-wrap items-start justify-between gap-3">
                  <div className="min-w-0">
                    <h2 className="truncate text-base font-semibold">{document.title}</h2>
                    <p className="mt-0.5 text-xs text-base-content/50">
                      {document.source_type} · {document.chunk_count} 块 · v{document.version} · {formatTime(document.created_at)}
                    </p>
                  </div>
                  <div className="flex gap-2">
                    <input
                      ref={fileInputRef}
                      type="file"
                      className="hidden"
                      accept=".pdf,.md,.markdown,.txt,.docx"
                      onChange={(e) => {
                        const file = e.target.files?.[0]
                        e.target.value = ''
                        if (file) void onReimportFile(file)
                      }}
                    />
                    <button type="button" className="btn btn-sm btn-outline" onClick={onPickReimport} disabled={reimporting}>
                      {reimporting ? <span className="loading loading-spinner loading-xs" /> : null}
                      重新导入
                    </button>
                    <button type="button" className="btn btn-sm btn-outline" onClick={() => void onRechunk()} disabled={rechunking || !document.content}>
                      {rechunking ? <span className="loading loading-spinner loading-xs" /> : null}
                      重新分块
                    </button>
                    <button type="button" className="btn btn-sm btn-outline btn-error" onClick={() => void onDelete()} disabled={deleting}>
                      {deleting ? <span className="loading loading-spinner loading-xs" /> : null}
                      删除
                    </button>
                  </div>
                </div>
                {actionMsg && (
                  <p className={`mt-2 text-xs ${actionMsg.includes('完成') || actionMsg.includes('已删除') || actionMsg.includes('队列') ? 'text-success' : 'text-error'}`}>{actionMsg}</p>
                )}
                <div role="tablist" className="tabs tabs-boxed tabs-sm mt-3 w-fit">
                  <button type="button" role="tab" className={`tab ${detailTab === 'source' ? 'tab-active' : ''}`} onClick={() => setDetailTab('source')}>原文</button>
                  <button type="button" role="tab" className={`tab ${detailTab === 'chunks' ? 'tab-active' : ''}`} onClick={() => setDetailTab('chunks')}>分块 ({chunks.length})</button>
                </div>
              </div>
              <div className="min-h-0 flex-1 overflow-y-auto p-4">
                {detailTab === 'source' ? renderSourceContent() : chunks.length === 0 ? (
                  <p className="text-sm text-base-content/50">暂无分块</p>
                ) : (
                  <div className="space-y-3">
                    {chunks.map((ch) => (
                      <article key={ch.id} className="surface p-4 transition-shadow hover:shadow-md">
                        <header className="mb-2 flex flex-wrap items-center gap-2 text-xs text-base-content/50">
                          <span className="rounded-md bg-primary/10 px-1.5 py-0.5 font-mono text-[11px] font-semibold text-primary">
                            #{ch.chunk_index}
                          </span>
                          {ch.page != null && ch.page > 0 && <span>第 {ch.page} 页</span>}
                          <span className="tabular-nums">{ch.token_count} tokens</span>
                        </header>
                        <pre className="whitespace-pre-wrap break-words text-sm leading-relaxed">{ch.content}</pre>
                      </article>
                    ))}
                  </div>
                )}
              </div>
            </>
          ) : null}
        </section>
      </div>
    </div>
  )
}
