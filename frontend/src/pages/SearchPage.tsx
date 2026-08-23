import { FormEvent, useState } from 'react'
import { Link } from 'react-router-dom'
import { search, type SearchResponse, type SearchResultItem } from '../lib/api'

function SearchIcon() {
  return (
    <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" className="h-4 w-4 shrink-0 text-base-content/40">
      <circle cx="11" cy="11" r="7" />
      <path d="m21 21-4.35-4.35" />
    </svg>
  )
}

function EmptyIcon() {
  return (
    <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" className="h-8 w-8 text-base-content/30">
      <circle cx="11" cy="11" r="7" />
      <path d="m21 21-4.35-4.35" />
    </svg>
  )
}

function ResultCard({ item, rank, delay }: { item: SearchResultItem; rank: number; delay: number }) {
  return (
    <article
      className="surface surface-hover animate-fade-up p-5"
      style={{ animationDelay: `${delay}ms` }}
    >
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div className="min-w-0 flex-1">
          <div className="flex flex-wrap items-center gap-2 text-xs">
            <span className="rounded-md bg-primary/10 px-1.5 py-0.5 font-mono text-[11px] font-semibold text-primary">
              #{rank}
            </span>
            <span className="truncate font-medium text-base-content/85">{item.document_title || '未命名文档'}</span>
            {item.page != null && item.page > 0 && (
              <span className="text-base-content/45">第 {item.page} 页</span>
            )}
          </div>
          <p className="mt-2.5 line-clamp-4 text-sm leading-6 text-base-content/80">{item.content}</p>
        </div>
        <div className="flex shrink-0 flex-col items-end gap-2">
          <span
            className="rounded-lg bg-base-200 px-2.5 py-1 font-mono text-xs tabular-nums text-base-content/70"
            title="相关度分数"
          >
            {item.score.toFixed(4)}
          </span>
          <Link to={`/documents/${item.document_id}`} className="btn btn-ghost btn-xs text-primary hover:bg-primary/10">
            查看文档
          </Link>
        </div>
      </div>
    </article>
  )
}

export default function SearchPage() {
  const [query, setQuery] = useState('')
  const [result, setResult] = useState<SearchResponse | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(false)

  const onSubmit = async (e: FormEvent) => {
    e.preventDefault()
    const q = query.trim()
    if (!q) return
    setLoading(true)
    setError(null)
    try {
      const data = await search(q)
      setResult(data)
    } catch (err) {
      setError(String(err))
      setResult(null)
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="mx-auto w-full max-w-4xl space-y-5 px-4 py-8 md:px-6">
      <div className="animate-fade-up">
        <h1 className="text-xl font-bold tracking-tight">文档检索</h1>
        <p className="mt-1 text-sm text-base-content/50">向量语义检索 · 返回最相关的文档片段</p>
      </div>

      <form
        onSubmit={onSubmit}
        className="surface flex animate-fade-up items-center gap-2 p-2 pl-4 transition-all focus-within:border-primary/60 focus-within:ring-4 focus-within:ring-primary/10"
        style={{ animationDelay: '60ms' }}
      >
        <SearchIcon />
        <input
          className="min-w-0 flex-1 bg-transparent py-1.5 text-sm outline-none placeholder:text-base-content/40"
          placeholder="输入问题或关键词，例如：Milvus 如何连接"
          value={query}
          onChange={(e) => setQuery(e.target.value)}
        />
        <button type="submit" className="btn btn-sm btn-primary rounded-lg px-5 shadow-sm" disabled={loading || !query.trim()}>
          {loading ? <span className="loading loading-spinner loading-xs" /> : '检索'}
        </button>
      </form>

      {error && (
        <div className="animate-fade-up rounded-xl border border-error/30 bg-error/10 px-4 py-3 text-sm text-error">
          {error}
        </div>
      )}

      {result && (
        <section className="space-y-3">
          <div className="flex flex-wrap items-center gap-2 text-xs text-base-content/50">
            <span className="rounded-full bg-base-100 px-2.5 py-1 shadow-sm ring-1 ring-base-300">
              查询：<span className="font-medium text-base-content/75">{result.query}</span>
            </span>
            <span className="rounded-full bg-base-100 px-2.5 py-1 shadow-sm ring-1 ring-base-300">
              {result.count} 条结果
            </span>
            <span className="rounded-full bg-base-100 px-2.5 py-1 shadow-sm ring-1 ring-base-300">
              Top {result.top_k}
            </span>
          </div>
          {result.results.length === 0 ? (
            <div className="surface flex animate-fade-up flex-col items-center gap-3 px-4 py-14 text-center">
              <EmptyIcon />
              <p className="text-sm text-base-content/50">未找到相关片段。请确认文档已导入并完成向量化。</p>
              <Link to="/import" className="btn btn-sm btn-outline btn-primary">
                去导入文档
              </Link>
            </div>
          ) : (
            result.results.map((item, i) => (
              <ResultCard key={item.chunk_id} item={item} rank={i + 1} delay={Math.min(i * 40, 240)} />
            ))
          )}
        </section>
      )}
    </div>
  )
}
