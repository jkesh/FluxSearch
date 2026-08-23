import { FormEvent, useCallback, useEffect, useRef, useState, type DragEvent } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import {
  createImportJob,
  formatBytes,
  listImportJobs,
  type ImportJob,
} from '../lib/api'
import { useWebSocket } from '../hooks/useWebSocket'

const ACCEPT = '.md,.markdown,.txt,.html,.htm,.pdf'

const JOB_STATUS: Record<string, string> = {
  queued: '排队中',
  running: '进行中',
  completed: '已完成',
  partial: '部分成功',
  failed: '失败',
}

const ITEM_STATUS: Record<string, string> = {
  queued: '等待',
  processing: '处理中',
  done: '新建',
  skipped: '已跳过',
  updated: '已更新',
  failed: '失败',
}

function UploadIcon({ active }: { active?: boolean }) {
  return (
    <svg
      xmlns="http://www.w3.org/2000/svg"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.8"
      strokeLinecap="round"
      strokeLinejoin="round"
      className={`h-8 w-8 transition-colors ${active ? 'text-primary' : 'text-base-content/35'}`}
    >
      <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4" />
      <polyline points="17 8 12 3 7 8" />
      <line x1="12" y1="3" x2="12" y2="15" />
    </svg>
  )
}

function CloseIcon() {
  return (
    <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" className="h-3.5 w-3.5">
      <line x1="18" y1="6" x2="6" y2="18" />
      <line x1="6" y1="6" x2="18" y2="18" />
    </svg>
  )
}

export default function ImportPage() {
  const navigate = useNavigate()
  const inputRef = useRef<HTMLInputElement>(null)

  const [files, setFiles] = useState<File[]>([])
  const [dragActive, setDragActive] = useState(false)
  const [textTitle, setTextTitle] = useState('')
  const [textContent, setTextContent] = useState('')
  const [sourceType, setSourceType] = useState('markdown')
  const [uploadTab, setUploadTab] = useState<'file' | 'text'>('file')
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const [jobs, setJobs] = useState<ImportJob[]>([])

  const mergeJob = useCallback((incoming: ImportJob) => {
    setJobs((prev) => {
      const idx = prev.findIndex((j) => j.id === incoming.id)
      if (idx < 0) return [incoming, ...prev]
      const next = [...prev]
      next[idx] = incoming
      return next
    })
  }, [])

  useWebSocket({
    url: '/api/v1/ws/events',
    onMessage: (msg) => {
      if (msg.type !== 'import_progress' || !msg.job) return
      mergeJob(msg.job as ImportJob)
    },
  })

  const loadJobs = useCallback(async () => {
    try {
      const data = await listImportJobs(30)
      setJobs(data.jobs ?? [])
    } catch {
      /* ignore poll errors */
    }
  }, [])

  useEffect(() => {
    void loadJobs()
  }, [loadJobs])

  const hasRunning = jobs.some((j) => j.status === 'queued' || j.status === 'running')
  useEffect(() => {
    if (!hasRunning) return
    const timer = window.setInterval(() => void loadJobs(), 2000)
    return () => window.clearInterval(timer)
  }, [hasRunning, loadJobs])

  const addFiles = (incoming: FileList | null) => {
    if (!incoming?.length) return
    setFiles((prev) => {
      const seen = new Set(prev.map((f) => `${f.name}:${f.size}:${f.lastModified}`))
      const merged = [...prev]
      for (const f of Array.from(incoming)) {
        const key = `${f.name}:${f.size}:${f.lastModified}`
        if (!seen.has(key)) {
          seen.add(key)
          merged.push(f)
        }
      }
      return merged
    })
  }

  const onDrop = (e: DragEvent) => {
    e.preventDefault()
    setDragActive(false)
    addFiles(e.dataTransfer.files)
  }

  const onDragOver = (e: DragEvent) => {
    e.preventDefault()
    setDragActive(true)
  }

  const onDragLeave = (e: DragEvent) => {
    e.preventDefault()
    setDragActive(false)
  }

  const onFileSubmit = async (e: FormEvent) => {
    e.preventDefault()
    if (files.length === 0) return
    setSubmitting(true)
    setError(null)
    try {
      await createImportJob(files)
      setFiles([])
      if (inputRef.current) inputRef.current.value = ''
      await loadJobs()
    } catch (err) {
      setError(String(err))
    } finally {
      setSubmitting(false)
    }
  }

  const onTextSubmit = async (e: FormEvent) => {
    e.preventDefault()
    setSubmitting(true)
    setError(null)
    try {
      const blob = new Blob([textContent.trim()], { type: 'text/plain' })
      const file = new File([blob], `${textTitle.trim() || 'untitled'}.${sourceType === 'markdown' ? 'md' : sourceType}`, { type: 'text/plain' })
      await createImportJob([file])
      setTextContent('')
      setTextTitle('')
      await loadJobs()
    } catch (err) {
      setError(String(err))
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <div className="mx-auto w-full max-w-4xl space-y-5 px-4 py-8 md:px-6">
      <div className="animate-fade-up">
        <h1 className="text-xl font-bold tracking-tight">导入文档</h1>
        <p className="mt-1 text-sm text-base-content/50">
          上传后进入后台队列处理 · 完成后在
          <Link to="/documents" className="link link-primary mx-1">文档库</Link>
          查看
        </p>
      </div>

      <section className="surface overflow-hidden">
        <div className="border-b border-base-300 bg-base-200/40 px-4 py-3">
          <div role="tablist" className="tabs tabs-boxed tabs-sm w-fit">
            <button type="button" role="tab" className={`tab ${uploadTab === 'file' ? 'tab-active' : ''}`} onClick={() => setUploadTab('file')}>文件批量</button>
            <button type="button" role="tab" className={`tab ${uploadTab === 'text' ? 'tab-active' : ''}`} onClick={() => setUploadTab('text')}>粘贴文本</button>
          </div>
        </div>
        <div className="p-4 md:p-5">
          {uploadTab === 'file' ? (
            <form onSubmit={onFileSubmit} className="space-y-3">
              <label
                onDragOver={onDragOver}
                onDragEnter={onDragOver}
                onDragLeave={onDragLeave}
                onDrop={onDrop}
                className={`flex cursor-pointer flex-col items-center justify-center gap-2 rounded-xl border-2 border-dashed px-4 py-10 text-center transition-all duration-150 ${
                  dragActive
                    ? 'scale-[1.01] border-primary bg-primary/5'
                    : 'border-base-300 hover:border-primary/40 hover:bg-base-200/40'
                }`}
              >
                <UploadIcon active={dragActive} />
                <span className={`text-sm font-medium transition-colors ${dragActive ? 'text-primary' : 'text-base-content/75'}`}>
                  {dragActive ? '松开以添加文件' : '拖拽文件到此处，或点击选择'}
                </span>
                <span className="text-xs text-base-content/45">支持 .md .txt .html .pdf，可多选</span>
                <input ref={inputRef} type="file" accept={ACCEPT} multiple className="hidden" onChange={(e) => addFiles(e.target.files)} />
              </label>
              {files.length > 0 && (
                <ul className="max-h-32 space-y-1.5 overflow-y-auto pr-1">
                  {files.map((f, i) => (
                    <li
                      key={`${f.name}-${i}`}
                      className="flex items-center justify-between gap-2 rounded-lg bg-base-200/70 px-3 py-1.5"
                    >
                      <span className="truncate text-xs">{f.name}</span>
                      <span className="flex shrink-0 items-center gap-2">
                        <span className="text-[11px] tabular-nums text-base-content/40">{formatBytes(f.size)}</span>
                        <button
                          type="button"
                          title="移除"
                          className="btn btn-ghost btn-xs btn-circle text-base-content/50 hover:text-error"
                          onClick={() => setFiles((p) => p.filter((_, j) => j !== i))}
                        >
                          <CloseIcon />
                        </button>
                      </span>
                    </li>
                  ))}
                </ul>
              )}
              <button type="submit" className="btn btn-sm btn-primary shadow-sm" disabled={submitting || files.length === 0}>
                {submitting ? '提交中…' : files.length > 1 ? `加入队列 (${files.length})` : '加入队列'}
              </button>
            </form>
          ) : (
            <form onSubmit={onTextSubmit} className="space-y-3">
              <div className="flex flex-wrap gap-3">
                <input className="input input-sm input-bordered min-w-[160px] flex-1" placeholder="标题" value={textTitle} onChange={(e) => setTextTitle(e.target.value)} required />
                <select className="select select-sm select-bordered w-32" value={sourceType} onChange={(e) => setSourceType(e.target.value)}>
                  <option value="markdown">Markdown</option>
                  <option value="txt">纯文本</option>
                  <option value="html">HTML</option>
                </select>
              </div>
              <textarea className="textarea textarea-bordered min-h-32 w-full font-mono text-sm leading-relaxed" placeholder="粘贴内容…" value={textContent} onChange={(e) => setTextContent(e.target.value)} required />
              <button type="submit" className="btn btn-sm btn-primary shadow-sm" disabled={submitting}>{submitting ? '提交中…' : '加入队列'}</button>
            </form>
          )}
          {error && (
            <p className="mt-3 rounded-lg border border-error/30 bg-error/10 px-3 py-2 text-sm text-error">{error}</p>
          )}
        </div>
      </section>

      <section className="surface overflow-hidden">
        <div className="flex items-center justify-between gap-2 border-b border-base-300 bg-base-200/40 px-4 py-3">
          <h2 className="flex items-center gap-2 text-sm font-semibold">
            导入队列
            {hasRunning && <span className="loading loading-spinner loading-xs text-primary" />}
          </h2>
          <span className="flex items-center gap-3">
            <span className="text-xs text-base-content/40">WebSocket 实时更新</span>
            <button type="button" className="btn btn-ghost btn-xs" onClick={() => void loadJobs()}>刷新</button>
          </span>
        </div>
        <div className="divide-y divide-base-300">
          {jobs.length === 0 && (
            <p className="px-4 py-10 text-center text-sm text-base-content/50">暂无导入任务</p>
          )}
          {jobs.map((job) => (
            <div key={job.id} className="px-4 py-3.5 md:px-5">
              <div className="flex flex-wrap items-center justify-between gap-2">
                <div className="min-w-0">
                  <div className="flex items-center gap-2 text-sm font-medium">
                    <span className="font-mono text-xs text-base-content/40">{job.id.slice(0, 8)}</span>
                    <span className={`badge badge-xs ${job.status === 'completed' ? 'badge-success' : job.status === 'failed' ? 'badge-error' : 'badge-info'}`}>
                      {JOB_STATUS[job.status] ?? job.status}
                    </span>
                  </div>
                  <p className="mt-0.5 text-xs text-base-content/50">
                    {job.done}/{job.total} 成功 · {job.failed} 失败 · {new Date(job.created_at).toLocaleString()}
                  </p>
                </div>
                <span className="text-sm font-medium tabular-nums text-base-content/70">{job.progress}%</span>
              </div>
              <progress className="progress progress-primary mt-2 h-1.5 w-full" value={job.progress} max={100} />
              <ul className="mt-2 space-y-1">
                {job.items.map((item) => (
                  <li key={item.filename} className="flex flex-wrap items-center gap-2 text-xs">
                    <span
                      className={`badge badge-xs badge-ghost ${
                        item.status === 'done' || item.status === 'updated'
                          ? 'text-success'
                          : item.status === 'skipped'
                            ? 'text-warning'
                            : item.status === 'failed'
                              ? 'text-error'
                              : ''
                      }`}
                    >
                      {ITEM_STATUS[item.status] ?? item.status}
                    </span>
                    <span className="truncate">{item.filename}</span>
                    {item.document_id && (
                      <button type="button" className="link link-primary link-hover" onClick={() => navigate(`/documents/${item.document_id}`)}>
                        查看
                      </button>
                    )}
                    {item.error && <span className="text-error">{item.error}</span>}
                  </li>
                ))}
              </ul>
            </div>
          ))}
        </div>
      </section>
    </div>
  )
}
