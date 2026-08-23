import { Link } from 'react-router-dom'
import type { ConversationListItem } from '../../lib/api'

type Props = {
  conversations: ConversationListItem[]
  activeId?: string
  onNewChat: () => void
  onDelete: (id: string, e: React.MouseEvent) => void
}

function PlusIcon() {
  return (
    <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" className="h-4 w-4">
      <path d="M12 5v14" /><path d="M5 12h14" />
    </svg>
  )
}

function TrashIcon() {
  return (
    <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" className="h-3.5 w-3.5">
      <path d="M3 6h18" /><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6" /><path d="M8 6V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2" />
    </svg>
  )
}

function ChatBubbleIcon() {
  return (
    <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" className="h-4 w-4 shrink-0 opacity-50">
      <path d="M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z" />
    </svg>
  )
}

export default function HistorySidebar({ conversations, activeId, onNewChat, onDelete }: Props) {
  return (
    <aside className="flex w-56 shrink-0 flex-col border-r border-base-300/80 bg-base-100 xl:w-64">
      <div className="flex items-center justify-between px-4 py-3.5">
        <span className="text-sm font-medium text-base-content/80">对话记录</span>
        <button
          type="button"
          onClick={onNewChat}
          className="flex h-7 w-7 items-center justify-center rounded-lg text-base-content/50 transition-colors hover:bg-base-200 hover:text-primary"
          title="新对话"
        >
          <PlusIcon />
        </button>
      </div>

      <div className="flex-1 overflow-y-auto px-2 pb-3">
        {conversations.length === 0 ? (
          <p className="px-3 py-6 text-center text-xs leading-relaxed text-base-content/35">
            暂无历史对话
            <br />
            发送消息后将自动保存
          </p>
        ) : (
          <ul className="space-y-0.5">
            {conversations.map((c) => {
              const active = activeId === c.id
              return (
                <li key={c.id}>
                  <Link
                    to={`/c/${c.id}`}
                    className={`group flex items-center gap-2 rounded-lg px-3 py-2.5 text-sm transition-colors ${
                      active
                        ? 'nav-item-active'
                        : 'text-base-content/65 hover:bg-base-200/80'
                    }`}
                  >
                    <ChatBubbleIcon />
                    <div className="min-w-0 flex-1">
                      <div className="truncate text-[13px]">
                        {c.title || c.last_preview || '新对话'}
                      </div>
                    </div>
                    <button
                      type="button"
                      onClick={(e) => onDelete(c.id, e)}
                      className="flex h-6 w-6 shrink-0 items-center justify-center rounded-md text-base-content/30 opacity-0 transition-all hover:bg-base-300 hover:text-error group-hover:opacity-100"
                      title="删除"
                    >
                      <TrashIcon />
                    </button>
                  </Link>
                </li>
              )
            })}
          </ul>
        )}
      </div>
    </aside>
  )
}
