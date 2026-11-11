import { useEffect, useRef, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { ArrowUpRight, FolderPlus, Lightbulb, ListTodo, Plus, Sparkles } from 'lucide-react'
import { ideaApi, projectApi, taskApi } from '../services/api'
import type { TaskCategory } from '../types'
import { useI18n, type MessageKey } from '@/i18n'
import { Button } from '@/components/ui/button'
import { Dialog } from '@/components/ui/dialog'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import { cn } from '@/lib/utils'

// 快捷口只覆盖「一句话就能记下来」的几类：灵感、任务类（待办/问题/需求/商机）、项目名。
// 细节（工时、标签、金额……）一律留到各自页面补——这里多一个输入框就少一分随手记的意愿。
type Kind =
  | { type: 'idea' }
  | { type: 'task'; category: TaskCategory }
  | { type: 'project' }

const OPTIONS: { key: string; icon: typeof Plus; titleKey: MessageKey; hintKey: MessageKey; kind: Kind }[] = [
  { key: 'idea', icon: Lightbulb, titleKey: 'quick.idea', hintKey: 'quick.ideaHint', kind: { type: 'idea' } },
  { key: 'todo', icon: ListTodo, titleKey: 'quick.todo', hintKey: 'quick.todoHint', kind: { type: 'task', category: 'todo' } },
  { key: 'issue', icon: ListTodo, titleKey: 'quick.issue', hintKey: 'quick.issueHint', kind: { type: 'task', category: 'issue' } },
  {
    key: 'requirement',
    icon: Sparkles,
    titleKey: 'quick.requirement',
    hintKey: 'quick.requirementHint',
    kind: { type: 'task', category: 'requirement' }
  },
  {
    key: 'opportunity',
    icon: ArrowUpRight,
    titleKey: 'quick.opportunity',
    hintKey: 'quick.opportunityHint',
    kind: { type: 'task', category: 'opportunity' }
  },
  { key: 'project', icon: FolderPlus, titleKey: 'quick.project', hintKey: 'quick.projectHint', kind: { type: 'project' } }
]

const KIND_KEY: Record<TaskCategory, MessageKey> = {
  todo: 'quick.todo',
  issue: 'quick.issue',
  requirement: 'quick.requirement',
  opportunity: 'quick.opportunity'
}

const sameKind = (a: Kind, b: Kind) =>
  a.type === b.type && (a.type !== 'task' || b.type !== 'task' || a.category === b.category)

/**
 * 顶栏右上角的快捷输入口：点 + 选类型，写一句回车就存。
 * 存完不清空弹窗、只清输入框，方便连着记好几条。
 */
export default function QuickCreate() {
  const navigate = useNavigate()
  const { t } = useI18n()
  const [open, setOpen] = useState(false)
  const [kind, setKind] = useState<Kind>({ type: 'idea' })
  const [text, setText] = useState('')
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')
  const inputRef = useRef<HTMLTextAreaElement>(null)

  // 打开就聚焦输入框：键盘一路走完，不用再点一下
  useEffect(() => {
    if (open) setTimeout(() => inputRef.current?.focus(), 0)
  }, [open, kind])

  const close = () => {
    setOpen(false)
    setText('')
    setError('')
    setSaving(false)
  }

  const submit = async () => {
    const value = text.trim()
    if (!value || saving) return
    setSaving(true)
    setError('')
    try {
      if (kind.type === 'idea') {
        await ideaApi.create(value)
      } else if (kind.type === 'project') {
        await projectApi.create({ name: value, description: '', status: 'active' })
      } else {
        await taskApi.create({
          title: value,
          content: '',
          category: kind.category,
          status: 'TODO',
          priority: 'MEDIUM',
          labels: [],
          estimated_hours: 0,
          actual_hours: 0,
          is_today: false
        })
      }
      setText('')
      setSaving(false)
      setTimeout(() => inputRef.current?.focus(), 0)
    } catch (err) {
      console.error('quick create failed:', err)
      setError(t('quick.createFailed'))
      setSaving(false)
    }
  }

  const activeLabel = kind.type === 'idea' ? t('quick.idea') : kind.type === 'project' ? t('quick.project') : t(KIND_KEY[kind.category])
  const placeholder =
    kind.type === 'idea'
      ? t('quick.placeholderIdea')
      : kind.type === 'project'
        ? t('quick.placeholderProject')
        : t('quick.placeholderTask', { kind: activeLabel })

  return (
    <>
      <Button size="sm" onClick={() => setOpen(true)} title={t('nav.quickCreate')}>
        <Plus />
        <span className="hidden sm:inline">{t('quick.new')}</span>
      </Button>

      <Dialog
        open={open}
        onClose={close}
        title={t('quick.title')}
        description={t('quick.description')}
        className="max-w-xl"
        footer={
          <>
            <Button variant="ghost" onClick={() => { close(); navigate('/ideas') }}>
              {t('quick.goIdeas')}
            </Button>
            <Button onClick={submit} disabled={saving || !text.trim()}>
              {saving ? t('common.saving') : t('quick.saveAs', { kind: activeLabel })}
            </Button>
          </>
        }
      >
        <div className="grid grid-cols-2 gap-1.5 sm:grid-cols-3">
          {OPTIONS.map(option => {
            const Icon = option.icon
            const active = sameKind(option.kind, kind)
            return (
              <button
                key={option.key}
                type="button"
                onClick={() => setKind(option.kind)}
                className={cn(
                  'flex items-start gap-2 rounded-md border px-2.5 py-2 text-left transition-colors',
                  active ? 'border-primary/40 bg-primary/5' : 'border-border hover:bg-secondary/60'
                )}
              >
                <Icon className={cn('mt-0.5 size-4 shrink-0', active ? 'text-primary' : 'text-muted-foreground')} />
                <span className="min-w-0">
                  <span className="block text-[13px] font-medium leading-tight">{t(option.titleKey)}</span>
                  <span className="mt-0.5 block text-[10.5px] leading-snug text-muted-foreground">{t(option.hintKey)}</span>
                </span>
              </button>
            )
          })}
        </div>

        <div className="mt-3.5">
          <Label>{t('quick.contentLabel', { kind: activeLabel })}</Label>
          <Textarea
            ref={inputRef}
            rows={kind.type === 'project' ? 2 : 4}
            value={text}
            onChange={e => setText(e.target.value)}
            onKeyDown={e => {
              if ((e.metaKey || e.ctrlKey) && e.key === 'Enter') {
                e.preventDefault()
                submit()
              }
            }}
            placeholder={placeholder}
          />
          <p className="mt-1.5 text-[11px] text-muted-foreground">
            {kind.type === 'idea'
              ? t('quick.hintIdea')
              : kind.type === 'project'
                ? t('quick.hintProject')
                : t('quick.hintTask')}
          </p>
          {error && <p className="mt-1.5 text-[11.5px] text-red-600">{error}</p>}
        </div>
      </Dialog>
    </>
  )
}
