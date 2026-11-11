import { useEffect, useMemo, useState } from 'react'
import { useSearchParams } from 'react-router-dom'
import { CalendarClock, Coins, FolderKanban, Pencil, Plus, ScrollText, Search, Timer, Trash, Wallet } from 'lucide-react'
import { errorCodeOf, projectApi } from '../services/api'
import type { Project } from '../types'
import { useI18n } from '@/i18n'
import PageHeader from '../components/PageHeader'
import ProjectDetail from '../components/ProjectDetail'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { Dialog } from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Select } from '@/components/ui/select'
import { Separator } from '@/components/ui/separator'
import { Textarea } from '@/components/ui/textarea'
import { cn } from '@/lib/utils'
import { formatHours } from '../components/taskMeta'
import { money, sumAmounts } from '../components/receivableMeta'

const emptyForm = (): Partial<Project> => ({
  name: '',
  description: '',
  git_repo: '',
  customer: '',
  status: 'active',
  amount: 0
})

export default function Projects() {
  const { t, te } = useI18n()
  const [projects, setProjects] = useState<Project[]>([])
  const [showForm, setShowForm] = useState(false)
  const [form, setForm] = useState<Partial<Project>>(emptyForm())
  const [keyword, setKeyword] = useState('')
  const [showArchived, setShowArchived] = useState(false)
  const [detail, setDetail] = useState<Project | null>(null)
  const [detailTab, setDetailTab] = useState('overview')
  // 编辑金额时顺手填变更说明，后端会落一条金额变更记录
  const [amountReason, setAmountReason] = useState('')
  const [formError, setFormError] = useState('')

  const [searchParams, setSearchParams] = useSearchParams()

  useEffect(() => {
    loadProjects()
  }, [])

  // 别处（灵感卡片、快捷入口）带 ?open=<id> 跳进来时直接开详情，看过就把参数抹掉
  useEffect(() => {
    const openId = Number(searchParams.get('open'))
    if (!openId) return
    const target = projects.find(p => p.id === openId)
    if (target) {
      setDetail(target)
      setDetailTab('overview')
    }
    const next = new URLSearchParams(searchParams)
    next.delete('open')
    setSearchParams(next, { replace: true })
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [searchParams, projects])

  const loadProjects = async () => {
    try {
      const data = await projectApi.list()
      setProjects(data || [])
      setDetail(prev => (prev ? data?.find(p => p.id === prev.id) ?? prev : prev))
    } catch (error) {
      console.error('Failed to load projects:', error)
    }
  }

  const submit = async (e: React.FormEvent) => {
    e.preventDefault()
    try {
      if (form.id) {
        await projectApi.update(form.id, form, amountReason.trim())
      } else {
        await projectApi.create(form)
      }
      setShowForm(false)
      setForm(emptyForm())
      setAmountReason('')
      setFormError('')
      loadProjects()
    } catch (error) {
      console.error('Failed to save project:', error)
      const { code } = errorCodeOf(error)
      setFormError(code ? te(code) : t('common.saveFailed'))
    }
  }

  const edit = (project: Project) => {
    setForm({ ...project })
    setAmountReason('')
    setFormError('')
    setShowForm(true)
  }

  const remove = async (id: number) => {
    if (!confirm(t('project.confirmDelete'))) return
    try {
      await projectApi.remove(id)
      loadProjects()
    } catch (error) {
      console.error('Failed to delete project:', error)
    }
  }

  const filtered = useMemo(() => {
    const kw = keyword.trim().toLowerCase()
    return projects
      .filter(p => {
        if (!showArchived && p.status === 'archived') return false
        if (!kw) return true
        return `${p.name} ${p.customer || ''} ${p.description || ''}`.toLowerCase().includes(kw)
      })
      // 合同金额大的排前面，没填金额的按更新时间垫底
      .sort((a, b) => {
        const gap = (b.amount || 0) - (a.amount || 0)
        if (gap !== 0) return gap
        return (b.updated_at || '').localeCompare(a.updated_at || '')
      })
  }, [projects, keyword, showArchived])

  const totalHours = filtered.reduce((s, p) => s + (p.actual_hours || 0), 0)
  const totalTasks = filtered.reduce((s, p) => s + (p.task_count || 0), 0)
  // 合计走分级累加，别让浮点尾巴跑进合计
  const totalAmount = sumAmounts(filtered.map(p => p.amount || 0))
  const totalReceived = sumAmounts(filtered.map(p => p.received || 0))
  const totalPending = sumAmounts(filtered.map(p => p.receivable_amount || 0))
  // 未收是所有项目各自「合同 - 已收」的和：某个项目收多了不拿去抵另一个项目的欠款
  const totalUnpaid = sumAmounts(filtered.map(p => Math.max(0, (p.amount || 0) - (p.received || 0))))
  const archivedCount = projects.filter(p => p.status === 'archived').length

  return (
    <div>
      <PageHeader
        eyebrow="PROJECTS"
        title={t('project.title')}
        description={t('project.description')}
        actions={
          <>
            {archivedCount > 0 && (
              <Button variant="outline" size="sm" onClick={() => setShowArchived(v => !v)}>
                {showArchived ? t('project.hideArchived') : t('project.showArchived', { n: archivedCount })}
              </Button>
            )}
            <Button onClick={() => { setForm(emptyForm()); setShowForm(true) }}>
              <Plus />
              {t('project.new')}
            </Button>
          </>
        }
      />

      <div className="mb-4 grid gap-3 sm:grid-cols-2 xl:grid-cols-3">
        <Card className="px-4 py-3">
          <div className="text-[11px] text-muted-foreground">{t('project.count')}</div>
          <div className="tabular mt-1 text-2xl font-semibold">{filtered.length}</div>
        </Card>
        <Card className="px-4 py-3">
          <div className="text-[11px] text-muted-foreground">{t('project.linkedTasks')}</div>
          <div className="tabular mt-1 text-2xl font-semibold">{totalTasks}</div>
        </Card>
        <Card className="border-amber-200 bg-amber-50/40 px-4 py-3">
          <div className="flex items-center gap-1 text-[11px] text-amber-700">
            <Timer className="size-3" />
            {t('project.invested')}
          </div>
          <div className="tabular mt-1 text-2xl font-semibold text-amber-700">{formatHours(totalHours) || '0h'}</div>
        </Card>
        <Card className="px-4 py-3">
          <div className="flex items-center gap-1 text-[11px] text-muted-foreground">
            <Coins className="size-3" />
            {t('project.contractTotal')}
          </div>
          <div className="tabular mt-1 text-2xl font-semibold">{money(totalAmount)}</div>
        </Card>
        <Card className="border-emerald-200 bg-emerald-50/40 px-4 py-3">
          <div className="flex items-center gap-1 text-[11px] text-emerald-700">
            <Wallet className="size-3" />
            {t('project.received')}
          </div>
          <div className="tabular mt-1 text-2xl font-semibold text-emerald-700">{money(totalReceived)}</div>
          <div className="mt-0.5 text-[10.5px] text-emerald-700/80">
            {t('project.unreceived', { amount: money(totalUnpaid) })}
          </div>
        </Card>
        <Card className="border-amber-200 bg-amber-50/40 px-4 py-3">
          <div className="flex items-center gap-1 text-[11px] text-amber-700">
            <CalendarClock className="size-3" />
            {t('project.receivablePlan')}
          </div>
          <div className="tabular mt-1 text-2xl font-semibold text-amber-700">{money(totalPending)}</div>
          <div className="mt-0.5 text-[10.5px] text-amber-700/80">{t('project.receivablePlanHint')}</div>
        </Card>
      </div>

      <Card className="mb-4">
        <div className="flex items-center gap-2 px-3 py-2.5">
          <div className="relative flex-1">
            <Search className="pointer-events-none absolute left-2.5 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
            <Input
              placeholder={t('project.searchPlaceholder')}
              value={keyword}
              onChange={e => setKeyword(e.target.value)}
              className="pl-8"
            />
          </div>
        </div>
      </Card>

      {filtered.length === 0 ? (
        <Card className="flex flex-col items-center justify-center gap-2 px-6 py-14 text-center">
          <FolderKanban className="size-6 text-muted-foreground/60" />
          <div className="text-sm font-medium">{t('project.empty')}</div>
          <div className="text-xs text-muted-foreground">{t('project.emptyHint')}</div>
        </Card>
      ) : (
        <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-3">
          {filtered.map(project => {
            const est = project.estimated_hours || 0
            const act = project.actual_hours || 0
            const over = est > 0 && act > est
            const ratio = est > 0 ? Math.min(100, Math.round((act / est) * 100)) : 0
            return (
              <Card key={project.id} className="group flex flex-col p-4">
                <div className="flex items-start gap-2">
                  <div className="min-w-0 flex-1">
                    <div className="flex items-center gap-1.5">
                      <button
                        type="button"
                        onClick={() => {
                          setDetailTab('overview')
                          setDetail(project)
                        }}
                        className="truncate text-left text-[14px] font-semibold hover:text-primary"
                        title={t('project.detail.overview')}
                      >
                        {project.name}
                      </button>
                      {project.status === 'archived' && <Badge variant="muted">{t('project.archived')}</Badge>}
                    </div>
                    <div className="mt-0.5 truncate text-[11px] text-muted-foreground">
                      {project.customer || t('project.noCustomer')}
                    </div>
                  </div>
                  <div className="flex shrink-0 gap-0.5 opacity-0 transition-opacity group-hover:opacity-100">
                    <Button variant="ghost" size="icon-sm" onClick={() => edit(project)} title={t('common.edit')}>
                      <Pencil />
                    </Button>
                    <Button
                      variant="ghost"
                      size="icon-sm"
                      className="text-muted-foreground hover:bg-red-50 hover:text-red-600"
                      onClick={() => remove(project.id!)}
                      title={t('common.delete')}
                    >
                      <Trash />
                    </Button>
                  </div>
                </div>

                {project.description && (
                  <p className="mt-2 line-clamp-2 text-[12px] leading-relaxed text-muted-foreground">
                    {project.description}
                  </p>
                )}

                <div className="mt-3">
                  <div className="flex items-baseline justify-between">
                    <span className="flex items-center gap-1 text-[11px] text-muted-foreground">
                      <Timer className="size-3" />
                      {t('project.invested')}
                    </span>
                    <span className="tabular text-[13px] font-semibold text-amber-700">
                      {formatHours(act) || '0h'}
                      {est > 0 && (
                        <span className="ml-1 text-[11px] font-normal text-muted-foreground">
                          {t('project.estimateLabel', { hours: formatHours(est) })}
                        </span>
                      )}
                    </span>
                  </div>
                  <div className="mt-1.5 h-1.5 overflow-hidden rounded-full bg-secondary">
                    <div
                      className={cn('h-full rounded-full', over ? 'bg-red-500' : 'bg-amber-500')}
                      style={{ width: `${ratio}%` }}
                    />
                  </div>
                  <div className="mt-1.5 flex items-center justify-between text-[11px] text-muted-foreground">
                    <span>{t('project.taskCount', { n: project.task_count || 0 })}</span>
                    {est > 0 && (
                      <span className={cn('tabular', over && 'font-medium text-red-600')}>
                        {t('project.usedPercent', { n: Math.round((act / est) * 100) })}
                      </span>
                    )}
                  </div>
                </div>

                <div className="mt-2.5 flex items-center justify-between rounded-md bg-secondary/40 px-2 py-1.5">
                  <span className="text-[11px] text-muted-foreground">
                    {project.amount ? t('project.contractLabel', { amount: money(project.amount) }) : t('project.contractUnset')}
                    {!!project.amount_change_count && (
                      <span className="ml-1 text-[10px]">{t('project.changeCount', { n: project.amount_change_count })}</span>
                    )}
                  </span>
                  <span className="flex items-center gap-1.5">
                    <span
                      className={cn(
                        'tabular text-[11px] font-medium',
                        (project.received || 0) > 0 ? 'text-emerald-600' : 'text-muted-foreground'
                      )}
                    >
                      {t('project.receivedLine', { amount: money(project.received) })}
                    </span>
                    {!!project.receivable_count && (
                      <span
                        className={cn(
                          'tabular text-[11px] font-medium',
                          project.receivable_overdue_amount ? 'text-red-600' : 'text-amber-600'
                        )}
                      >
                        {project.receivable_overdue_amount
                          ? t('project.overdueLine', { amount: money(project.receivable_amount) })
                          : t('project.pendingLine', { amount: money(project.receivable_amount) })}
                      </span>
                    )}
                  </span>
                </div>

                <div className="mt-2 flex gap-1.5">
                  <Button
                    variant="outline"
                    size="xs"
                    className="flex-1 justify-center"
                    onClick={() => {
                      setDetailTab('receivables')
                      setDetail(project)
                    }}
                  >
                    <CalendarClock />
                    {t('project.receivableButton')}
                    {!!project.receivable_count && (
                      <span className="tabular ml-0.5 text-amber-600">{project.receivable_count}</span>
                    )}
                  </Button>
                  <Button
                    variant="outline"
                    size="xs"
                    className="flex-1 justify-center"
                    onClick={() => {
                      setDetailTab('changes')
                      setDetail(project)
                    }}
                  >
                    <ScrollText />
                    {t('project.changesButton')}
                  </Button>
                </div>

                <Separator className="my-3" />
                <div className="flex items-center gap-2 text-[11px] text-muted-foreground">
                  {project.git_repo ? (
                    <a
                      href={project.git_repo}
                      target="_blank"
                      rel="noopener noreferrer"
                      className="truncate text-primary hover:underline"
                    >
                      {project.git_repo.replace(/^https?:\/\//, '')}
                    </a>
                  ) : (
                    <span>{t('project.noRepo')}</span>
                  )}
                </div>
              </Card>
            )
          })}
        </div>
      )}

      <Dialog
        open={showForm}
        onClose={() => { setShowForm(false); setForm(emptyForm()) }}
        title={form.id ? t('project.editTitle') : t('project.new')}
        description={t('project.formDescription')}
        footer={
          <>
            <Button variant="outline" onClick={() => { setShowForm(false); setForm(emptyForm()) }}>{t('common.cancel')}</Button>
            <Button type="submit" form="project-form">{t('common.save')}</Button>
          </>
        }
      >
        <form id="project-form" onSubmit={submit} className="space-y-3.5">
          <div className="grid gap-3 sm:grid-cols-2">
            <div>
              <Label>{t('project.field.name')}</Label>
              <Input value={form.name || ''} onChange={e => setForm({ ...form, name: e.target.value })} required />
            </div>
            <div>
              <Label>{t('project.field.customer')}</Label>
              <Input value={form.customer || ''} onChange={e => setForm({ ...form, customer: e.target.value })} />
            </div>
          </div>
          <div>
            <Label>{t('project.field.description')}</Label>
            <Textarea rows={2} value={form.description || ''} onChange={e => setForm({ ...form, description: e.target.value })} />
          </div>
          <div className="grid gap-3 sm:grid-cols-2">
            <div>
              <Label>{t('project.field.amount')}</Label>
              <Input
                type="number"
                step="0.01"
                min="0"
                value={form.amount ?? 0}
                onChange={e => setForm({ ...form, amount: parseFloat(e.target.value) || 0 })}
                placeholder="0.00"
              />
            </div>
            <div>
              <Label>{t('project.field.status')}</Label>
              <Select value={form.status} onChange={e => setForm({ ...form, status: e.target.value as Project['status'] })}>
                <option value="active">{t('project.status.active')}</option>
                <option value="archived">{t('project.status.archived')}</option>
              </Select>
            </div>
          </div>
          <div>
            <Label>{t('project.field.repo')}</Label>
            <Input
              value={form.git_repo || ''}
              onChange={e => setForm({ ...form, git_repo: e.target.value })}
              placeholder="https://github.com/..."
            />
          </div>
          {form.id ? (
            <div>
              <Label>{t('project.field.amountReason')}</Label>
              <Input
                value={amountReason}
                onChange={e => setAmountReason(e.target.value)}
                placeholder={t('project.field.amountReasonPlaceholder')}
              />
            </div>
          ) : null}
          {formError && <div className="text-[11.5px] text-red-600">{formError}</div>}
        </form>
      </Dialog>

      <ProjectDetail
        project={detail}
        onClose={() => setDetail(null)}
        onChanged={loadProjects}
        initialTab={detailTab}
      />
    </div>
  )
}
