import { FormEvent, KeyboardEvent, useEffect, useRef } from 'react'

type Props = {
  value: string
  onChange: (v: string) => void
  onSubmit: () => void
  disabled?: boolean
  busy?: boolean
  connected?: boolean
  modelName?: string
  className?: string
}

function SearchIcon({ className }: { className?: string }) {
  return (
    <svg
      xmlns="http://www.w3.org/2000/svg"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round"
      className={className}
    >
      <circle cx="11" cy="11" r="7" />
      <path d="m21 21-4.35-4.35" />
    </svg>
  )
}

function ArrowUpIcon() {
  return (
    <svg
      xmlns="http://www.w3.org/2000/svg"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="2.5"
      strokeLinecap="round"
      strokeLinejoin="round"
      className="h-4 w-4"
    >
      <path d="M12 19V5" />
      <path d="m5 12 7-7 7 7" />
    </svg>
  )
}

export default function ChatComposer({
  value,
  onChange,
  onSubmit,
  disabled,
  busy,
  connected,
  modelName,
  className = '',
}: Props) {
  const textareaRef = useRef<HTMLTextAreaElement>(null)

  useEffect(() => {
    const el = textareaRef.current
    if (!el) return
    el.style.height = 'auto'
    el.style.height = `${Math.min(el.scrollHeight, 200)}px`
  }, [value])

  const handleKeyDown = (e: KeyboardEvent<HTMLTextAreaElement>) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault()
      if (!disabled && value.trim()) onSubmit()
    }
  }

  const handleSubmit = (e: FormEvent) => {
    e.preventDefault()
    if (!disabled && value.trim()) onSubmit()
  }

  const canSend = connected && !disabled && value.trim().length > 0

  return (
    <form onSubmit={handleSubmit} className={`w-full ${className}`}>
      <div className="composer-card overflow-hidden">
        <textarea
          ref={textareaRef}
          rows={3}
          className="w-full resize-none bg-transparent px-5 pb-2 pt-4 text-[15px] leading-relaxed outline-none placeholder:text-base-content/35 disabled:cursor-not-allowed"
          placeholder={
            connected
              ? busy
                ? '正在生成回答…'
                : '在这里输入内容，基于知识库探索答案'
              : '等待连接服务…'
          }
          value={value}
          onChange={(e) => onChange(e.target.value)}
          onKeyDown={handleKeyDown}
          disabled={!connected || disabled}
        />

        <div className="flex items-center justify-between gap-3 border-t border-base-300/60 px-4 py-2.5">
          <div className="flex min-w-0 flex-wrap items-center gap-2">
            <span className="inline-flex items-center gap-1.5 rounded-lg border border-primary/25 bg-primary/[0.06] px-2.5 py-1 text-xs font-medium text-primary">
              <SearchIcon className="h-3.5 w-3.5" />
              知识库检索
            </span>
            {modelName && (
              <span className="hidden truncate text-xs text-base-content/40 sm:inline">
                {modelName}
              </span>
            )}
          </div>

          <div className="flex shrink-0 items-center gap-3">
            <span className="hidden items-center gap-1.5 text-xs text-base-content/35 md:flex">
              <span
                className={`inline-block h-1.5 w-1.5 rounded-full ${connected ? 'bg-success' : 'bg-warning'}`}
              />
              {connected ? '已连接' : '连接中'}
            </span>
            <button
              type="submit"
              disabled={!canSend}
              title="发送 (Enter)"
              className={`flex h-8 w-8 items-center justify-center rounded-lg transition-all ${
                canSend
                  ? 'bg-primary text-primary-content shadow-sm hover:bg-primary/90'
                  : 'bg-base-300 text-base-content/30'
              }`}
            >
              <ArrowUpIcon />
            </button>
          </div>
        </div>
      </div>
      <p className="mt-2 text-center text-[11px] text-base-content/30">
        Enter 发送 · Shift+Enter 换行 · 内容由 AI 生成，请结合引用来源核实
      </p>
    </form>
  )
}
