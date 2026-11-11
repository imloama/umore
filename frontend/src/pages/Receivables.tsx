import { useEffect, useMemo, useState } from 'react'
import { AlertTriangle, CalendarClock, CheckCircle2, Plus, Wallet } from 'lucide-react'
import { projectApi, receivableApi } from '../services/api'
import type { Project, Receivable, ReceivableSummary } from '../types'
import { useI18n } from '@/i18n'
import PageHeader from '../components/PageHeader'
import ProjectDetail from '../components/ProjectDetail'
import ReceivableForm from '../components/ReceivableForm'
import ReceivableList from '../components/ReceivableList'
import { money } from '../components/receivableMeta'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { cn } from '@/lib/utils'

const EMPTY_SUMMARY: ReceivableSummary = {
  pending_count: 0,
  pending_amount: 0,
  overdue_count: 0,
  overdue_amount: 0,
  due_soon_count: 0,
  due_soon_amount: 0,
  done_count: 0,
  done_amount: 0
}

/**
 * 回款：待收的钱单独一张桌子。
 * 顶层按「未收 / 逾期 / 7 天内到期 / 已收」分 tab，
 * 想看某笔挂在哪个单上，点项目名进项目详情（下一级）。
 */
export default function Receivables() {
  const { t } = useI18n()
  const [items, setItems] = useState<Receivable[]>([])
  const [projects, setProjects] = useState<Project[]>([])
  const [summary, setSummary] = useState<ReceivableSummary>(EMPTY_SUMMARY)
  const [tab, setTab] = useState('pending')
  const [formOpen, setFormOpen] = useState(false)
  const [editing, setEditing] = useState<Receivable | null>(null)
  const [detail, setDetail] = useState<Project | null>(null)

  useEffect(() => {
    load()
  }, [])

  const load = async () => {
    try {
      const [list, sum, projectList] = await Promise.all([
        receivableApi.list(),
        receivableApi.summary(),
        projectApi.list()
      ])
      setItems(list || [])
      setSummary(sum || EMPTY_SUMMARY)
      setProjects(projectList || [])
    } catch (error) {
      console.error('Failed to load receivables:', error)
    }
  }

  const pending = useMemo(() => items.filter(i => i.status === 'pending'), [items])
  const overdue = useMemo(() => pending.filter(i => i.overdue), [pending])
  const dueSoon = useMemo(
    () => pending.filter(i => !i.overdue && !!i.due_date && (i.days_left ?? 99) <= 7),
    [pending]
  )
  const done = useMemo(() => items.filter(i => i.status === 'done'), [items])

  // 按项目看还欠多少：项目维度是第二层入口
  const byProject = useMemo(() => {
    const map = new Map<number, { project?: Project; name: string; amount: number; count: number; overdue: number; next?: string }>()
    pending.forEach(item => {
      const key = item.project_id ?? 0
      const row = map.get(key) || {
        project: projects.find(p => p.id === item.project_id),
        name: item.project_name || t('dashboard.unlinkedProject'),
        amount: 0,
        count: 0,
        overdue: 0
      }
      row.amount += item.amount
      row.count += 1
      if (item.overdue) row.overdue += item.amount
      if (item.due_date && (!row.next || item.due_date < row.next)) row.next = item.due_date
      map.set(key, row)
    })
    return [...map.entries()]
      .map(([id, row]) => ({ id, ...row, amount: Math.round(row.amount * 100) / 100 }))
      .sort((a, b) => b.amount - a.amount)
  }, [pending, projects, t])

  const toggleDone = async (item: Receivable) => {
    if (!item.id) return
    try {
      await receivableApi.setDone(item.id, item.status !== 'done')
      load()
    } catch (error) {
      console.error('Failed to toggle receivable:', error)
    }
  }

  const remove = async (item: Receivable) => {
    if (!item.id) return
    if (!confirm(t('receivable.confirmDelete', { title: item.title }))) return
    await receivableApi.remove(item.id)
    load()
  }

  const openProject = async (projectId?: number) => {
    if (!projectId) return
    const local = projects.find(p => p.id === projectId)
    if (local) {
      setDetail(local)
      return
    }
    try {
      setDetail(await projectApi.get(projectId))
    } catch (error) {
      console.error('Failed to open project:', error)
    }
  }

  const listProps = {
    onEdit: (item: Receivable) => {
      setEditing(item)
      setFormOpen(true)
    },
    onToggleDone: toggleDone,
    onDelete: remove,
    onOpenProject: openProject
  }

  return (
    <div>
      <PageHeader
        eyebrow="RECEIVABLES"
        title={t('receivable.title')}
        description={t('receivable.description')}
        actions={
          <Button
            onClick={() => {
              setEditing(null)
              setFormOpen(true)
            }}
          >
            <Plus />
            {t('receivable.new')}
          </Button>
        }
      />

      <div className="mb-4 grid gap-3 sm:grid-cols-3">
        <button type="button" onClick={() => setTab('pending')} className="text-left">
          <Card className={cn('px-4 py-3.5 transition-colors hover:border-primary/40', tab === 'pending' && 'border-primary/40')}>
            <div className="flex items-center justify-between text-[11px] text-muted-foreground">
              <span>{t('receivable.pending')}</span>
              <Wallet className="size-4 text-amber-500" />
            </div>
            <div className="tabular mt-1.5 text-2xl font-semibold text-amber-700">{money(summary.pending_amount)}</div>
            <div className="mt-0.5 text-[11px] text-muted-foreground">{t('receivable.pendingCount', { n: summary.pending_count })}</div>
          </Card>
        </button>

        <button type="button" onClick={() => setTab('overdue')} className="text-left">
          <Card
            className={cn(
              'px-4 py-3.5 transition-colors hover:border-red-300',
              summary.overdue_count > 0 && 'border-red-200 bg-red-50/40',
              tab === 'overdue' && 'border-red-300'
            )}
          >
            <div className="flex items-center justify-between text-[11px] text-muted-foreground">
              <span>{t('receivable.overdue')}</span>
              <AlertTriangle className={cn('size-4', summary.overdue_count > 0 ? 'text-red-500' : 'text-muted-foreground')} />
            </div>
            <div className={cn('tabular mt-1.5 text-2xl font-semibold', summary.overdue_count > 0 ? 'text-red-600' : '')}>
              {money(summary.overdue_amount)}
            </div>
            <div className="mt-0.5 text-[11px] text-muted-foreground">
              {summary.overdue_count > 0
                ? t('receivable.overdueCount', { n: summary.overdue_count })
                : t('receivable.noOverdue')}
            </div>
          </Card>
        </button>

        <button type="button" onClick={() => setTab('soon')} className="text-left">
          <Card className={cn('px-4 py-3.5 transition-colors hover:border-amber-300', tab === 'soon' && 'border-amber-300')}>
            <div className="flex items-center justify-between text-[11px] text-muted-foreground">
              <span>{t('receivable.dueSoon')}</span>
              <CalendarClock className="size-4 text-primary" />
            </div>
            <div className="tabular mt-1.5 text-2xl font-semibold">{money(summary.due_soon_amount)}</div>
            <div className="mt-0.5 text-[11px] text-muted-foreground">{t('receivable.dueSoonCount', { n: summary.due_soon_count })}</div>
          </Card>
        </button>
      </div>

      <Card className="mb-4 px-4 py-3">
        <Tabs value={tab} onValueChange={setTab}>
          <TabsList>
            <TabsTrigger value="pending">{t('receivable.tab.pending', { n: pending.length })}</TabsTrigger>
            <TabsTrigger value="overdue">{t('receivable.tab.overdue', { n: overdue.length })}</TabsTrigger>
            <TabsTrigger value="soon">{t('receivable.tab.soon', { n: dueSoon.length })}</TabsTrigger>
            <TabsTrigger value="done">{t('receivable.tab.done', { n: done.length })}</TabsTrigger>
            <TabsTrigger value="projects">{t('receivable.tab.byProject')}</TabsTrigger>
          </TabsList>

          <TabsContent value="pending">
            <ReceivableList items={pending} {...listProps} />
          </TabsContent>
          <TabsContent value="overdue">
            <ReceivableList items={overdue} {...listProps} />
          </TabsContent>
          <TabsContent value="soon">
            <ReceivableList items={dueSoon} {...listProps} />
          </TabsContent>
          <TabsContent value="done">
            <ReceivableList items={done} {...listProps} />
          </TabsContent>
          <TabsContent value="projects">
            {byProject.length === 0 ? (
              <div className="rounded-lg border border-dashed border-border px-4 py-10 text-center text-xs text-muted-foreground">
                {t('receivable.emptyByProject')}
              </div>
            ) : (
              <ul className="divide-y divide-border overflow-hidden rounded-lg border border-border">
                {byProject.map(row => (
                  <li key={row.id} className="flex items-center gap-3 px-3.5 py-2.5 hover:bg-secondary/40">
                    <CheckCircle2 className={cn('size-4 shrink-0', row.overdue > 0 ? 'text-red-500' : 'text-amber-500')} />
                    <button
                      type="button"
                      className="min-w-0 flex-1 text-left disabled:cursor-default"
                      disabled={!row.id}
                      onClick={() => openProject(row.id || undefined)}
                    >
                      <div className="truncate text-[13px] font-medium hover:text-primary disabled:hover:text-foreground">
                        {row.name}
                      </div>
                      <div className="mt-0.5 text-[11px] text-muted-foreground">
                        {t('receivable.count', { n: row.count })}
                        {row.overdue > 0 && (
                          <span className="ml-1 text-red-600">{t('receivable.overdueInline', { amount: money(row.overdue) })}</span>
                        )}
                        {row.next && <span className="ml-1">{t('receivable.nextDue', { date: row.next })}</span>}
                        {row.project && (
                          <span className="ml-1">
                            {t('receivable.contractLine', {
                              amount: money(row.project.amount),
                              received: money(row.project.received)
                            })}
                          </span>
                        )}
                      </div>
                    </button>
                    <span className="tabular shrink-0 text-[13px] font-semibold text-amber-700">{money(row.amount)}</span>
                    {row.project && (
                      <Button variant="outline" size="xs" className="shrink-0" onClick={() => openProject(row.id)}>
                        {t('receivable.enterProject')}
                      </Button>
                    )}
                  </li>
                ))}
              </ul>
            )}
          </TabsContent>
        </Tabs>
      </Card>

      {items.length === 0 && (
        <p className="text-center text-[11px] text-muted-foreground">{t('receivable.footer')}</p>
      )}

      <ReceivableForm
        open={formOpen}
        item={editing}
        projects={projects}
        onClose={() => setFormOpen(false)}
        onSaved={() => load()}
      />

      {/* 下一级：点项目名进项目详情，里面还有自己的 tab */}
      <ProjectDetail
        project={detail}
        onClose={() => setDetail(null)}
        onChanged={async () => {
          load()
          if (detail?.id) {
            try {
              setDetail(await projectApi.get(detail.id))
            } catch (error) {
              console.error('Failed to refresh project:', error)
            }
          }
        }}
      />
    </div>
  )
}
