import { useEffect, useState } from 'react'
import { ArrowDownRight, ArrowUpRight, Plus, Trash, Wallet } from 'lucide-react'
import { projectApi, receivableApi } from '../services/api'
import type { Project, ProjectAmountChange, Receivable, TaskComment } from '../types'
import RichText from './RichText'
import RichTextEditor, { type QuickInsert } from './RichTextEditor'
import ReceivableForm from './ReceivableForm'
import ReceivableList from './ReceivableList'
import { useI18n } from '@/i18n'
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
  const { t } = useI18n()
  const commentTemplates: QuickInsert[] = [
    { label: t('project.detail.tpl.change'), text: t('project.detail.tpl.changeText') },
    { label: t('project.detail.tpl.issue'), text: t('project.detail.tpl.issueText') },
    { label: t('project.detail.tpl.receipt'), text: t('project.detail.tpl.receiptText') }
  ]
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
  const [fresh, setFresh] = useState<Project | null>(null)

  useEffect(() => {
    setFresh(null)
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

  // 弹窗里的 project 是父页面的快照，收款变化后单独取一份最新的，概览才跟着动
  const reloadProject = async () => {
    if (!project?.id) return
    try {
      setFresh(await projectApi.get(project.id))
    } catch (err) {
      console.error('Failed to reload project:', err)
    }
  }

  const submitAmount = async () => {
    if (!project?.id) return
    const next = parseFloat(amountDraft)
    if (!Number.isFinite(next) || next < 0) {
      setError(t('project.detail.amountInvalid'))
      return
    }
    if (next === (project.amount || 0)) {
      setError(t('project.detail.amountUnchanged'))
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
      setFresh(reloaded)
      setAmountDraft(draftOf(reloaded.amount))
      setReasonDraft('')
      onChanged?.()
    } catch (err) {
      console.error('Failed to add amount change:', err)
      setError(t('project.detail.saveFailed'))
    } finally {
      setSaving(false)
    }
  }

  const removeChange = async (changeId?: number) => {
    if (!changeId || !project?.id) return
    if (!confirm(t('project.detail.confirmDeleteChange'))) return
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

  // 勾已收会同时落一笔收入流水，项目的「已收款」得重新拉一次，
  // 只刷收款计划列表的话概览上的数字还停在旧值。
  const toggleDone = async (item: Receivable) => {
    if (!item.id) return
    await receivableApi.setDone(item.id, item.status !== 'done')
    await Promise.all([reloadReceivables(), reloadProject()])
    onChanged?.()
  }

  const removeReceivable = async (item: Receivable) => {
    if (!item.id) return
    if (!confirm(t('project.detail.confirmDeleteReceivable', { title: item.title }))) return
    await receivableApi.remove(item.id)
    await Promise.all([reloadReceivables(), reloadProject()])
    onChanged?.()
  }

  // 汇总字段用刚拉回来的那份，其余仍听父页面传的 project
  const view = fresh && fresh.id === project?.id ? fresh : project
  const amount = view?.amount || 0
  const received = view?.received || 0
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
            {project.status === 'archived' && <Badge variant="muted">{t('project.archived')}</Badge>}
            {project.customer && <Badge variant="outline">{project.customer}</Badge>}
          </span>
        )
      }
      description={project?.description || t('project.detail.description')}
      footer={<Button variant="outline" onClick={onClose}>{t('common.close')}</Button>}
    >
      {project && (
        <Tabs value={tab} onValueChange={setTab}>
          <TabsList>
            <TabsTrigger value="overview">{t('project.detail.overview')}</TabsTrigger>
            <TabsTrigger value="receivables">
              {t('project.detail.receivables')}
              <span className="text-[11px] font-normal text-muted-foreground">{pendingReceivable.length}</span>
            </TabsTrigger>
            <TabsTrigger value="changes">
              {t('project.detail.changes')}
              <span className="text-[11px] font-normal text-muted-foreground">{changes.length}</span>
            </TabsTrigger>
            <TabsTrigger value="records">
              {t('project.detail.records')}
              <span className="text-[11px] font-normal text-muted-foreground">{comments.length}</span>
            </TabsTrigger>
          </TabsList>

          {/* 概览 */}
          <TabsContent value="overview" className="space-y-4">
            <div className="grid gap-2 sm:grid-cols-3">
              <div className="rounded-lg border border-border px-3 py-2.5">
                <div className="text-[11px] text-muted-foreground">{t('project.detail.contractAmount')}</div>
                <div className="tabular mt-1 text-[17px] font-semibold">{money(amount)}</div>
                {!!view?.amount_change_count && (
                  <div className="mt-0.5 text-[10.5px] text-muted-foreground">
                    {t('project.changeCount', { n: view.amount_change_count })}
                    {view.amount_changed_at && ` · ${formatTime(view.amount_changed_at)}`}
                  </div>
                )}
              </div>
              <div className="rounded-lg border border-emerald-200 bg-emerald-50/40 px-3 py-2.5">
                <div className="flex items-center gap-1 text-[11px] text-emerald-700">
                  <Wallet className="size-3" />
                  {t('project.detail.receivedAmount')}
                </div>
                <div className="tabular mt-1 text-[17px] font-semibold text-emerald-700">{money(received)}</div>
                <div className="mt-0.5 text-[10.5px] text-emerald-700/80">
                  {t('project.detail.receivedRatio', { n: amount > 0 ? Math.round((received / amount) * 100) : 0 })}
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
                  {overReceived ? t('project.detail.overReceived') : t('project.detail.unreceived')}
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
                  {t('project.detail.hoursLine', {
                    actual: formatHours(view?.actual_hours) || '0h',
                    estimate: formatHours(view?.estimated_hours) || '—'
                  })}
                </div>
              </div>
            </div>

            <div className="grid gap-2 sm:grid-cols-2">
              <div className="rounded-lg border border-border px-3 py-2.5">
                <div className="flex items-center justify-between text-[11px] text-muted-foreground">
                  <span>{t('project.detail.pendingPlan')}</span>
                  <Button variant="ghost" size="xs" onClick={() => setTab('receivables')}>{t('project.detail.goSee')}</Button>
                </div>
                <div className="tabular mt-1 text-[15px] font-semibold">{money(pendingAmount)}</div>
                <div className="mt-0.5 text-[10.5px] text-muted-foreground">
                  {t('project.detail.pendingCount', { n: pendingReceivable.length })}
                  {overdueAmount > 0 && (
                    <span className="ml-1 text-red-600">{t('project.detail.overdueAmount', { amount: money(overdueAmount) })}</span>
                  )}
                </div>
              </div>
              <div className="rounded-lg border border-border px-3 py-2.5">
                <div className="text-[11px] text-muted-foreground">{t('project.linkedTasks')}</div>
                <div className="tabular mt-1 text-[15px] font-semibold">
                  {t('project.detail.linkedTaskCount', { n: view?.task_count || 0 })}
                </div>
                <div className="mt-0.5 text-[10.5px] text-muted-foreground">
                  {t('project.detail.investedHours', { hours: formatHours(view?.actual_hours) || '0h' })}
                </div>
              </div>
            </div>

            {loading && <p className="text-xs text-muted-foreground">{t('common.loading')}</p>}
          </TabsContent>

          {/* 收款计划：下一级就是每一笔的编辑 */}
          <TabsContent value="receivables" className="space-y-3">
            <div className="flex items-center justify-between">
              <div className="text-[12px] text-muted-foreground">
                {t('project.detail.unreceived')}{' '}
                <span className="tabular font-semibold text-amber-700">{money(pendingAmount)}</span>
                {overdueAmount > 0 && (
                  <span className="ml-2 text-red-600">
                    {t('project.detail.overdueAmount', { amount: money(overdueAmount) })}
                  </span>
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
                {t('project.detail.addReceivable')}
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
              {t('project.detail.receivableHint')}
            </p>
          </TabsContent>

          {/* 金额变更 */}
          <TabsContent value="changes" className="space-y-3">
            <div className="rounded-lg border border-border bg-secondary/30 p-2.5">
              <div className="grid gap-2 sm:grid-cols-[130px_1fr_auto]">
                <div>
                  <Label className="text-[11px]">{t('project.detail.adjustAmount')}</Label>
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
                  <Label className="text-[11px]">{t('project.detail.changeReason')}</Label>
                  <Input
                    value={reasonDraft}
                    onChange={e => setReasonDraft(e.target.value)}
                    placeholder={t('project.detail.changeReasonPlaceholder')}
                    className="h-8"
                  />
                </div>
                <div className="flex items-end">
                  <Button size="sm" onClick={submitAmount} disabled={saving}>
                    <Plus />
                    {t('project.detail.addChange')}
                  </Button>
                </div>
              </div>
              {error && <div className="mt-1.5 text-[11px] text-red-600">{error}</div>}
            </div>

            {changes.length === 0 ? (
              <p className="text-xs text-muted-foreground">{t('project.detail.noChanges')}</p>
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
                        <div className="truncate font-medium">{change.reason || t('project.detail.manualAdjust')}</div>
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
              <p className="text-xs text-muted-foreground">{t('project.detail.noRecords')}</p>
            )}

            <ul className="space-y-2.5">
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
                        onClick={() => removeComment(comment.id)}
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
              value={commentDraft}
              onChange={setCommentDraft}
              onSubmit={submitComment}
              placeholder={t('project.detail.recordPlaceholder')}
              quickInserts={commentTemplates}
              submitLabel={t('project.detail.addRecord')}
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
          await Promise.all([reloadReceivables(), reloadProject()])
          onChanged?.()
        }}
      />
    </Dialog>
  )
}
