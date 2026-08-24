import type { AppSettings } from '../../lib/api'
import { Field, Section } from './shared'

const INDEX_TYPES = [
  { value: 'ivf_flat', label: 'IVF_FLAT（默认，均衡）' },
  { value: 'hnsw', label: 'HNSW（低延迟，高内存）' },
]

const METRICS = [
  { value: 'IP', label: 'IP 内积（归一化向量）' },
  { value: 'L2', label: 'L2 欧氏距离' },
  { value: 'COSINE', label: 'COSINE 余弦' },
]

type Props = {
  form: AppSettings
  patch: <K extends keyof AppSettings>(key: K, value: AppSettings[K]) => void
}

export default function SearchPanel({ form, patch }: Props) {
  const showIVF = form.milvus_index_type === 'ivf_flat'
  const showHNSW = form.milvus_index_type === 'hnsw'

  return (
    <div className="space-y-5">
      <Section
        title="Milvus 向量索引"
        desc="索引结构变更会自动重建集合并重处理文档；nprobe / ef / Top K 仅影响检索"
      >
        <Field label="索引类型" hint="变更后自动重建 Collection">
          <select
            className="select select-bordered select-sm w-full"
            value={form.milvus_index_type}
            onChange={(e) => patch('milvus_index_type', e.target.value)}
          >
            {INDEX_TYPES.map((o) => (
              <option key={o.value} value={o.value}>
                {o.label}
              </option>
            ))}
          </select>
        </Field>

        <Field label="距离度量" hint="需与 Embedding 模型一致">
          <select
            className="select select-bordered select-sm w-full"
            value={form.milvus_metric}
            onChange={(e) => patch('milvus_metric', e.target.value)}
          >
            {METRICS.map((o) => (
              <option key={o.value} value={o.value}>
                {o.label}
              </option>
            ))}
          </select>
        </Field>

        {showIVF && (
          <>
            <Field label="nlist（建索引）" hint="簇数量，变更需重建索引">
              <input
                type="number"
                className="input input-bordered input-sm w-full"
                value={form.milvus_nlist}
                min={1}
                onChange={(e) => patch('milvus_nlist', Number(e.target.value))}
              />
            </Field>
            <Field label="nprobe（检索）" hint="搜索簇数，越大召回越高、越慢">
              <input
                type="number"
                className="input input-bordered input-sm w-full"
                value={form.milvus_nprobe}
                min={1}
                onChange={(e) => patch('milvus_nprobe', Number(e.target.value))}
              />
            </Field>
          </>
        )}

        {showHNSW && (
          <>
            <Field label="HNSW M（建索引）">
              <input
                type="number"
                className="input input-bordered input-sm w-full"
                value={form.milvus_hnsw_m}
                min={4}
                onChange={(e) => patch('milvus_hnsw_m', Number(e.target.value))}
              />
            </Field>
            <Field label="efConstruction（建索引）">
              <input
                type="number"
                className="input input-bordered input-sm w-full"
                value={form.milvus_hnsw_ef_construction}
                min={8}
                onChange={(e) => patch('milvus_hnsw_ef_construction', Number(e.target.value))}
              />
            </Field>
            <Field label="ef（检索）" hint="搜索宽度，越大越准、越慢">
              <input
                type="number"
                className="input input-bordered input-sm w-full"
                value={form.milvus_hnsw_ef}
                min={8}
                onChange={(e) => patch('milvus_hnsw_ef', Number(e.target.value))}
              />
            </Field>
          </>
        )}
      </Section>

      <Section title="检索默认参数">
        <Field label="Hybrid 检索（Dense + Sparse）" hint="BGE-M3 专用；开启后需重建 Collection 并重嵌入">
          <input
            type="checkbox"
            className="toggle toggle-primary"
            checked={form.search_hybrid_enabled}
            onChange={(e) => patch('search_hybrid_enabled', e.target.checked)}
          />
        </Field>
        <Field label="Cross-Encoder 重排" hint="召回后用 bge-reranker 精排">
          <input
            type="checkbox"
            className="toggle toggle-primary"
            checked={form.search_rerank_enabled}
            onChange={(e) => patch('search_rerank_enabled', e.target.checked)}
          />
        </Field>
        <Field label="召回 Top K" hint="Hybrid/Rerank 前的候选数量">
          <input
            type="number"
            className="input input-bordered input-sm w-full"
            value={form.search_recall_k}
            min={form.search_top_k}
            max={200}
            onChange={(e) => patch('search_recall_k', Number(e.target.value))}
          />
        </Field>
        <Field label="默认 Top K">
          <input
            type="number"
            className="input input-bordered input-sm w-full"
            value={form.search_top_k}
            min={1}
            max={100}
            onChange={(e) => patch('search_top_k', Number(e.target.value))}
          />
        </Field>
        <Field label="分数阈值" hint="0 表示不过滤；IP 分数需按实际调参">
          <input
            type="number"
            step="0.01"
            className="input input-bordered input-sm w-full"
            value={form.search_score_threshold}
            min={0}
            onChange={(e) => patch('search_score_threshold', Number(e.target.value))}
          />
        </Field>
        {form.search_rerank_enabled && (
          <Field label="Reranker 模型">
            <input
              className="input input-bordered input-sm w-full"
              value={form.rerank_model}
              onChange={(e) => patch('rerank_model', e.target.value)}
            />
          </Field>
        )}
      </Section>
    </div>
  )
}
