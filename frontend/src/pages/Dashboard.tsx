import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { AlertTriangle, ArrowRight, Briefcase, CalendarClock, CircleCheckBig, FolderKanban, Timer, TrendingDown, TrendingUp, Wallet } from 'lucide-react'
import api, { receivableApi, taskApi } from '../services/api'
import type { Project, Receivable, ReceivableSummary, Task, TaskStats } from '../types'
import { useI18n } from '../i18n'
import PageHeader from '../components/PageHeader'
import HoursDelta from '../components/HoursDelta'
import TaskDetail from '../components/TaskDetail'
import TaskRow from '../components/TaskRow'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { cn } from '@/lib/utils'
import { CATEGORY_KEYS, CATEGORY_ORDER, CATEGORY_STYLE, formatHours } from '../components/taskMeta'

const EMPTY_RECEIVABLE: ReceivableSummary = {
  pending_count: 0,
  pending_amount: 0,
  overdue_count: 0,
  overdue_amount: 0,
  due_soon_count: 0,
  due_soon_amount: 0,
  done_count: 0,
  done_amount: 0
}

const EMPTY_STATS: TaskStats = {
  by_category: {},
  by_status: {},
  today_total: 0,
  today_done: 0,
  today_estimated_hours: 0,
  today_actual_hours: 0
}

export default function Dashboard() {
  const { t } = useI18n()
  const [stats, setStats] = useState<TaskStats>(EMPTY_STATS)
  const [todayTasks, setTodayTasks] = useState<Task[]>([])
  const [projects, setProjects] = useState<Project[]>([])
  const [monthlyBalance, setMonthlyBalance] = useState(0)
  const [receivableSum, setReceivableSum] = useState<ReceivableSummary>(EMPTY_RECEIVABLE)
  const [urgentReceivables, setUrgentReceivables] = useState<Receivable[]>([])
  const [detail, setDetail] = useState<Task | null>(null)

  useEffect(() => {
    loadStats()
  }, [])

  const loadStats = async () => {
    try {
      const today = new Date().toISOString().split('T')[0]
      const firstDay = new Date(new Date().getFullYear(), new Date().getMonth(), 1).toISOString().split('T')[0]

      const [taskStats, todayList, projectList, finances, receivableList, receivableSummary] = await Promise.all([
        taskApi.stats(),
        taskApi.list({ today: true }),
        api.get<any, Project[]>('/projects'),
        api.get<any, any>('/finances/summary', { params: { start_date: firstDay, end_date: today } }),
        receivableApi.list({ status: 'pending' }),
        receivableApi.summary()
      ])

      setStats(taskStats)
      setTodayTasks(todayList || [])
      setProjects(projectList || [])
      setMonthlyBalance(finances?.balance || 0)
      setReceivableSum(receivableSummary || EMPTY_RECEIVABLE)
      // 首页只放「该催的」：逾期的排前面，其次 7 天内到期
      setUrgentReceivables(
        (receivableList || [])
          .filter(item => item.overdue || (item.due_date && (item.days_left ?? 99) <= 7))
          .sort((a, b) => (a.due_date || '').localeCompare(b.due_date || ''))
          .slice(0, 4)
      )
    } catch (error) {
      console.error('Failed to load stats:', error)
    }
  }

  const progress = stats.today_total ? Math.round((stats.today_done / stats.today_total) * 100) : 0
  const activeProjects = projects.filter(p => p.status === 'active')
  const totalHours = projects.reduce((s, p) => s + (p.actual_hours || 0), 0)
  const pending = (stats.by_status.DOING || 0) + (stats.by_status.TODO || 0) + (stats.by_status.BLOCKED || 0)

  return (
    <div>
      <PageHeader
        eyebrow="WORKSPACE"
        title={t('dashboard.title')}
        description={t('dashboard.description')}
        actions={
          <Button asChild size="sm" variant="outline">
            <Link to="/today">
              {t('dashboard.enterToday')}
              <ArrowRight />
            </Link>
          </Button>
        }
      />

      <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-5">
        {/* 主卡：今日进度 */}
        <div className="relative overflow-hidden rounded-lg bg-gradient-to-br from-primary to-blue-600 px-4 py-3.5 text-white shadow-sm sm:col-span-2 xl:col-span-1">
          <div className="flex items-center justify-between text-[11px] text-blue-100">
            <span>{t('dashboard.todayProgress')}</span>
            <CircleCheckBig className="size-4" />
          </div>
          <div className="tabular mt-2 text-3xl font-semibold">
            {stats.today_done}
            <span className="ml-1 text-sm font-normal text-blue-100">/ {stats.today_total}</span>
          </div>
          <div className="mt-1 text-[11px] text-blue-100">{t('dashboard.progressRate', { n: progress })}</div>
          <div className="mt-3 h-1.5 overflow-hidden rounded-full bg-white/25">
            <div className="h-full rounded-full bg-white transition-all" style={{ width: `${progress}%` }} />
          </div>
        </div>

        <Card>
          <CardHeader className="pb-1">
            <div className="flex items-center justify-between text-[11px] text-muted-foreground">
              <span>{t('dashboard.todayHours')}</span>
              <Timer className="size-4 text-amber-500" />
            </div>
          </CardHeader>
          <CardContent>
            <div className="tabular text-2xl font-semibold">
              {formatHours(stats.today_actual_hours) || '0h'}
              <span className="ml-1 text-xs font-normal text-muted-foreground">
                / {formatHours(stats.today_estimated_hours) || t('task.stats.notEstimated')}
              </span>
            </div>
            <div className="mt-1.5">
              <HoursDelta actual={stats.today_actual_hours} estimated={stats.today_estimated_hours} />
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="pb-1">
            <div className="flex items-center justify-between text-[11px] text-muted-foreground">
              <span>{t('dashboard.activeProjects')}</span>
              <FolderKanban className="size-4 text-primary" />
            </div>
          </CardHeader>
          <CardContent>
            <div className="tabular text-2xl font-semibold">
              {activeProjects.length}
              <span className="ml-1 text-xs font-normal text-muted-foreground">{t('dashboard.projectUnit')}</span>
            </div>
            <div className="mt-1.5 text-[11px] text-muted-foreground">
              {t('dashboard.totalInvested', { hours: formatHours(totalHours) || '0h' })}
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="pb-1">
            <div className="flex items-center justify-between text-[11px] text-muted-foreground">
              <span>{t('dashboard.monthBalance')}</span>
              <Wallet className="size-4 text-emerald-500" />
            </div>
          </CardHeader>
          <CardContent>
            <div
              className={cn(
                'tabular flex items-baseline gap-1 text-2xl font-semibold',
                monthlyBalance >= 0 ? 'text-emerald-600' : 'text-red-600'
              )}
            >
              ¥{monthlyBalance.toFixed(2)}
              {monthlyBalance >= 0 ? (
                <TrendingUp className="size-4 self-center" />
              ) : (
                <TrendingDown className="size-4 self-center" />
              )}
            </div>
            <div className="mt-1.5 text-[11px] text-muted-foreground">{t('dashboard.pendingTasks', { n: pending })}</div>
          </CardContent>
        </Card>

        <Card className={cn(receivableSum.overdue_count > 0 && 'border-red-200 bg-red-50/30')}>
          <CardHeader className="pb-1">
            <div className="flex items-center justify-between text-[11px] text-muted-foreground">
              <span>{t('dashboard.unreceived')}</span>
              <Wallet className={cn('size-4', receivableSum.overdue_count > 0 ? 'text-red-500' : 'text-amber-500')} />
            </div>
          </CardHeader>
          <CardContent>
            <div className="tabular text-2xl font-semibold text-amber-700">
              ¥{receivableSum.pending_amount.toFixed(2)}
            </div>
            <div className="mt-1.5 flex flex-wrap items-center gap-1.5 text-[11px]">
              {receivableSum.overdue_count > 0 ? (
                <Link to="/receivables" className="font-medium text-red-600 hover:underline">
                  {t('dashboard.overdueLine', {
                    n: receivableSum.overdue_count,
                    amount: `¥${receivableSum.overdue_amount.toFixed(2)}`
                  })}
                </Link>
              ) : (
                <span className="text-muted-foreground">{t('dashboard.noOverdue')}</span>
              )}
              {receivableSum.due_soon_count > 0 && (
                <span className="text-muted-foreground">{t('dashboard.dueSoonLine', { n: receivableSum.due_soon_count })}</span>
              )}
            </div>
          </CardContent>
        </Card>
      </div>

      <div className="mt-4 grid gap-4 xl:grid-cols-[1.5fr_1fr]">
        <Card className="overflow-hidden">
          <CardHeader className="flex-row items-center justify-between border-b border-border py-3">
            <CardTitle>{t('dashboard.todayTasks')}</CardTitle>
            <Button asChild variant="ghost" size="sm">
              <Link to="/today">
                {t('dashboard.all')}
                <ArrowRight />
              </Link>
            </Button>
          </CardHeader>
          {todayTasks.length === 0 ? (
            <div className="px-4 py-10 text-center text-xs text-muted-foreground">
              {t('dashboard.noTodayTasks')}
            </div>
          ) : (
            <ul className="divide-y divide-border">
              {todayTasks.slice(0, 8).map(task => (
                <TaskRow
                  key={task.id}
                  task={task}
                  showTodayAction={false}
                  onOpen={() => setDetail(task)}
                  onToggle={async () => {
                    await taskApi.update(task.id!, { ...task, status: task.status === 'DONE' ? 'TODO' : 'DONE' })
                    loadStats()
                  }}
                  onComplete={async () => {
                    await taskApi.update(task.id!, { ...task, status: 'DONE' })
                    loadStats()
                  }}
                />
              ))}
            </ul>
          )}
        </Card>

        <div className="space-y-4">
          <Card>
            <CardHeader className="flex-row items-center justify-between py-3">
              <CardTitle>{t('dashboard.urgentMoney')}</CardTitle>
              <Button asChild variant="ghost" size="sm">
                <Link to="/receivables">
                  {t('dashboard.receivables')}
                  <ArrowRight />
                </Link>
              </Button>
            </CardHeader>
            {urgentReceivables.length === 0 ? (
              <div className="px-4 pb-4 text-xs text-muted-foreground">{t('dashboard.noUrgentMoney')}</div>
            ) : (
              <ul className="divide-y divide-border border-t border-border">
                {urgentReceivables.map(item => (
                  <li key={item.id} className="flex items-center gap-2 px-4 py-2">
                    {item.overdue ? (
                      <AlertTriangle className="size-3.5 shrink-0 text-red-500" />
                    ) : (
                      <CalendarClock className="size-3.5 shrink-0 text-amber-500" />
                    )}
                    <div className="min-w-0 flex-1">
                      <div className="truncate text-[12.5px] font-medium">{item.title}</div>
                      <div className="truncate text-[10.5px] text-muted-foreground">
                        {item.project_name || t('dashboard.unlinkedProject')} · {item.due_date}
                        {item.overdue && (
                          <span className="ml-1 text-red-600">
                            {t('dashboard.overdueDays', { n: -item.days_left! })}
                          </span>
                        )}
                      </div>
                    </div>
                    <span className="tabular shrink-0 text-[12px] font-semibold text-amber-700">
                      ¥{item.amount.toFixed(2)}
                    </span>
                  </li>
                ))}
              </ul>
            )}
          </Card>

          <Card>
            <CardHeader className="pb-2">
              <CardTitle>{t('dashboard.byCategory')}</CardTitle>
            </CardHeader>
            <CardContent className="space-y-2">
              {CATEGORY_ORDER.map(key => {
                const count = stats.by_category[key] || 0
                const total = Object.values(stats.by_category).reduce((a, b) => a + b, 0) || 1
                return (
                  <div key={key} className="flex items-center gap-2">
                    <span className={cn('size-2 rounded-sm', CATEGORY_STYLE[key].dot)} />
                    <span className="w-14 truncate text-[12px] text-muted-foreground">{t(CATEGORY_KEYS[key])}</span>
                    <div className="h-1.5 flex-1 overflow-hidden rounded-full bg-secondary">
                      <div
                        className={cn('h-full rounded-full', CATEGORY_STYLE[key].bar)}
                        style={{ width: `${Math.round((count / total) * 100)}%` }}
                      />
                    </div>
                    <span className="tabular w-6 text-right text-[12px] text-muted-foreground">{count}</span>
                  </div>
                )
              })}
            </CardContent>
          </Card>

          <Card>
            <CardHeader className="flex-row items-center justify-between py-3">
              <CardTitle>{t('dashboard.activeProjectList')}</CardTitle>
              <Button asChild variant="ghost" size="sm">
                <Link to="/projects">
                  {t('dashboard.all')}
                  <ArrowRight />
                </Link>
              </Button>
            </CardHeader>
            <CardContent className="space-y-2 pt-0">
              {activeProjects.length === 0 && (
                <p className="text-xs text-muted-foreground">{t('dashboard.noActiveProjects')}</p>
              )}
              {activeProjects.slice(0, 4).map(project => (
                <div key={project.id} className="flex items-center gap-2 rounded-md px-1.5 py-1.5 hover:bg-secondary/50">
                  <Briefcase className="size-3.5 shrink-0 text-muted-foreground" />
                  <span className="min-w-0 flex-1 truncate text-[12.5px] font-medium">{project.name}</span>
                  <Badge variant="muted">{formatHours(project.actual_hours) || '0h'}</Badge>
                </div>
              ))}
            </CardContent>
          </Card>
        </div>
      </div>

      {/* 仪表盘里点任务直接看详情，不用先跳到今日 */}
      <TaskDetail
        task={detail}
        onComplete={async task => {
          await taskApi.update(task.id!, { ...task, status: 'DONE' })
          setDetail(null)
          loadStats()
        }}
        onClose={() => {
          setDetail(null)
          loadStats()
        }}
      />
    </div>
  )
}
