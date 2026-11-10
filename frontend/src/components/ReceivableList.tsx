import { CalendarClock, Check, Pencil, Trash } from 'lucide-react'
import type { Receivable } from '../types'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { cn } from '@/lib/utils'
import { dayOf, dueText, dueTone, money } from './receivableMeta'

/**
 * 待收款清单。回款页和项目详情的「收款计划」tab 共用一份。
 * 行上直接勾已收 / 撤回，点内容进下一级编辑（备注、金额、到期时间）。
 */
export default function ReceivableList({
  items,
  showProject = true,
  onEdit,
  onToggleDone,
  onDelete,
  onOpenProject
}: {
  items: Receivable[]
  showProject?: boolean
  onEdit: (item: Receivable) => void
  onToggleDone: (item: Receivable) => void
  onDelete: (item: Receivable) => void
  onOpenProject?: (projectId: number) => void
}) {
  if (items.length === 0) {
    return (
      <div className="flex flex-col items-center gap-1.5 rounded-lg border border-dashed border-border px-4 py-10 text-center">
        <CalendarClock className="size-5 text-muted-foreground/60" />
        <div className="text-xs text-muted-foreground">这里还是空的。记一笔待收，到期我来提醒你。</div>
      </div>
    )
  }

  return (
    <ul className="divide-y divide-border overflow-hidden rounded-lg border border-border">
      {items.map(item => {
        const done = item.status === 'done'
        return (
          <li
            key={item.id}
            className={cn('group flex items-center gap-3 px-3.5 py-2.5 transition-colors hover:bg-secondary/40', done && 'bg-secondary/20')}
          >
            <button
              type="button"
              onClick={() => onToggleDone(item)}
              title={done ? '撤回为未收' : '标记已收'}
              className={cn(
                'flex size-4.5 shrink-0 items-center justify-center rounded-full border transition-colors',
                done
                  ? 'border-emerald-500 bg-emerald-500 text-white'
                  : 'border-border text-transparent hover:border-emerald-500 hover:text-emerald-500'
              )}
            >
              <Check className="size-3" />
            </button>

            <button type="button" onClick={() => onEdit(item)} className="min-w-0 flex-1 text-left">
              <div className="flex flex-wrap items-center gap-1.5">
                <span className={cn('truncate text-[13px] font-medium', done && 'text-muted-foreground line-through')}>
                  {item.title}
                </span>
                {!done && item.overdue && <Badge variant="destructive">逾期</Badge>}
                {done && <Badge variant="success">已收</Badge>}
                {showProject && item.project_name && (
                  <span
                    role="button"
                    tabIndex={0}
                    onClick={e => {
                      e.stopPropagation()
                      if (item.project_id) onOpenProject?.(item.project_id)
                    }}
                    onKeyDown={e => e.key === 'Enter' && item.project_id && onOpenProject?.(item.project_id)}
                    className="truncate text-[11px] text-primary hover:underline"
                  >
                    {item.project_name}
                  </span>
                )}
              </div>
              <div className="mt-0.5 flex items-center gap-2 text-[11px] text-muted-foreground">
                <span className={cn('tabular', dueTone(item))}>
                  {done
                    ? `到期 ${item.due_date || '—'} · 收到 ${dayOf(item.done_at) || '—'}`
                    : item.due_date
                      ? `${item.due_date} · ${dueText(item)}`
                      : dueText(item)}
                </span>
                {item.note && <span className="truncate">备注：{item.note}</span>}
              </div>
            </button>

            <span className={cn('tabular shrink-0 text-[13px] font-semibold', done ? 'text-muted-foreground' : 'text-foreground')}>
              {money(item.amount)}
            </span>

            <div className="flex shrink-0 gap-0.5 opacity-0 transition-opacity group-hover:opacity-100">
              <Button variant="ghost" size="icon-sm" onClick={() => onEdit(item)} title="编辑">
                <Pencil />
              </Button>
              <Button
                variant="ghost"
                size="icon-sm"
                className="text-muted-foreground hover:bg-red-50 hover:text-red-600"
                onClick={() => onDelete(item)}
                title="删除"
              >
                <Trash />
              </Button>
            </div>
          </li>
        )
      })}
    </ul>
  )
}
