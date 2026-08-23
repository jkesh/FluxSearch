import { useSystemStatus } from '../hooks/useSystemStatus'
import { formatBytes, type ServiceCheck, type ServiceStatus } from '../lib/api'

const CATEGORY_LABELS: Record<string, string> = {
  application: '应用服务',
  database: '数据库',
  cache: '缓存',
  storage: '对象存储',
  vector: '向量检索',
  metadata: '元数据',
}

const STATUS_LABEL: Record<ServiceStatus, string> = {
  up: '正常',
  down: '离线',
  degraded: '异常',
}

const STATUS_DOT: Record<ServiceStatus, string> = {
  up: 'bg-success',
  down: 'bg-error',
  degraded: 'bg-warning',
}

const STATUS_TEXT: Record<ServiceStatus, string> = {
  up: 'text-success',
  down: 'text-error',
  degraded: 'text-warning',
}

const SOURCE_LABEL: Record<string, string> = {
  remote: 'HTTP 远程 Monitor',
  local: '本地直连',
  monitor: 'Monitor 服务',
  'remote-failed': '远程连接失败',
}

function StatusDot({ status }: { status: ServiceStatus }) {
  return (
    <span className="relative flex h-2 w-2 shrink-0">
      {status === 'up' && (
        <span className="absolute inline-flex h-full w-full animate-ping rounded-full bg-success opacity-50" />
      )}
      <span className={`relative inline-flex h-2 w-2 rounded-full ${STATUS_DOT[status]}`} />
    </span>
  )
}

function ServiceCard({ service }: { service: ServiceCheck }) {
  return (
    <div className="surface p-4 transition-shadow hover:shadow-md">
      <div className="flex items-center justify-between gap-2">
        <h3 className="truncate text-sm font-medium">{service.label}</h3>
        <span className="flex shrink-0 items-center gap-1.5 text-xs">
          <StatusDot status={service.status} />
          <span className={STATUS_TEXT[service.status]}>{STATUS_LABEL[service.status]}</span>
        </span>
      </div>
      <p className="mt-2 break-all font-mono text-xs text-base-content/50">{service.endpoint}</p>
      <div className="mt-2 flex items-center justify-between gap-2 text-xs">
        <span className="truncate text-base-content/70">{service.message}</span>
        <span className="shrink-0 tabular-nums text-base-content/40">{service.latency_ms} ms</span>
      </div>
    </div>
  )
}

function MetricCard({ label, value, sub }: { label: string; value: string | number; sub?: string }) {
  return (
    <div className="surface p-4 transition-shadow hover:shadow-md">
      <div className="truncate text-xs font-medium text-base-content/50">{label}</div>
      <div className="mt-1.5 truncate text-xl font-bold tabular-nums tracking-tight">{value}</div>
      {sub && <div className="mt-1 truncate text-xs text-base-content/40">{sub}</div>}
    </div>
  )
}

function RefreshIcon() {
  return (
    <svg
      xmlns="http://www.w3.org/2000/svg"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round"
      className="h-3.5 w-3.5"
    >
      <path d="M21 12a9 9 0 1 1-2.64-6.36" />
      <path d="M21 3v6h-6" />
    </svg>
  )
}

export default function MonitorPanel() {
  const { report, loading, error, refresh, intervalSec } = useSystemStatus()

  const grouped =
    report?.services.reduce<Record<string, ServiceCheck[]>>((acc, svc) => {
      if (!acc[svc.category]) acc[svc.category] = []
      acc[svc.category].push(svc)
      return acc
    }, {}) ?? {}

  const m = report?.metrics

  return (
    <div className="space-y-5">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <p className="text-xs text-base-content/50">
          每 {intervalSec}s 自动刷新
          {report?.source && ` · ${SOURCE_LABEL[report.source] ?? report.source}`}
        </p>
        <button className="btn btn-sm btn-outline gap-1.5" onClick={refresh} disabled={loading}>
          {loading ? <span className="loading loading-spinner loading-xs" /> : <RefreshIcon />}
          刷新
        </button>
      </div>

      {error && (
        <div className="rounded-xl border border-error/30 bg-error/10 px-4 py-3 text-sm text-error">
          无法获取状态：{error}
        </div>
      )}

      {report && (
        <div className="surface flex flex-wrap items-center gap-x-4 gap-y-1.5 px-4 py-3">
          <span className="flex items-center gap-2 text-sm font-medium">
            <StatusDot status={report.overall} />
            整体状态：{STATUS_LABEL[report.overall]}
          </span>
          <span className="font-mono text-xs text-base-content/50">
            {new Date(report.checked_at).toLocaleString()}
            {report.host && ` · ${report.host}`}
            {report.monitor_url && ` · ${report.monitor_url}`}
          </span>
        </div>
      )}

      {m && (
        <section>
          <h2 className="mb-2 text-sm font-semibold">数据指标</h2>
          <div className="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-6">
            <MetricCard label="文档数量" value={m.documents_total} sub="documents 表" />
            <MetricCard label="Chunk 数量" value={m.chunks_total} sub="chunks 表" />
            <MetricCard label="向量实体" value={m.vector_entities} sub={`${m.collections_total} 个集合`} />
            <MetricCard label="MinIO 对象" value={m.minio_objects} sub={formatBytes(m.minio_bytes)} />
            <MetricCard label="Redis 键数" value={m.redis_keys} sub={`${m.redis_memory_mb.toFixed(1)} MB 内存`} />
            <MetricCard label="PostgreSQL" value={`${m.postgres_size_mb.toFixed(1)} MB`} sub="public 表空间" />
          </div>
        </section>
      )}

      {Object.entries(grouped).map(([category, services]) => (
        <section key={category}>
          <h2 className="mb-2 flex items-baseline gap-2 text-sm font-semibold">
            {CATEGORY_LABELS[category] ?? category}
            <span className="text-xs font-normal text-base-content/40">{services.length}</span>
          </h2>
          <div className="grid gap-3 sm:grid-cols-2">
            {services.map((svc) => (
              <ServiceCard key={svc.name} service={svc} />
            ))}
          </div>
        </section>
      ))}
    </div>
  )
}
