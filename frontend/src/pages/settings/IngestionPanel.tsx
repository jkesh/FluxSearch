import type { AppSettings } from '../../lib/api'
import { Field, Section, Toggle } from './shared'

const DOCUMENT_DEDUP_MODES = [
  { value: 'skip', label: '跳过重复（默认）' },
  { value: 'replace', label: '覆盖更新（同名文件内容变更时）' },
]

const CHUNK_DEDUP_SCOPES = [
  { value: 'collection', label: '整个知识库（跨文档）' },
  { value: 'document', label: '当前文档内' },
]

type Props = {
  form: AppSettings
  patch: <K extends keyof AppSettings>(key: K, value: AppSettings[K]) => void
}

export default function IngestionPanel({ form, patch }: Props) {
  return (
    <div className="space-y-5">
      <Section title="文档分块" desc="影响新导入与重新分块">
        <Field label="最大 token 数">
          <input
            type="number"
            className="input input-bordered input-sm w-full"
            value={form.chunk_max_tokens}
            min={64}
            onChange={(e) => patch('chunk_max_tokens', Number(e.target.value))}
          />
        </Field>
        <Field label="重叠 token 数">
          <input
            type="number"
            className="input input-bordered input-sm w-full"
            value={form.chunk_overlap_tokens}
            min={0}
            onChange={(e) => patch('chunk_overlap_tokens', Number(e.target.value))}
          />
        </Field>
      </Section>

      <Section title="文档去重（分块前）" desc="导入时检测整篇文档是否已存在，避免重复入库与重复 Embedding">
        <Toggle
          checked={form.document_dedup_enabled}
          onChange={(v) => patch('document_dedup_enabled', v)}
          label="启用文档去重"
          hint="关闭后每次导入都会新建文档"
        />

        {form.document_dedup_enabled && (
          <>
            <Field label="重复时行为">
              <select
                className="select select-bordered select-sm w-full"
                value={form.document_dedup_mode}
                onChange={(e) => patch('document_dedup_mode', e.target.value)}
              >
                {DOCUMENT_DEDUP_MODES.map((o) => (
                  <option key={o.value} value={o.value}>
                    {o.label}
                  </option>
                ))}
              </select>
            </Field>

            <Toggle
              checked={form.document_dedup_by_content_hash}
              onChange={(v) => patch('document_dedup_by_content_hash', v)}
              label="按内容哈希"
              hint="正文完全相同则视为重复"
            />

            <Toggle
              checked={form.document_dedup_by_source_uri}
              onChange={(v) => patch('document_dedup_by_source_uri', v)}
              label="按文件名 / 来源路径"
              hint="同一文件名再次导入时，覆盖模式下会更新已有文档"
            />
          </>
        )}
      </Section>

      <Section title="片段去重（分块后）" desc="跳过已存在的 chunk，不再重复 Embedding；适用于多文档共享相同段落">
        <Toggle
          checked={form.chunk_dedup_enabled}
          onChange={(v) => patch('chunk_dedup_enabled', v)}
          label="启用片段去重"
          hint="默认关闭；开启后仅写入新片段"
        />

        {form.chunk_dedup_enabled && (
          <Field label="去重范围">
            <select
              className="select select-bordered select-sm w-full"
              value={form.chunk_dedup_scope}
              onChange={(e) => patch('chunk_dedup_scope', e.target.value)}
            >
              {CHUNK_DEDUP_SCOPES.map((o) => (
                <option key={o.value} value={o.value}>
                  {o.label}
                </option>
              ))}
            </select>
          </Field>
        )}
      </Section>
    </div>
  )
}
