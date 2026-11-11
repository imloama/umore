import { useEffect, useState } from 'react'
import { Check, Timer } from 'lucide-react'
import { taskApi } from '../services/api'
import type { Task, TaskComment } from '../types'
import { useI18n } from '@/i18n'
import RichText from './RichText'
import RichTextEditor, { type QuickInsert } from './RichTextEditor'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Dialog } from '@/components/ui/dialog'
import { Separator } from '@/components/ui/separator'
import { cn } from '@/lib/utils'
import {
  CATEGORY_KEYS,
  PRIORITY_COLOR,
  PRIORITY_KEYS,
  STATUS_STYLE,
  formatHours
} from './taskMeta'

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
 *
 * onComplete 传了就显示「完成任务」：详情里看完正文和评论才决定收工的场景比列表多，
 * 状态在组件内自己流转，省得调用方来回传 task。
 */
export default function TaskDetail({
  task,
  onClose,
  onComplete
}: {
  task: Task | null
  onClose: () => void
  onComplete?: (task: Task) => Promise<void> | void
}) {
  const { t } = useI18n()
  const [comments, setComments] = useState<TaskComment[]>([])
  const [draft, setDraft] = useState('')
  const [loading, setLoading] = useState(false)
  const [saving, setSaving] = useState(false)

  const templates: QuickInsert[] = [
    { label: t('task.tpl.progress'), text: t('task.tpl.progressText') },
    { label: t('task.tpl.conclusion'), text: t('task.tpl.conclusionText') },
    { label: t('task.tpl.todo'), text: t('task.tpl.todoText') },
    { label: t('task.tpl.code'), text: t('task.tpl.codeText') }
  ]

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

  const complete = async () => {
    if (!task || saving) return
    setSaving(true)
    try {
      await onComplete?.(task)
    } catch (error) {
      console.error('Failed to complete task:', error)
    } finally {
      setSaving(false)
    }
  }

  const hours = formatHours(task?.actual_hours) || formatHours(task?.estimated_hours)

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
            <Badge>{t(CATEGORY_KEYS[task.category])}</Badge>
            <span className={cn('rounded-md px-1.5 py-0.5 text-[11px] font-medium', STATUS_STYLE[task.status])}>
              {t(`status.${task.status}`)}
            </span>
          </span>
        )
      }
      footer={
        <>
          <Button variant="outline" onClick={onClose}>
            {t('task.detail.close')}
          </Button>
          {onComplete && task && task.status !== 'DONE' && (
            <Button onClick={complete} disabled={saving}>
              <Check />
              {t('task.detail.complete')}
            </Button>
          )}
        </>
      }
    >
      {task && (
        <div className="space-y-4">
          <div className="flex flex-wrap items-center gap-x-4 gap-y-1.5 text-xs text-muted-foreground">
            <span>
              {t('task.detail.priority')}
              <b style={{ color: PRIORITY_COLOR[task.priority] }}>{t(PRIORITY_KEYS[task.priority])}</b>
            </span>
            {task.project_name && <span>{t('task.detail.project', { name: task.project_name })}</span>}
            {task.scheduled_date && <span>{t('task.detail.scheduled', { date: task.scheduled_date })}</span>}
            {task.due_date && <span>{t('task.detail.due', { date: task.due_date })}</span>}
            {hours && (
              <span className="inline-flex items-center gap-1">
                <Timer className="size-3" />
                {formatHours(task.actual_hours) || '0h'} / {formatHours(task.estimated_hours) || t('hours.notSet')}
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
              {t('task.detail.comments')}
              <span className="text-[11px] font-normal text-muted-foreground">
                {t('task.detail.commentCount', { n: comments.length })}
              </span>
            </div>

            {loading && <p className="text-xs text-muted-foreground">{t('task.detail.loadingComments')}</p>}
            {!loading && comments.length === 0 && (
              <p className="mb-3 text-xs text-muted-foreground">{t('task.detail.noComments')}</p>
            )}

            <ul className="mb-3 space-y-2.5">
              {comments.map((comment, index) => (
                <li key={comment.id ?? index} className="flex gap-2.5 rounded-lg border border-border px-3 py-2.5">
                  <span className="mt-0.5 flex size-6 shrink-0 items-center justify-center rounded-full bg-accent text-[10px] font-semibold text-accent-foreground">
                    {t('task.detail.me').slice(0, 1)}
                  </span>
                  <div className="min-w-0 flex-1">
                    <div className="mb-1 flex items-center gap-2">
                      <span className="text-[12px] font-medium">{t('task.detail.me')}</span>
                      <span className="text-[11px] text-muted-foreground">{formatTime(comment.created_at)}</span>
                      <Button
                        variant="ghost"
                        size="xs"
                        className="ml-auto text-muted-foreground hover:bg-red-50 hover:text-red-600"
                        onClick={() => remove(comment.id)}
                      >
                        {t('common.delete')}
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
              placeholder={t('task.detail.commentPlaceholder')}
              quickInserts={templates}
              submitLabel={t('task.detail.send')}
            />
          </div>
        </div>
      )}
    </Dialog>
  )
}
