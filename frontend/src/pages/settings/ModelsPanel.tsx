import type { AppSettings } from '../../lib/api'
import { Field, Section } from './shared'

const EMBEDDING_PROVIDERS = [
  { value: '', label: '禁用（仅分块）' },
  { value: 'bailian', label: '阿里云百炼' },
  { value: 'local', label: '本地（Ollama / llama.cpp）' },
  { value: 'ollama', label: 'Ollama（直连）' },
  { value: 'llamacpp', label: 'llama-server（OpenAI 兼容）' },
]

const LOCAL_BACKENDS = [
  { value: 'ollama', label: 'Ollama' },
  { value: 'llamacpp', label: 'llama.cpp server' },
]

const LLM_PROVIDERS = [
  { value: '', label: '未配置' },
  { value: 'bailian', label: '阿里云百炼' },
  { value: 'local', label: '本地（Ollama / llama.cpp）' },
  { value: 'ollama', label: 'Ollama（直连）' },
  { value: 'llamacpp', label: 'llama-server' },
]

type Props = {
  form: AppSettings
  embeddingKey: string
  llmKey: string
  setEmbeddingKey: (v: string) => void
  setLlmKey: (v: string) => void
  patch: <K extends keyof AppSettings>(key: K, value: AppSettings[K]) => void
}

export default function ModelsPanel({ form, embeddingKey, llmKey, setEmbeddingKey, setLlmKey, patch }: Props) {
  const showLocalEmbedding = form.embedding_provider === 'local'
  const showLocalLLM = form.llm_provider === 'local'

  return (
    <div className="space-y-5">
      <Section title="Embedding 向量化" desc="百炼 OpenAI 兼容 API，或本地 Ollama / llama-server">
        <Field label="Provider">
          <select
            className="select select-bordered select-sm w-full"
            value={form.embedding_provider}
            onChange={(e) => patch('embedding_provider', e.target.value)}
          >
            {EMBEDDING_PROVIDERS.map((o) => (
              <option key={o.value} value={o.value}>
                {o.label}
              </option>
            ))}
          </select>
        </Field>

        {showLocalEmbedding && (
          <Field label="本地后端">
            <select
              className="select select-bordered select-sm w-full"
              value={form.embedding_local_backend}
              onChange={(e) => patch('embedding_local_backend', e.target.value)}
            >
              {LOCAL_BACKENDS.map((o) => (
                <option key={o.value} value={o.value}>
                  {o.label}
                </option>
              ))}
            </select>
          </Field>
        )}

        <Field label="API 地址" hint="百炼: https://dashscope.aliyuncs.com/compatible-mode/v1">
          <input
            className="input input-bordered input-sm w-full font-mono"
            value={form.embedding_api_url}
            onChange={(e) => patch('embedding_api_url', e.target.value)}
            placeholder="http://127.0.0.1:11434"
          />
        </Field>

        <Field label="API Key" hint={form.embedding_api_key_set ? '已配置，留空则不修改' : '百炼必填'}>
          <input
            type="password"
            className="input input-bordered input-sm w-full font-mono"
            value={embeddingKey}
            onChange={(e) => setEmbeddingKey(e.target.value)}
            placeholder={form.embedding_api_key_set ? '********' : 'sk-...'}
            autoComplete="off"
          />
        </Field>

        <Field label="模型">
          <input
            className="input input-bordered input-sm w-full"
            value={form.embedding_model}
            onChange={(e) => patch('embedding_model', e.target.value)}
            placeholder="bge-m3 / text-embedding-v3"
          />
        </Field>

        <Field label="向量维度">
          <input
            type="number"
            className="input input-bordered input-sm w-full"
            value={form.embedding_dim}
            min={1}
            onChange={(e) => patch('embedding_dim', Number(e.target.value))}
          />
        </Field>

        <Field label="批大小">
          <input
            type="number"
            className="input input-bordered input-sm w-full"
            value={form.embedding_batch_size}
            min={1}
            onChange={(e) => patch('embedding_batch_size', Number(e.target.value))}
          />
        </Field>
      </Section>

      <Section title="LLM 对话" desc="RAG 对话用，百炼或本地 llama.cpp（后续接入 WebSocket）">
        <Field label="Provider">
          <select
            className="select select-bordered select-sm w-full"
            value={form.llm_provider}
            onChange={(e) => patch('llm_provider', e.target.value)}
          >
            {LLM_PROVIDERS.map((o) => (
              <option key={o.value} value={o.value}>
                {o.label}
              </option>
            ))}
          </select>
        </Field>

        {showLocalLLM && (
          <Field label="本地后端">
            <select
              className="select select-bordered select-sm w-full"
              value={form.llm_local_backend}
              onChange={(e) => patch('llm_local_backend', e.target.value)}
            >
              {LOCAL_BACKENDS.map((o) => (
                <option key={o.value} value={o.value}>
                  {o.label}
                </option>
              ))}
            </select>
          </Field>
        )}

        <Field label="API 地址">
          <input
            className="input input-bordered input-sm w-full font-mono"
            value={form.llm_api_url}
            onChange={(e) => patch('llm_api_url', e.target.value)}
            placeholder="https://dashscope.aliyuncs.com/compatible-mode/v1"
          />
        </Field>

        <Field label="API Key" hint={form.llm_api_key_set ? '已配置，留空则不修改' : ''}>
          <input
            type="password"
            className="input input-bordered input-sm w-full font-mono"
            value={llmKey}
            onChange={(e) => setLlmKey(e.target.value)}
            placeholder={form.llm_api_key_set ? '********' : 'sk-...'}
            autoComplete="off"
          />
        </Field>

        <Field label="模型">
          <input
            className="input input-bordered input-sm w-full"
            value={form.llm_model}
            onChange={(e) => patch('llm_model', e.target.value)}
            placeholder="qwen-plus / llama3"
          />
        </Field>

        <Field label="Temperature">
          <input
            type="number"
            step="0.1"
            min={0}
            max={2}
            className="input input-bordered input-sm w-full"
            value={form.llm_temperature}
            onChange={(e) => patch('llm_temperature', Number(e.target.value))}
          />
        </Field>

        <Field label="Max Tokens">
          <input
            type="number"
            className="input input-bordered input-sm w-full"
            value={form.llm_max_tokens}
            min={1}
            onChange={(e) => patch('llm_max_tokens', Number(e.target.value))}
          />
        </Field>
      </Section>
    </div>
  )
}
