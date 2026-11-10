import { useEffect, useRef, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { ArrowUpRight, FolderPlus, Lightbulb, ListTodo, Plus, Sparkles } from 'lucide-react'
import { ideaApi, projectApi, taskApi } from '../services/api'
import type { TaskCategory } from '../types'
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

const OPTIONS: { key: string; icon: typeof Plus; title: string; hint: string; kind: Kind }[] = [
  { key: 'idea', icon: Lightbulb, title: '灵感', hint: '先记下来，之后归档或变项目', kind: { type: 'idea' } },
  { key: 'todo', icon: ListTodo, title: '待办', hint: '今天要做的一件事', kind: { type: 'task', category: 'todo' } },
  { key: 'issue', icon: ListTodo, title: '问题', hint: '哪里坏了、哪里不对', kind: { type: 'task', category: 'issue' } },
  {
    key: 'requirement',
    icon: Sparkles,
    title: '需求',
    hint: '要加的东西、要改的行为',
    kind: { type: 'task', category: 'requirement' }
  },
  {
    key: 'opportunity',
    icon: ArrowUpRight,
    title: '商机',
    hint: '可能有戏的活，赢单后转项目',
    kind: { type: 'task', category: 'opportunity' }
  },
  { key: 'project', icon: FolderPlus, title: '项目', hint: '直接开工，客户金额之后补', kind: { type: 'project' } }
]

const KIND_LABEL: Record<TaskCategory, string> = {
  todo: '待办',
  issue: '问题',
  requirement: '需求',
  opportunity: '商机'
}

const sameKind = (a: Kind, b: Kind) =>
  a.type === b.type && (a.type !== 'task' || b.type !== 'task' || a.category === b.category)

/**
 * 顶栏右上角的快捷输入口：点 + 选类型，写一句回车就存。
 * 存完不清空弹窗、只清输入框，方便连着记好几条。
 */
export default function QuickCreate() {
  const navigate = useNavigate()
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
      setError('没存上，再试一次')
      setSaving(false)
    }
  }

  const activeLabel = kind.type === 'idea' ? '灵感' : kind.type === 'project' ? '项目' : KIND_LABEL[kind.category]
  const placeholder =
    kind.type === 'idea'
      ? '想到什么就写什么，一句话也行…'
      : kind.type === 'project'
        ? '项目叫什么？'
        : `这条${activeLabel}要做什么？`

  return (
    <>
      <Button size="sm" onClick={() => setOpen(true)} title="快捷新建">
        <Plus />
        <span className="hidden sm:inline">新建</span>
      </Button>

      <Dialog
        open={open}
        onClose={close}
        title="记一笔"
        description="⌘ / Ctrl + Enter 保存，存完可以直接接着记下一条。"
        className="max-w-xl"
        footer={
          <>
            <Button variant="ghost" onClick={() => { close(); navigate('/ideas') }}>
              去灵感墙
            </Button>
            <Button onClick={submit} disabled={saving || !text.trim()}>
              {saving ? '保存中…' : `存为${activeLabel}`}
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
                  <span className="block text-[13px] font-medium leading-tight">{option.title}</span>
                  <span className="mt-0.5 block text-[10.5px] leading-snug text-muted-foreground">{option.hint}</span>
                </span>
              </button>
            )
          })}
        </div>

        <div className="mt-3.5">
          <Label>{activeLabel}内容 *</Label>
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
              ? '只存正文，标题和标签都不要。'
              : kind.type === 'project'
                ? '名称先占个位，客户、金额、仓库去项目页补。'
                : '这一句就是标题，细节去任务页补。'}
          </p>
          {error && <p className="mt-1.5 text-[11.5px] text-red-600">{error}</p>}
        </div>
      </Dialog>
    </>
  )
}
