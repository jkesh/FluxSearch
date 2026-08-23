import { FormEvent, useCallback, useEffect, useState } from 'react'
import {
  fetchSettings,
  updateSettings,
  type AppSettings,
  type AppSettingsUpdate,
} from '../lib/api'
import MonitorPanel from '../components/MonitorPanel'
import IngestionPanel from './settings/IngestionPanel'
import ModelsPanel from './settings/ModelsPanel'
import SearchPanel from './settings/SearchPanel'
import SystemPanel from './settings/SystemPanel'

type TopTab = 'config' | 'monitor'
type ConfigTab = 'models' | 'ingestion' | 'search' | 'system'

const CONFIG_TABS: { id: ConfigTab; label: string }[] = [
  { id: 'models', label: '模型' },
  { id: 'ingestion', label: '导入与去重' },
  { id: 'search', label: '检索' },
  { id: 'system', label: '系统' },
]

function emptyForm(): AppSettings {
  return {
    monitor_url: '',
    embedding_provider: '',
    embedding_local_backend: 'ollama',
    embedding_api_url: '',
    embedding_model: '',
    embedding_dim: 1024,
    embedding_batch_size: 16,
    llm_provider: '',
    llm_local_backend: 'ollama',
    llm_api_url: '',
    llm_model: '',
    llm_temperature: 0.7,
    llm_max_tokens: 2048,
    chunk_max_chars: 2048,
    chunk_overlap: 256,
    search_top_k: 5,
    search_score_threshold: 0,
    milvus_index_type: 'ivf_flat',
    milvus_metric: 'IP',
    milvus_nlist: 128,
    milvus_nprobe: 16,
    milvus_hnsw_m: 16,
    milvus_hnsw_ef_construction: 200,
    milvus_hnsw_ef: 64,
    document_dedup_enabled: true,
    document_dedup_mode: 'skip',
    document_dedup_by_content_hash: true,
    document_dedup_by_source_uri: true,
    chunk_dedup_enabled: false,
    chunk_dedup_scope: 'collection',
    embedding_api_key_set: false,
    llm_api_key_set: false,
    settings_path: '',
    embedding_ready: false,
    embedding_status: '',
    reindex: { running: false, total: 0, done: 0, failed: 0 },
  }
}

export default function SettingsPage() {
  const [topTab, setTopTab] = useState<TopTab>('config')
  const [configTab, setConfigTab] = useState<ConfigTab>('models')
  const [form, setForm] = useState<AppSettings>(emptyForm)
  const [embeddingKey, setEmbeddingKey] = useState('')
  const [llmKey, setLlmKey] = useState('')
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [message, setMessage] = useState<string | null>(null)

  const load = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      const data = await fetchSettings()
      setForm(data)
      setEmbeddingKey('')
      setLlmKey('')
    } catch (err) {
      setError(String(err))
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    load()
  }, [load])

  useEffect(() => {
    if (!form.reindex?.running) return
    const timer = window.setInterval(() => {
      fetchSettings()
        .then((data) => setForm(data))
        .catch(() => {})
    }, 3000)
    return () => window.clearInterval(timer)
  }, [form.reindex?.running])

  const patch = <K extends keyof AppSettings>(key: K, value: AppSettings[K]) => {
    setForm((prev) => ({ ...prev, [key]: value }))
  }

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault()
    setSaving(true)
    setError(null)
    setMessage(null)
    try {
      const payload: AppSettingsUpdate = {
        monitor_url: form.monitor_url,
        embedding_provider: form.embedding_provider,
        embedding_local_backend: form.embedding_local_backend,
        embedding_api_url: form.embedding_api_url,
        embedding_model: form.embedding_model,
        embedding_dim: form.embedding_dim,
        embedding_batch_size: form.embedding_batch_size,
        llm_provider: form.llm_provider,
        llm_local_backend: form.llm_local_backend,
        llm_api_url: form.llm_api_url,
        llm_model: form.llm_model,
        llm_temperature: form.llm_temperature,
        llm_max_tokens: form.llm_max_tokens,
        chunk_max_chars: form.chunk_max_chars,
        chunk_overlap: form.chunk_overlap,
        search_top_k: form.search_top_k,
        search_score_threshold: form.search_score_threshold,
        milvus_index_type: form.milvus_index_type,
        milvus_metric: form.milvus_metric,
        milvus_nlist: form.milvus_nlist,
        milvus_nprobe: form.milvus_nprobe,
        milvus_hnsw_m: form.milvus_hnsw_m,
        milvus_hnsw_ef_construction: form.milvus_hnsw_ef_construction,
        milvus_hnsw_ef: form.milvus_hnsw_ef,
        document_dedup_enabled: form.document_dedup_enabled,
        document_dedup_mode: form.document_dedup_mode,
        document_dedup_by_content_hash: form.document_dedup_by_content_hash,
        document_dedup_by_source_uri: form.document_dedup_by_source_uri,
        chunk_dedup_enabled: form.chunk_dedup_enabled,
        chunk_dedup_scope: form.chunk_dedup_scope,
      }
      if (embeddingKey.trim()) payload.embedding_api_key = embeddingKey.trim()
      if (llmKey.trim()) payload.llm_api_key = llmKey.trim()

      const res = await updateSettings(payload)
      setForm(res.settings)
      setEmbeddingKey('')
      setLlmKey('')
      setMessage(res.message)
    } catch (err) {
      setError(String(err))
    } finally {
      setSaving(false)
    }
  }

  return (
    <div className="mx-auto w-full max-w-4xl space-y-5 px-4 py-8 md:px-6">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h1 className="text-xl font-bold tracking-tight">设置</h1>
          <p className="mt-1 text-sm text-base-content/50">分模块配置模型、导入去重与检索参数</p>
        </div>
        <div role="tablist" className="tabs tabs-boxed tabs-sm">
          <button
            type="button"
            role="tab"
            className={`tab ${topTab === 'config' ? 'tab-active' : ''}`}
            onClick={() => setTopTab('config')}
          >
            应用配置
          </button>
          <button
            type="button"
            role="tab"
            className={`tab ${topTab === 'monitor' ? 'tab-active' : ''}`}
            onClick={() => setTopTab('monitor')}
          >
            系统监控
          </button>
        </div>
      </div>

      {topTab === 'monitor' && <MonitorPanel />}

      {topTab === 'config' && (
        <>
          {loading ? (
            <div className="flex justify-center py-12">
              <span className="loading loading-spinner loading-md" />
            </div>
          ) : (
            <form onSubmit={handleSubmit} className="space-y-5">
              {error && (
                <div className="animate-fade-up rounded-xl border border-error/30 bg-error/10 px-4 py-3 text-sm text-error">
                  {error}
                </div>
              )}
              {message && (
                <div className="animate-fade-up rounded-xl border border-success/30 bg-success/10 px-4 py-3 text-sm text-success">
                  {message}
                </div>
              )}

              <div className="surface flex flex-wrap items-center gap-3 px-4 py-3 text-sm">
                <span className={`badge badge-sm ${form.embedding_ready ? 'badge-success' : 'badge-ghost'}`}>
                  Embedding {form.embedding_ready ? '已就绪' : '未启用'}
                </span>
                <span className="text-xs text-base-content/60">{form.embedding_status || '—'}</span>
                {form.reindex?.running && (
                  <span className="badge badge-sm badge-info gap-1">
                    <span className="loading loading-spinner loading-xs" />
                    重处理 {form.reindex.done}/{form.reindex.total}
                  </span>
                )}
              </div>

              {form.reindex?.message && (
                <div
                  className={`rounded-xl border px-4 py-3 text-sm ${
                    form.reindex.failed > 0
                      ? 'border-warning/30 bg-warning/10 text-warning'
                      : 'border-base-300 bg-base-100 text-base-content/70'
                  }`}
                >
                  {form.reindex.message}
                  {form.reindex.last_error && (
                    <div className="mt-1 text-xs opacity-80">{form.reindex.last_error}</div>
                  )}
                </div>
              )}

              <div role="tablist" className="tabs tabs-boxed tabs-sm w-fit max-w-full overflow-x-auto">
                {CONFIG_TABS.map(({ id, label }) => (
                  <button
                    key={id}
                    type="button"
                    role="tab"
                    className={`tab whitespace-nowrap ${configTab === id ? 'tab-active' : ''}`}
                    onClick={() => setConfigTab(id)}
                  >
                    {label}
                  </button>
                ))}
              </div>

              {configTab === 'models' && (
                <ModelsPanel
                  form={form}
                  embeddingKey={embeddingKey}
                  llmKey={llmKey}
                  setEmbeddingKey={setEmbeddingKey}
                  setLlmKey={setLlmKey}
                  patch={patch}
                />
              )}
              {configTab === 'ingestion' && <IngestionPanel form={form} patch={patch} />}
              {configTab === 'search' && <SearchPanel form={form} patch={patch} />}
              {configTab === 'system' && <SystemPanel form={form} patch={patch} />}

              <div className="sticky bottom-4 z-20 flex flex-wrap items-center gap-2 rounded-xl border border-base-300 bg-base-100/95 px-4 py-3 shadow-lift backdrop-blur">
                <button type="submit" className="btn btn-primary btn-sm shadow-sm" disabled={saving}>
                  {saving ? <span className="loading loading-spinner loading-xs" /> : null}
                  保存并应用
                </button>
                <button type="button" className="btn btn-ghost btn-sm" onClick={load} disabled={loading || saving}>
                  重新加载
                </button>
                <span className="ml-auto hidden text-[11px] text-base-content/40 lg:inline">
                  配置保存至 <code className="font-mono">config/local/app.settings.json</code>
                </span>
              </div>

              <p className="text-xs leading-relaxed text-base-content/40">
                变更分块 / Embedding / Milvus 索引结构时会<strong>自动后台重处理</strong>全部文档；
                去重策略变更立即生效，无需重处理。
              </p>
            </form>
          )}
        </>
      )}
    </div>
  )
}
