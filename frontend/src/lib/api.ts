const API_BASE = '/api/v1'

export type ServiceStatus = 'up' | 'down' | 'degraded'

export interface ServiceCheck {
  name: string
  label: string
  category: string
  status: ServiceStatus
  endpoint: string
  latency_ms: number
  message: string
}

export interface SystemMetrics {
  documents_total: number
  chunks_total: number
  collections_total: number
  vector_entities: number
  minio_objects: number
  minio_bytes: number
  redis_keys: number
  redis_memory_mb: number
  postgres_size_mb: number
}

export interface SystemStatusReport {
  checked_at: string
  overall: ServiceStatus
  source?: string
  monitor_url?: string
  host?: string
  services: ServiceCheck[]
  metrics: SystemMetrics
}

export interface DocumentListItem {
  id: string
  title: string
  source_type: string
  source_uri?: string
  status: string
  chunk_count: number
  version: number
  content_preview: string
  created_at: string
  updated_at: string
}

export interface PageSnapshot {
  page: number
  text: string
}

export interface Document {
  id: string
  collection_id: string
  title: string
  source_type: string
  source_uri?: string
  content_hash?: string
  content?: string
  content_pages?: PageSnapshot[]
  version: number
  status: string
  error_message?: string
  chunk_count: number
  metadata?: Record<string, unknown>
  created_at: string
  updated_at: string
  indexed_at?: string | null
}

export interface DocumentListResponse {
  collection_id: string
  documents: DocumentListItem[]
  limit: number
  offset: number
}

export interface Collection {
  id: string
  name: string
  description?: string
  embedding_model: string
  milvus_collection: string
  created_at: string
  updated_at: string
}

export interface CollectionListResponse {
  collections: Collection[]
}

export const DEFAULT_COLLECTION_ID = '00000000-0000-0000-0000-000000000001'

export function collectionLabel(c: Collection): string {
  switch (c.name) {
    case 'default':
      return '默认知识库'
    case 'eval-cqadupstack-unix':
      return '评测 · CQADupStack Unix'
    default:
      return c.description?.trim() || c.name
  }
}

export interface ImportDocumentResponse {
  document_id: string
  title: string
  status: string
  version: number
  chunk_count: number
  message: string
  vectors_stored?: boolean
}

export interface BatchImportItem {
  filename: string
  ok: boolean
  document_id?: string
  title?: string
  status?: string
  chunk_count?: number
  vectors_stored?: boolean
  outcome?: string
  message?: string
  error?: string
}

export interface BatchImportResponse {
  total: number
  succeeded: number
  failed: number
  message: string
  items: BatchImportItem[]
}

export interface Chunk {
  id: string
  document_id: string
  document_version: number
  chunk_index: number
  chunk_hash: string
  content: string
  token_count: number
  page?: number | null
  section?: string
  status: string
  metadata?: Record<string, unknown>
  created_at: string
}

export interface DocumentDetailResponse {
  document: Document
  chunks: Chunk[]
}

export interface RechunkResponse {
  document_id: string
  title: string
  status: string
  version: number
  chunk_count: number
  message: string
  chunks: Chunk[]
}

export interface AppSettings {
  monitor_url: string
  embedding_provider: string
  embedding_local_backend: string
  embedding_api_url: string
  embedding_model: string
  embedding_dim: number
  embedding_batch_size: number
  llm_provider: string
  llm_local_backend: string
  llm_api_url: string
  llm_model: string
  llm_temperature: number
  llm_max_tokens: number
  chunk_max_chars: number
  chunk_overlap: number
  search_top_k: number
  search_score_threshold: number
  search_hybrid_enabled: boolean
  search_recall_k: number
  search_rerank_enabled: boolean
  rerank_api_url: string
  rerank_model: string
  milvus_index_type: string
  milvus_metric: string
  milvus_nlist: number
  milvus_nprobe: number
  milvus_hnsw_m: number
  milvus_hnsw_ef_construction: number
  milvus_hnsw_ef: number
  milvus_sparse_drop_ratio_build: number
  milvus_sparse_drop_ratio_search: number
  document_dedup_enabled: boolean
  document_dedup_mode: string
  document_dedup_by_content_hash: boolean
  document_dedup_by_source_uri: boolean
  chunk_dedup_enabled: boolean
  chunk_dedup_scope: string
  embedding_api_key_set: boolean
  llm_api_key_set: boolean
  settings_path: string
  embedding_ready: boolean
  embedding_status: string
  reindex: ReindexStatus
}

export interface ReindexStatus {
  running: boolean
  total: number
  done: number
  failed: number
  last_error?: string
  message?: string
}

export type AppSettingsUpdate = Partial<
  Omit<
    AppSettings,
    | 'embedding_api_key_set'
    | 'llm_api_key_set'
    | 'settings_path'
    | 'embedding_ready'
    | 'embedding_status'
    | 'reindex'
  >
> & {
  embedding_api_key?: string
  llm_api_key?: string
}

export interface ReindexPlan {
  needed: boolean
  rechunk_all: boolean
  reembed_all: boolean
  recreate_collection: boolean
  reasons: string[]
}

export interface UpdateSettingsResponse {
  message: string
  settings: AppSettings
  reload_error?: string
  reindex_plan?: ReindexPlan
  reindex_started?: boolean
}

async function parseError(res: Response): Promise<never> {
  const text = await res.text()
  try {
    const json = JSON.parse(text) as { error?: string }
    throw new Error(json.error ?? text)
  } catch (e) {
    if (e instanceof Error && e.message !== text) throw e
    throw new Error(text)
  }
}

export async function search(query: string, topK = 5): Promise<SearchResponse> {
  const res = await fetch(`${API_BASE}/search`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ query, top_k: topK }),
  })
  if (!res.ok) await parseError(res)
  return res.json()
}

export interface SearchResultItem {
  chunk_id: string
  document_id: string
  document_title: string
  content: string
  score: number
  page?: number | null
  section?: string
}

export interface SearchResponse {
  query: string
  top_k: number
  collection: string
  count: number
  results: SearchResultItem[]
  message?: string
}

export interface ConversationListItem {
  id: string
  title: string
  status: string
  message_count: number
  last_preview: string
  created_at: string
  updated_at: string
}

export interface ChatMessage {
  id: string
  conversation_id: string
  role: 'user' | 'assistant' | 'system'
  content: string
  sources?: ChatSource[]
  metadata?: Record<string, unknown>
  created_at: string
}

export interface Conversation {
  id: string
  collection_id: string
  title: string
  status: string
  metadata?: Record<string, unknown>
  created_at: string
  updated_at: string
}

export interface ConversationListResponse {
  collection_id: string
  conversations: ConversationListItem[]
  limit: number
  offset: number
}

export interface ConversationDetailResponse {
  conversation: Conversation
  messages: ChatMessage[]
}

export type ChatSource = {
  chunk_id: string
  document_id: string
  title: string
  content: string
  score: number
  page?: number
}

export async function listConversations(limit = 50, offset = 0): Promise<ConversationListResponse> {
  const res = await fetch(`${API_BASE}/conversations?limit=${limit}&offset=${offset}`)
  if (!res.ok) await parseError(res)
  return res.json()
}

export async function createConversation(title = ''): Promise<{ conversation: Conversation }> {
  const res = await fetch(`${API_BASE}/conversations`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ title }),
  })
  if (!res.ok) await parseError(res)
  return res.json()
}

export async function getConversation(id: string): Promise<ConversationDetailResponse> {
  const res = await fetch(`${API_BASE}/conversations/${id}`)
  if (!res.ok) await parseError(res)
  return res.json()
}

export async function updateConversation(
  id: string,
  payload: { title?: string; status?: string },
): Promise<{ conversation: Conversation }> {
  const res = await fetch(`${API_BASE}/conversations/${id}`, {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload),
  })
  if (!res.ok) await parseError(res)
  return res.json()
}

export async function deleteConversation(id: string): Promise<{ message: string; conversation_id: string }> {
  const res = await fetch(`${API_BASE}/conversations/${id}`, { method: 'DELETE' })
  if (!res.ok) await parseError(res)
  return res.json()
}

export interface ImportJobItem {
  filename: string
  status: string
  outcome?: string
  document_id?: string
  title?: string
  chunk_count?: number
  vectors_stored?: boolean
  error?: string
}

export interface ImportJob {
  id: string
  status: string
  total: number
  done: number
  failed: number
  progress: number
  items: ImportJobItem[]
  message?: string
  created_at: string
  updated_at: string
  completed_at?: string | null
}

export async function createImportJob(files: File[]): Promise<{ job: ImportJob }> {
  const form = new FormData()
  for (const file of files) form.append('files', file)
  const res = await fetch(`${API_BASE}/import/jobs`, { method: 'POST', body: form })
  if (!res.ok) await parseError(res)
  return res.json()
}

export async function getImportJob(id: string): Promise<{ job: ImportJob }> {
  const res = await fetch(`${API_BASE}/import/jobs/${id}`)
  if (!res.ok) await parseError(res)
  return res.json()
}

export async function listImportJobs(limit = 20): Promise<{ jobs: ImportJob[] }> {
  const res = await fetch(`${API_BASE}/import/jobs?limit=${limit}`)
  if (!res.ok) await parseError(res)
  return res.json()
}

export async function healthCheck() {
  const res = await fetch('/healthz')
  return res.json()
}

export async function fetchSystemStatus(): Promise<SystemStatusReport> {
  const res = await fetch(`${API_BASE}/system/status`)
  if (!res.ok) await parseError(res)
  return res.json()
}

export async function deleteDocument(id: string): Promise<{ message: string; document_id: string }> {
  const res = await fetch(`${API_BASE}/documents/${id}`, { method: 'DELETE' })
  if (!res.ok) await parseError(res)
  return res.json()
}

export async function fetchSettings(): Promise<AppSettings> {
  const res = await fetch(`${API_BASE}/settings`)
  if (!res.ok) await parseError(res)
  return res.json()
}

export async function updateSettings(payload: AppSettingsUpdate): Promise<UpdateSettingsResponse> {
  const res = await fetch(`${API_BASE}/settings`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload),
  })
  if (!res.ok) await parseError(res)
  return res.json()
}

export async function listCollections(): Promise<CollectionListResponse> {
  const res = await fetch(`${API_BASE}/collections`)
  if (!res.ok) await parseError(res)
  return res.json()
}

export async function listDocuments(
  limit = 50,
  offset = 0,
  collectionId = DEFAULT_COLLECTION_ID,
): Promise<DocumentListResponse> {
  const params = new URLSearchParams({
    limit: String(limit),
    offset: String(offset),
    collection_id: collectionId,
  })
  const res = await fetch(`${API_BASE}/documents?${params}`)
  if (!res.ok) await parseError(res)
  return res.json()
}

export async function getDocument(id: string): Promise<DocumentDetailResponse> {
  const res = await fetch(`${API_BASE}/documents/${id}`)
  if (!res.ok) await parseError(res)
  return res.json()
}

export async function rechunkDocument(
  id: string,
  async = false,
): Promise<RechunkResponse | AsyncTaskResponse> {
  const res = await fetch(`${API_BASE}/documents/${id}/rechunk${async ? '?async=true' : ''}`, { method: 'POST' })
  if (!res.ok) await parseError(res)
  return res.json()
}

export async function reimportDocument(id: string, file: File, sourceType?: string): Promise<AsyncTaskResponse> {
  const form = new FormData()
  form.append('file', file)
  if (sourceType?.trim()) form.append('source_type', sourceType.trim())

  const res = await fetch(`${API_BASE}/documents/${id}/reimport`, { method: 'POST', body: form })
  if (!res.ok) await parseError(res)
  return res.json()
}

export interface AsyncTaskResponse {
  message: string
  document_id: string
  async: boolean
}

export async function importDocumentFile(file: File, title?: string): Promise<ImportDocumentResponse> {
  const form = new FormData()
  form.append('file', file)
  if (title?.trim()) form.append('title', title.trim())

  const res = await fetch(`${API_BASE}/documents`, { method: 'POST', body: form })
  if (!res.ok) await parseError(res)
  return res.json()
}

export async function importDocumentFiles(files: File[]): Promise<BatchImportResponse> {
  const form = new FormData()
  for (const file of files) {
    form.append('files', file)
  }

  const res = await fetch(`${API_BASE}/documents/batch`, { method: 'POST', body: form })
  const data = (await res.json()) as BatchImportResponse & { error?: string }
  if (!res.ok && res.status !== 207) {
    throw new Error(data.error ?? `batch import failed (${res.status})`)
  }
  return data
}

export async function importDocumentText(
  title: string,
  content: string,
  sourceType = 'markdown',
): Promise<ImportDocumentResponse> {
  const res = await fetch(`${API_BASE}/documents`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ title, content, source_type: sourceType }),
  })
  if (!res.ok) await parseError(res)
  return res.json()
}

function formatBytes(n: number) {
  if (n < 1024) return `${n} B`
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`
  if (n < 1024 * 1024 * 1024) return `${(n / 1024 / 1024).toFixed(1)} MB`
  return `${(n / 1024 / 1024 / 1024).toFixed(2)} GB`
}

export { formatBytes }
