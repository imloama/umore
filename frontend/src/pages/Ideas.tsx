import { useEffect, useRef, useState } from 'react'
import { Link } from 'react-router-dom'
import {
  Archive,
  ArchiveRestore,
  Check,
  FolderPlus,
  Inbox,
  Lightbulb,
  Pencil,
  Plus,
  Trash,
  X
} from 'lucide-react'
import { ideaApi } from '../services/api'
import type { Idea, IdeaStatus } from '../types'
import PageHeader from '../components/PageHeader'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Textarea } from '@/components/ui/textarea'
import { cn } from '@/lib/utils'

// 灵感墙：卡片即内容，点开就在卡片里改，不弹表单。
// 状态只有三个：待整理 → 已成项目 / 已归档，归档的还能捞回来。
const TABS: { value: IdeaStatus; label: string; icon: typeof Inbox }[] = [
  { value: 'inbox', label: '待整理', icon: Inbox },
  { value: 'project', label: '已成项目', icon: FolderPlus },
  { value: 'archived', label: '归档', icon: Archive }
]

const STATUS_BADGE: Record<IdeaStatus, { label: string; variant: 'default' | 'success' | 'muted' }> = {
  inbox: { label: '待整理', variant: 'default' },
  project: { label: '已成项目', variant: 'success' },
  archived: { label: '已归档', variant: 'muted' }
}

const formatTime = (iso?: string) => {
  if (!iso) return ''
  const d = new Date(iso)
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`
}

export default function Ideas() {
  const [status, setStatus] = useState<IdeaStatus>('inbox')
  const [ideas, setIdeas] = useState<Idea[]>([])
  const [loading, setLoading] = useState(true)
  const [draft, setDraft] = useState('')
  const [creating, setCreating] = useState(false)
  const [editingId, setEditingId] = useState<number | null>(null)
  const [editText, setEditText] = useState('')
  const [busyId, setBusyId] = useState<number | null>(null)
  const [toast, setToast] = useState('')
  const draftRef = useRef<HTMLTextAreaElement>(null)
  const editRef = useRef<HTMLTextAreaElement>(null)

  useEffect(() => {
    load()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [status])

  useEffect(() => {
    if (editingId) setTimeout(() => editRef.current?.focus(), 0)
  }, [editingId])

  useEffect(() => {
    if (!toast) return
    const timer = setTimeout(() => setToast(''), 2600)
    return () => clearTimeout(timer)
  }, [toast])

  const load = async () => {
    setLoading(true)
    try {
      const list = await ideaApi.list(status)
      setIdeas(list || [])
    } catch (error) {
      console.error('Failed to load ideas:', error)
    } finally {
      setLoading(false)
    }
  }

  // 顶部的快速记一笔：⌘/Ctrl + Enter 或者点按钮，存完输入框留在原地等下一句
  const create = async () => {
    const value = draft.trim()
    if (!value || creating) return
    setCreating(true)
    try {
      const created = await ideaApi.create(value)
      setDraft('')
      // 当前就在待整理：直接把新卡片塞到最前面，省一次整表刷新
      if (created.status === status) setIdeas(prev => [created, ...prev])
      setTimeout(() => draftRef.current?.focus(), 0)
    } catch (error) {
      console.error('Failed to create idea:', error)
      setToast('没记上，再试一次')
    } finally {
      setCreating(false)
    }
  }

  const saveEdit = async (idea: Idea) => {
    const value = editText.trim()
    setEditingId(null)
    if (!value || value === idea.content) return
    try {
      const updated = await ideaApi.update(idea.id!, { content: value })
      setIdeas(prev => prev.map(item => (item.id === updated.id ? updated : item)))
      setToast('已保存')
    } catch (error) {
      console.error('Failed to update idea:', error)
      setToast('没存上，再试一次')
    }
  }

  const changeStatus = async (idea: Idea, next: IdeaStatus) => {
    if (busyId) return
    setBusyId(idea.id!)
    try {
      await ideaApi.update(idea.id!, { status: next })
      // 当前 tab 里就不该再有它了：直接摘掉，比整表重拉更像「点一下就走」
      setIdeas(prev => prev.filter(item => item.id !== idea.id))
      setToast(next === 'archived' ? '已归档' : '已放回待整理')
    } catch (error) {
      console.error('Failed to change idea status:', error)
      setToast('操作没成功')
    } finally {
      setBusyId(null)
    }
  }

  // 转项目：后端建项目 + 标状态，成功后让它从当前 tab 消失
  const promote = async (idea: Idea) => {
    if (busyId) return
    setBusyId(idea.id!)
    try {
      const res = await ideaApi.promote(idea.id!)
      setIdeas(prev => prev.filter(item => item.id !== idea.id))
      setToast(`已建成项目「${res.project?.name ?? ''}」`)
    } catch (error) {
      console.error('Failed to promote idea:', error)
      setToast('转项目失败，可能有同名项目')
    } finally {
      setBusyId(null)
    }
  }

  const remove = async (idea: Idea) => {
    if (!confirm('删掉就找不回来了。只是不想看到的话，用归档更好。')) return
    try {
      await ideaApi.remove(idea.id!)
      setIdeas(prev => prev.filter(item => item.id !== idea.id))
    } catch (error) {
      console.error('Failed to delete idea:', error)
      setToast('删除失败')
    }
  }

  const emptyHint: Record<IdeaStatus, string> = {
    inbox: '还没有待整理的灵感。想到什么就丢进来，不用先想放哪儿。',
    project: '还没有灵感长成项目。灵感卡片上点「转项目」就有一条。',
    archived: '归档是空的。不想再看到的灵感会收在这儿，随时能捞回来。'
  }

  return (
    <div>
      <PageHeader
        eyebrow="IDEAS"
        title="灵感"
        description="想到就先记下来。之后要么变成项目，要么收进归档，别让它占着脑子。"
        actions={
          <Button size="sm" variant="outline" asChild>
            <Link to="/projects">
              <FolderPlus />
              看项目
            </Link>
          </Button>
        }
      />

      {/* 快速记一笔：这块本身也是卡片，跟下面那堆长得一样 */}
      <Card className="mb-4 p-3">
        <Textarea
          ref={draftRef}
          rows={2}
          value={draft}
          onChange={e => setDraft(e.target.value)}
          onKeyDown={e => {
            if ((e.metaKey || e.ctrlKey) && e.key === 'Enter') {
              e.preventDefault()
              create()
            }
          }}
          placeholder="有个想法…（⌘ / Ctrl + Enter 记下来）"
          className="min-h-[64px] resize-none border-0 bg-transparent px-1 shadow-none focus-visible:ring-0"
        />
        <div className="mt-1.5 flex items-center justify-between gap-2">
          <span className="text-[11px] text-muted-foreground">存在这里的不需要标题，也不打标签。</span>
          <Button size="sm" onClick={create} disabled={creating || !draft.trim()}>
            <Plus />
            记下来
          </Button>
        </div>
      </Card>

      <div className="mb-3 flex items-center justify-between gap-3">
        <Tabs value={status} onValueChange={value => setStatus(value as IdeaStatus)}>
          <TabsList>
            {TABS.map(tab => {
              const Icon = tab.icon
              return (
                <TabsTrigger key={tab.value} value={tab.value}>
                  <Icon className="size-3.5" />
                  {tab.label}
                </TabsTrigger>
              )
            })}
          </TabsList>
        </Tabs>
        <span className="text-[11.5px] text-muted-foreground">{ideas.length} 条</span>
      </div>

      {loading ? (
        <div className="py-16 text-center text-xs text-muted-foreground">加载中…</div>
      ) : ideas.length === 0 ? (
        <Card className="flex flex-col items-center gap-2 px-6 py-14 text-center">
          <Lightbulb className="size-6 text-muted-foreground/60" />
          <div className="text-sm font-medium">这里空着</div>
          <div className="max-w-sm text-xs text-muted-foreground">{emptyHint[status]}</div>
        </Card>
      ) : (
        <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-3">
          {ideas.map(idea => {
            const editing = editingId === idea.id
            const badge = STATUS_BADGE[idea.status]
            return (
              <Card
                key={idea.id}
                className={cn(
                  'group flex flex-col p-3.5 transition-shadow hover:shadow-md',
                  idea.status === 'archived' && 'opacity-80'
                )}
              >
                <div className="mb-2 flex items-center gap-1.5">
                  <Badge variant={badge.variant}>{badge.label}</Badge>
                  {idea.project_id ? (
                    <Link
                      to={`/projects?open=${idea.project_id}`}
                      className="truncate text-[11px] text-primary hover:underline"
                    >
                      {idea.project_name || `项目 #${idea.project_id}`}
                    </Link>
                  ) : (
                    <span className="text-[10.5px] text-muted-foreground">{formatTime(idea.created_at)}</span>
                  )}
                  <div className="ml-auto flex shrink-0 gap-0.5 opacity-0 transition-opacity group-hover:opacity-100 focus-within:opacity-100">
                    {!editing && (
                      <Button
                        variant="ghost"
                        size="icon-sm"
                        title="改内容"
                        onClick={() => {
                          setEditingId(idea.id!)
                          setEditText(idea.content)
                        }}
                      >
                        <Pencil />
                      </Button>
                    )}
                    <Button
                      variant="ghost"
                      size="icon-sm"
                      className="text-muted-foreground hover:bg-red-50 hover:text-red-600"
                      title="删除"
                      onClick={() => remove(idea)}
                    >
                      <Trash />
                    </Button>
                  </div>
                </div>

                {editing ? (
                  <div className="flex-1">
                    <Textarea
                      ref={editRef}
                      rows={4}
                      value={editText}
                      onChange={e => setEditText(e.target.value)}
                      onKeyDown={e => {
                        if ((e.metaKey || e.ctrlKey) && e.key === 'Enter') {
                          e.preventDefault()
                          saveEdit(idea)
                        }
                        if (e.key === 'Escape') setEditingId(null)
                      }}
                      className="min-h-[96px] text-[13px]"
                    />
                    <div className="mt-1.5 flex justify-end gap-1.5">
                      <Button variant="ghost" size="xs" onClick={() => setEditingId(null)}>
                        <X />
                        取消
                      </Button>
                      <Button size="xs" onClick={() => saveEdit(idea)} disabled={!editText.trim()}>
                        <Check />
                        保存
                      </Button>
                    </div>
                  </div>
                ) : (
                  <p
                    className="flex-1 cursor-text whitespace-pre-wrap text-[13px] leading-relaxed"
                    onDoubleClick={() => {
                      setEditingId(idea.id!)
                      setEditText(idea.content)
                    }}
                    title="双击改内容"
                  >
                    {idea.content}
                  </p>
                )}

                {!editing && (
                  <div className="mt-3 flex flex-wrap items-center gap-1.5 border-t border-border/70 pt-2.5">
                    {idea.status === 'inbox' && (
                      <>
                        <Button
                          size="xs"
                          variant="outline"
                          disabled={busyId === idea.id}
                          onClick={() => promote(idea)}
                          title="建一条项目，灵感标为已成项目"
                        >
                          <FolderPlus />
                          转项目
                        </Button>
                        <Button
                          size="xs"
                          variant="ghost"
                          disabled={busyId === idea.id}
                          onClick={() => changeStatus(idea, 'archived')}
                        >
                          <Archive />
                          归档
                        </Button>
                      </>
                    )}
                    {idea.status === 'project' && idea.project_id && (
                      <Button size="xs" variant="outline" asChild>
                        <Link to={`/projects?open=${idea.project_id}`}>打开项目</Link>
                      </Button>
                    )}
                    {idea.status === 'archived' && (
                      <Button
                        size="xs"
                        variant="ghost"
                        disabled={busyId === idea.id}
                        onClick={() => changeStatus(idea, 'inbox')}
                      >
                        <ArchiveRestore />
                        放回待整理
                      </Button>
                    )}
                    <span className="ml-auto text-[10.5px] text-muted-foreground">{formatTime(idea.updated_at)}</span>
                  </div>
                )}
              </Card>
            )
          })}
        </div>
      )}

      {/* 轻提示：存没存上，一句话就够 */}
      {toast && (
        <div className="fixed bottom-6 left-1/2 z-50 -translate-x-1/2 rounded-full bg-slate-900/90 px-3.5 py-1.5 text-[12px] text-white shadow-lg">
          {toast}
        </div>
      )}
    </div>
  )
}
