import { useEffect, useState } from 'react'
import { ArrowDownRight, ArrowUpRight, Plus, Trash, Wallet } from 'lucide-react'
import { projectApi, receivableApi } from '../services/api'
import type { Project, ProjectAmountChange, Receivable, TaskComment } from '../types'
import RichText from './RichText'
import RichTextEditor, { type QuickInsert } from './RichTextEditor'
import ReceivableForm from './ReceivableForm'
import ReceivableList from './ReceivableList'
import { money, sumAmounts } from './receivableMeta'
import { formatTime } from './TaskDetail'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Dialog } from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { cn } from '@/lib/utils'
import { formatHours } from './taskMeta'

const draftOf = (n?: number) => (n ? String(Math.round(n * 100) / 100) : '')

const COMMENT_TEMPLATES: QuickInsert[] = [
  { label: '变更', text: '**变更**：客户加了一个 XXX，工期顺延 X 天' },
  { label: '问题', text: '**问题**：\n\n> 描述 + 影响范围' },
  { label: '收款', text: '**收款**：已收 XXX，剩余 XXX' }
]

/**
 * 项目详情：一层 tab 分开「概览 / 收款计划 / 金额变更 / 项目记录」。
 * 收款计划是下一级：点某一笔再进编辑弹窗改备注、到期时间。
 */
export default function ProjectDetail({
  project,
  onClose,
  onChanged,
  initialTab = 'overview'
}: {
  project: Project | null
  onClose: () => void
  onChanged?: () => void
  /** 从回款页点进来时直接落到收款计划那一页 */
  initialTab?: string
}) {
  const [tab, setTab] = useState('overview')
  const [changes, setChanges] = useState<ProjectAmountChange[]>([])
  const [comments, setComments] = useState<TaskComment[]>([])
  const [receivables, setReceivables] = useState<Receivable[]>([])
  const [commentDraft, setCommentDraft] = useState('')
  const [loading, setLoading] = useState(false)
  const [amountDraft, setAmountDraft] = useState('')
  const [reasonDraft, setReasonDraft] = useState('')
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')
  const [formOpen, setFormOpen] = useState(false)
  const [editing, setEditing] = useState<Receivable | null>(null)

  useEffect(() => {
    if (!project?.id) {
      setChanges([])
      setComments([])
      setReceivables([])
      return
    }
    const id = project.id
    setTab(initialTab)
    setLoading(true)
    setCommentDraft('')
    setReasonDraft('')
    setAmountDraft(draftOf(project.amount))
    setError('')
    let alive = true
    Promise.all([projectApi.amountChanges(id), projectApi.comments(id), receivableApi.list({ project_id: id })])
      .then(([changeList, commentList, receivableList]) => {
        if (!alive) return
        setChanges(changeList || [])
        setComments(commentList || [])
        setReceivables(receivableList || [])
      })
      .catch(err => console.error('Failed to load project detail:', err))
      .finally(() => alive && setLoading(false))
    return () => {
      alive = false
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [project?.id, initialTab])

  const reloadReceivables = async () => {
    if (!project?.id) return
    const list = await receivableApi.list({ project_id: project.id })
    setReceivables(list || [])
  }

  const submitAmount = async () => {
    if (!project?.id) return
    const next = parseFloat(amountDraft)
    if (!Number.isFinite(next) || next < 0) {
      setError('金额要填个非负数字')
      return
    }
    if (next === (project.amount || 0)) {
      setError('金额没变，不用记录')
      return
    }
    setSaving(true)
    setError('')
    try {
      await projectApi.addAmountChange(project.id, next, reasonDraft.trim())
      const [changeList, reloaded] = await Promise.all([
        projectApi.amountChanges(project.id),
        projectApi.get(project.id)
      ])
      setChanges(changeList || [])
      setAmountDraft(draftOf(reloaded.amount))
      setReasonDraft('')
      onChanged?.()
    } catch (err) {
      console.error('Failed to add amount change:', err)
      setError('保存失败')
    } finally {
      setSaving(false)
    }
  }

  const removeChange = async (changeId?: number) => {
    if (!changeId || !project?.id) return
    if (!confirm('删掉这条变更记录？项目总金额不会回滚。')) return
    await projectApi.removeAmountChange(changeId)
    const list = await projectApi.amountChanges(project.id)
    setChanges(list || [])
    onChanged?.()
  }

  const submitComment = async () => {
    if (!project?.id || !commentDraft.trim()) return
    try {
      const created = await projectApi.addComment(project.id, commentDraft.trim())
      setComments(prev => [...prev, created])
      setCommentDraft('')
    } catch (err) {
      console.error('Failed to add project comment:', err)
    }
  }

  const removeComment = async (commentId?: number) => {
    if (!commentId) return
    await projectApi.removeComment(commentId)
    setComments(prev => prev.filter(c => c.id !== commentId))
  }

  const toggleDone = async (item: Receivable) => {
    if (!item.id) return
    await receivableApi.setDone(item.id, item.status !== 'done')
    await reloadReceivables()
    onChanged?.()
  }

  const removeReceivable = async (item: Receivable) => {
    if (!item.id) return
    if (!confirm(`删掉「${item.title}」这笔待收？`)) return
    await receivableApi.remove(item.id)
    await reloadReceivables()
    onChanged?.()
  }

  const amount = project?.amount || 0
  const received = project?.received || 0
  const unpaid = Math.max(0, amount - received)
  // 收的比合同还多：不是欠款，是超收，得单独说一句，不能只显示「未收 ¥0」
  const overReceived = amount > 0 && received > amount
  const pendingReceivable = receivables.filter(r => r.status === 'pending')
  const pendingAmount = sumAmounts(pendingReceivable.map(r => r.amount))
  const overdueAmount = sumAmounts(pendingReceivable.filter(r => r.overdue).map(r => r.amount))

  return (
    <Dialog
      open={!!project}
      onClose={onClose}
      className="max-w-3xl"
      title={
        project && (
          <span className="flex flex-wrap items-center gap-2">
            <span>{project.name}</span>
            {project.status === 'archived' && <Badge variant="muted">已归档</Badge>}
            {project.customer && <Badge variant="outline">{project.customer}</Badge>}
          </span>
        )
      }
      description={project?.description || '金额、收款计划、变更记录都在这张单上。'}
      footer={<Button variant="outline" onClick={onClose}>关闭</Button>}
    >
      {project && (
        <Tabs value={tab} onValueChange={setTab}>
          <TabsList>
            <TabsTrigger value="overview">概览</TabsTrigger>
            <TabsTrigger value="receivables">
              收款计划
              <span className="text-[11px] font-normal text-muted-foreground">{pendingReceivable.length}</span>
            </TabsTrigger>
            <TabsTrigger value="changes">
              金额变更
              <span className="text-[11px] font-normal text-muted-foreground">{changes.length}</span>
            </TabsTrigger>
            <TabsTrigger value="records">
              项目记录
              <span className="text-[11px] font-normal text-muted-foreground">{comments.length}</span>
            </TabsTrigger>
          </TabsList>

          {/* 概览 */}
          <TabsContent value="overview" className="space-y-4">
            <div className="grid gap-2 sm:grid-cols-3">
              <div className="rounded-lg border border-border px-3 py-2.5">
                <div className="text-[11px] text-muted-foreground">合同总金额</div>
                <div className="tabular mt-1 text-[17px] font-semibold">{money(amount)}</div>
                {!!project.amount_change_count && (
                  <div className="mt-0.5 text-[10.5px] text-muted-foreground">
                    变更 {project.amount_change_count} 次
                    {project.amount_changed_at && ` · ${formatTime(project.amount_changed_at)}`}
                  </div>
                )}
              </div>
              <div className="rounded-lg border border-emerald-200 bg-emerald-50/40 px-3 py-2.5">
                <div className="flex items-center gap-1 text-[11px] text-emerald-700">
                  <Wallet className="size-3" />
                  已收款
                </div>
                <div className="tabular mt-1 text-[17px] font-semibold text-emerald-700">{money(received)}</div>
                <div className="mt-0.5 text-[10.5px] text-emerald-700/80">
                  占比 {amount > 0 ? Math.round((received / amount) * 100) : 0}%
                </div>
              </div>
              <div
                className={cn(
                  'rounded-lg border px-3 py-2.5',
                  overReceived ? 'border-sky-200 bg-sky-50/40' : unpaid > 0 ? 'border-amber-200 bg-amber-50/40' : 'border-border'
                )}
              >
                <div
                  className={cn(
                    'text-[11px]',
                    overReceived ? 'text-sky-700' : unpaid > 0 ? 'text-amber-700' : 'text-muted-foreground'
                  )}
                >
                  {overReceived ? '超收' : '未收'}
                </div>
                <div
                  className={cn(
                    'tabular mt-1 text-[17px] font-semibold',
                    overReceived ? 'text-sky-700' : unpaid > 0 ? 'text-amber-700' : ''
                  )}
                >
                  {overReceived ? money(received - amount) : money(unpaid)}
                </div>
                <div className="mt-0.5 text-[10.5px] text-muted-foreground">
                  工时 {formatHours(project.actual_hours) || '0h'} / 评估 {formatHours(project.estimated_hours) || '—'}
                </div>
              </div>
            </div>

            <div className="grid gap-2 sm:grid-cols-2">
              <div className="rounded-lg border border-border px-3 py-2.5">
                <div className="flex items-center justify-between text-[11px] text-muted-foreground">
                  <span>待收款计划</span>
                  <Button variant="ghost" size="xs" onClick={() => setTab('receivables')}>去看</Button>
                </div>
                <div className="tabular mt-1 text-[15px] font-semibold">{money(pendingAmount)}</div>
                <div className="mt-0.5 text-[10.5px] text-muted-foreground">
                  {pendingReceivable.length} 笔未收
                  {overdueAmount > 0 && <span className="ml-1 text-red-600">· 逾期 {money(overdueAmount)}</span>}
                </div>
              </div>
              <div className="rounded-lg border border-border px-3 py-2.5">
                <div className="text-[11px] text-muted-foreground">关联任务</div>
                <div className="tabular mt-1 text-[15px] font-semibold">{project.task_count || 0} 个</div>
                <div className="mt-0.5 text-[10.5px] text-muted-foreground">
                  {formatHours(project.actual_hours) || '0h'} 已投入
                </div>
              </div>
            </div>

            {loading && <p className="text-xs text-muted-foreground">加载中…</p>}
          </TabsContent>

          {/* 收款计划：下一级就是每一笔的编辑 */}
          <TabsContent value="receivables" className="space-y-3">
            <div className="flex items-center justify-between">
              <div className="text-[12px] text-muted-foreground">
                未收 <span className="tabular font-semibold text-amber-700">{money(pendingAmount)}</span>
                {overdueAmount > 0 && (
                  <span className="ml-2 text-red-600">逾期 <span className="tabular">{money(overdueAmount)}</span></span>
                )}
              </div>
              <Button
                size="xs"
                onClick={() => {
                  setEditing(null)
                  setFormOpen(true)
                }}
              >
                <Plus />
                记一笔待收
              </Button>
            </div>

            <ReceivableList
              items={receivables}
              showProject={false}
              onEdit={item => {
                setEditing(item)
                setFormOpen(true)
              }}
              onToggleDone={toggleDone}
              onDelete={removeReceivable}
            />
            <p className="text-[11px] text-muted-foreground">
              点某一笔可以改备注、金额、到期时间；勾上圆圈就是收到了。
            </p>
          </TabsContent>

          {/* 金额变更 */}
          <TabsContent value="changes" className="space-y-3">
            <div className="rounded-lg border border-border bg-secondary/30 p-2.5">
              <div className="grid gap-2 sm:grid-cols-[130px_1fr_auto]">
                <div>
                  <Label className="text-[11px]">调整后总金额</Label>
                  <Input
                    type="number"
                    step="0.01"
                    min="0"
                    value={amountDraft}
                    onChange={e => setAmountDraft(e.target.value)}
                    className="h-8"
                  />
                </div>
                <div>
                  <Label className="text-[11px]">变更说明</Label>
                  <Input
                    value={reasonDraft}
                    onChange={e => setReasonDraft(e.target.value)}
                    placeholder="客户加了一个模块 / 砍掉一部分需求"
                    className="h-8"
                  />
                </div>
                <div className="flex items-end">
                  <Button size="sm" onClick={submitAmount} disabled={saving}>
                    <Plus />
                    记一笔变更
                  </Button>
                </div>
              </div>
              {error && <div className="mt-1.5 text-[11px] text-red-600">{error}</div>}
            </div>

            {changes.length === 0 ? (
              <p className="text-xs text-muted-foreground">还没有金额变更。中途加需求、砍范围，都在这儿记一笔。</p>
            ) : (
              <ul className="space-y-1.5">
                {changes.map(change => {
                  const up = change.delta > 0
                  return (
                    <li
                      key={change.id}
                      className="group flex items-center gap-2.5 rounded-lg border border-border px-3 py-2 text-[12px]"
                    >
                      <span
                        className={cn(
                          'flex size-6 shrink-0 items-center justify-center rounded-full',
                          up ? 'bg-emerald-50 text-emerald-600' : 'bg-red-50 text-red-600'
                        )}
                      >
                        {up ? <ArrowUpRight className="size-3.5" /> : <ArrowDownRight className="size-3.5" />}
                      </span>
                      <div className="min-w-0 flex-1">
                        <div className="truncate font-medium">{change.reason || '手动调整总金额'}</div>
                        <div className="tabular mt-0.5 text-[11px] text-muted-foreground">
                          {money(change.old_amount)} → {money(change.new_amount)} · {formatTime(change.created_at)}
                        </div>
                      </div>
                      <span
                        className={cn(
                          'tabular shrink-0 text-[13px] font-semibold',
                          up ? 'text-emerald-600' : 'text-red-600'
                        )}
                      >
                        {up ? '+' : '-'}
                        {money(Math.abs(change.delta))}
                      </span>
                      <Button
                        variant="ghost"
                        size="icon-sm"
                        className="shrink-0 text-muted-foreground opacity-0 hover:bg-red-50 hover:text-red-600 group-hover:opacity-100"
                        onClick={() => removeChange(change.id)}
                      >
                        <Trash />
                      </Button>
                    </li>
                  )
                })}
              </ul>
            )}
          </TabsContent>

          {/* 项目记录 */}
          <TabsContent value="records" className="space-y-3">
            {!loading && comments.length === 0 && (
              <p className="text-xs text-muted-foreground">还没有记录。变更说明、问题、收款进度都能写在这儿。</p>
            )}

            <ul className="space-y-2.5">
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
                        onClick={() => removeComment(comment.id)}
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
              value={commentDraft}
              onChange={setCommentDraft}
              onSubmit={submitComment}
              placeholder="记一条变更说明 / 问题 / 收款进度…（⌘/Ctrl + Enter 发送）"
              quickInserts={COMMENT_TEMPLATES}
              submitLabel="记一条"
              minRows={3}
            />
          </TabsContent>
        </Tabs>
      )}

      <ReceivableForm
        open={formOpen}
        item={editing}
        defaultProjectId={project?.id}
        lockProject
        onClose={() => setFormOpen(false)}
        onSaved={async () => {
          await reloadReceivables()
          onChanged?.()
        }}
      />
    </Dialog>
  )
}
