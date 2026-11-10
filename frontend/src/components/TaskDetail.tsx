import { useEffect, useState } from 'react'
import { Timer } from 'lucide-react'
import { taskApi } from '../services/api'
import type { Task, TaskComment } from '../types'
import RichText from './RichText'
import RichTextEditor, { type QuickInsert } from './RichTextEditor'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Dialog } from '@/components/ui/dialog'
import { Separator } from '@/components/ui/separator'
import { cn } from '@/lib/utils'
import { CATEGORY_LABEL, PRIORITY_COLOR, PRIORITY_LABEL, STATUS_LABEL, STATUS_STYLE, hoursPair } from './taskMeta'

const COMMENT_TEMPLATES: QuickInsert[] = [
  { label: '进度', text: '**进度**：已完成 XXX，剩余 XXX' },
  { label: '结论', text: '**结论**：\n\n> 这里是结论' },
  { label: '待办', text: '- [ ] 待确认 1\n- [ ] 待确认 2' },
  { label: '代码', text: '```go\n// 相关代码片段\n```' }
]

export const formatTime = (value?: string) => {
  if (!value) return ''
  const d = new Date(value)
  if (Number.isNaN(d.getTime())) return value
  const p = (n: number) => `${n}`.padStart(2, '0')
  return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())} ${p(d.getHours())}:${p(d.getMinutes())}`
}

/**
 * 任务详情 + 评论。仪表盘和任务列表共用一份，勾选完想细看就直接点开。
 * task 为 null 时关闭。
 */
export default function TaskDetail({ task, onClose }: { task: Task | null; onClose: () => void }) {
  const [comments, setComments] = useState<TaskComment[]>([])
  const [draft, setDraft] = useState('')
  const [loading, setLoading] = useState(false)

  useEffect(() => {
    if (!task?.id) {
      setComments([])
      return
    }
    setDraft('')
    setComments([])
    setLoading(true)
    let alive = true
    taskApi
      .comments(task.id)
      .then(list => alive && setComments(list || []))
      .catch(error => console.error('Failed to load comments:', error))
      .finally(() => alive && setLoading(false))
    return () => {
      alive = false
    }
  }, [task?.id])

  const submit = async () => {
    if (!task?.id || !draft.trim()) return
    try {
      const created = await taskApi.addComment(task.id, draft.trim())
      setComments(prev => [...prev, created])
      setDraft('')
    } catch (error) {
      console.error('Failed to add comment:', error)
    }
  }

  const remove = async (commentId?: number) => {
    if (!commentId) return
    await taskApi.removeComment(commentId)
    setComments(prev => prev.filter(c => c.id !== commentId))
  }

  return (
    <Dialog
      open={!!task}
      onClose={onClose}
      className="max-w-3xl"
      title={
        task && (
          <span className="flex flex-wrap items-center gap-2">
            <span className="tabular text-xs font-normal text-muted-foreground">#{task.number}</span>
            <span>{task.title}</span>
            <Badge>{CATEGORY_LABEL[task.category]}</Badge>
            <span className={cn('rounded-md px-1.5 py-0.5 text-[11px] font-medium', STATUS_STYLE[task.status])}>
              {STATUS_LABEL[task.status]}
            </span>
          </span>
        )
      }
      footer={<Button variant="outline" onClick={onClose}>关闭</Button>}
    >
      {task && (
        <div className="space-y-4">
          <div className="flex flex-wrap items-center gap-x-4 gap-y-1.5 text-xs text-muted-foreground">
            <span>
              优先级：
              <b style={{ color: PRIORITY_COLOR[task.priority] }}>{PRIORITY_LABEL[task.priority]}</b>
            </span>
            {task.project_name && <span>项目：{task.project_name}</span>}
            {task.scheduled_date && <span>计划：{task.scheduled_date}</span>}
            {task.due_date && <span>截止：{task.due_date}</span>}
            {(task.estimated_hours || task.actual_hours) && (
              <span className="inline-flex items-center gap-1">
                <Timer className="size-3" />
                {hoursPair(task.actual_hours, task.estimated_hours)}
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

          {task.content && (
            <div className="rounded-lg border border-border bg-secondary/40 px-3.5 py-3">
              <RichText source={task.content} />
            </div>
          )}

          <Separator />

          <div>
            <div className="mb-2 flex items-center gap-2 text-[13px] font-semibold">
              评论
              <span className="text-[11px] font-normal text-muted-foreground">{comments.length} 条</span>
            </div>

            {loading && <p className="text-xs text-muted-foreground">加载中…</p>}
            {!loading && comments.length === 0 && (
              <p className="mb-3 text-xs text-muted-foreground">还没有评论，用下面的编辑器记录进展。</p>
            )}

            <ul className="mb-3 space-y-2.5">
              {comments.map((comment, index) => (
                <li key={comment.id ?? index} className="flex gap-2.5 rounded-lg border border-border px-3 py-2.5">
                  <span className="mt-0.5 flex size-6 shrink-0 items-center justify-center rounded-full bg-accent text-[10px] font-semibold text-accent-foreground">
                    我
                  </span>
                  <div className="min-w-0 flex-1">
                    <div className="mb-1 flex items-center gap-2">
                      <span className="text-[12px] font-medium">我</span>
                      <span className="text-[11px] text-muted-foreground">{formatTime(comment.created_at)}</span>
                      <Button
                        variant="ghost"
                        size="xs"
                        className="ml-auto text-muted-foreground hover:bg-red-50 hover:text-red-600"
                        onClick={() => remove(comment.id)}
                      >
                        删除
                      </Button>
                    </div>
                    <RichText source={comment.content} />
                  </div>
                </li>
              ))}
            </ul>

            <RichTextEditor
              value={draft}
              onChange={setDraft}
              onSubmit={submit}
              placeholder="写点进展、结论或贴段代码…（⌘/Ctrl + Enter 发送）"
              quickInserts={COMMENT_TEMPLATES}
              submitLabel="发送评论"
            />
          </div>
        </div>
      )}
    </Dialog>
  )
}
