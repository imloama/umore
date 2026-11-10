import { Check, Clock, FolderKanban, Pencil, Trash } from 'lucide-react'
import type { Task } from '@/types'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { cn } from '@/lib/utils'
import {
  CATEGORY_LABEL,
  CATEGORY_STYLE,
  PRIORITY_COLOR,
  PRIORITY_LABEL,
  STATUS_DOT,
  hoursPair
} from './taskMeta'

export default function TaskRow({
  task,
  onToggle,
  onOpen,
  onToday,
  onEdit,
  onRemove,
  showTodayAction = true
}: {
  task: Task
  onToggle: () => void
  onOpen: () => void
  onToday?: () => void
  onEdit?: () => void
  onRemove?: () => void
  showTodayAction?: boolean
}) {
  const done = task.status === 'DONE'
  const hours = hoursPair(task.actual_hours, task.estimated_hours)
  // 单条任务超了评估就标出来，别等汇总才发现
  const overrun =
    (task.actual_hours || 0) > (task.estimated_hours || 0) && (task.estimated_hours || 0) > 0
      ? Math.round(((task.actual_hours || 0) - (task.estimated_hours || 0)) * 100) / 100
      : 0

  return (
    <li className="group flex items-start gap-3 px-4 py-3 transition-colors hover:bg-secondary/45">
      <button
        type="button"
        onClick={onToggle}
        aria-label={done ? '标记未完成' : '标记完成'}
        className={cn(
          'mt-0.5 flex size-4 shrink-0 items-center justify-center rounded-full border transition-colors',
          done
            ? 'border-emerald-500 bg-emerald-500 text-white'
            : 'border-slate-300 bg-card hover:border-primary hover:text-primary'
        )}
      >
        {done && <Check className="size-3" strokeWidth={3} />}
      </button>

      <div className="min-w-0 flex-1">
        <div className="flex flex-wrap items-center gap-x-2 gap-y-1">
          <span className="tabular text-[11px] text-muted-foreground/70">#{task.number}</span>
          <button
            type="button"
            onClick={onOpen}
            className={cn(
              'max-w-full truncate text-left text-[13.5px] font-medium text-foreground hover:text-primary',
              done && 'text-muted-foreground line-through decoration-slate-300'
            )}
          >
            {task.title}
          </button>
          <Badge className={CATEGORY_STYLE[task.category].badge}>{CATEGORY_LABEL[task.category]}</Badge>
          <span className="inline-flex items-center gap-1 text-[11px]" style={{ color: PRIORITY_COLOR[task.priority] }}>
            <i className={cn('size-1.5 rounded-full', task.priority === 'MEDIUM' ? 'bg-amber-500' : 'bg-current')} />
            {PRIORITY_LABEL[task.priority]}
          </span>
          {task.is_today && <Badge variant="success">今日</Badge>}
        </div>

        <div className="mt-1 flex flex-wrap items-center gap-x-3 gap-y-1 text-[11px] text-muted-foreground">
          <span className="inline-flex items-center gap-1">
            <i className={cn('size-1.5 rounded-full', STATUS_DOT[task.status])} />
            {done ? '已完成' : task.status === 'DOING' ? '进行中' : task.status === 'BLOCKED' ? '阻塞' : '未开始'}
          </span>
          {task.project_name && (
            <span className="inline-flex items-center gap-1">
              <FolderKanban className="size-3" />
              {task.project_name}
            </span>
          )}
          {task.scheduled_date && <span className="tabular">计划 {task.scheduled_date}</span>}
          {task.due_date && <span className="tabular">截止 {task.due_date}</span>}
          {hours && (
            <span
              className={cn(
                'inline-flex items-center gap-1 tabular',
                overrun > 0 ? 'text-red-600' : 'text-amber-700'
              )}
            >
              <Clock className="size-3" />
              {hours}
              {overrun > 0 && <span className="font-medium">超 {overrun}h</span>}
            </span>
          )}
          {(task.labels || []).map(label => (
            <span
              key={label.id ?? label.name}
              className="rounded-md px-1.5 py-0.5 text-[10.5px] font-medium"
              style={{ background: `${label.color}14`, color: label.color }}
            >
              {label.name}
            </span>
          ))}
        </div>
      </div>

      <div className="flex shrink-0 items-center gap-1 opacity-0 transition-opacity focus-within:opacity-100 group-hover:opacity-100">
        {showTodayAction && onToday && (
          <Button variant="ghost" size="xs" onClick={onToday}>
            {task.is_today ? '移出' : '今日'}
          </Button>
        )}
        {onEdit && (
          <Button variant="ghost" size="icon-sm" onClick={onEdit} title="编辑">
            <Pencil />
          </Button>
        )}
        {onRemove && (
          <Button
            variant="ghost"
            size="icon-sm"
            onClick={onRemove}
            title="删除"
            className="text-muted-foreground hover:bg-red-50 hover:text-red-600"
          >
            <Trash />
          </Button>
        )}
      </div>
    </li>
  )
}
