import { useEffect, useMemo, useState } from 'react'
import { useSearchParams } from 'react-router-dom'
import { ListFilter, Plus, Search, Timer, X } from 'lucide-react'
import { labelApi, taskApi, errorCodeOf } from '../services/api'
import api from '../services/api'
import type { Label, Project, Task, TaskCategory, TaskPriority, TaskStatus } from '../types'
import { useI18n } from '@/i18n'
import TaskDetail from '../components/TaskDetail'
import TaskRow from '../components/TaskRow'
import PageHeader from '../components/PageHeader'
import HoursDelta from '../components/HoursDelta'
import RichTextEditor from '../components/RichTextEditor'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { Dialog } from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label as FieldLabel } from '@/components/ui/label'
import { Select } from '@/components/ui/select'
import { cn } from '@/lib/utils'
import {
  CATEGORY_KEYS,
  CATEGORY_ORDER,
  DEFAULT_LABEL_COLORS,
  PRIORITY_KEYS,
  PRIORITY_ORDER,
  STATUS_KEYS,
  STATUS_ORDER,
  STATUS_STYLE,
  formatHours,
  todayStr
} from '../components/taskMeta'

const emptyForm = (): Task => ({
  title: '',
  content: '',
  category: 'todo',
  status: 'TODO',
  priority: 'MEDIUM',
  labels: [],
  estimated_hours: 0,
  actual_hours: 0,
  is_today: false
})

// 工时输入：允许留空，空即 0
const parseHours = (raw: string) => {
  const n = parseFloat(raw)
  return Number.isFinite(n) && n > 0 ? n : 0
}

export default function Tasks({ todayMode = false }: { todayMode?: boolean }) {
  const { t, te } = useI18n()
  const [searchParams, setSearchParams] = useSearchParams()
  const [tasks, setTasks] = useState<Task[]>([])
  const [projects, setProjects] = useState<Project[]>([])
  const [labels, setLabels] = useState<Label[]>([])
  const [form, setForm] = useState<Task>(emptyForm())
  const [showForm, setShowForm] = useState(false)
  const [labelInput, setLabelInput] = useState('')
  const [formError, setFormError] = useState('')
  const [filters, setFilters] = useState({
    category: '',
    status: '',
    priority: '',
    project_id: '',
    label: '',
    keyword: searchParams.get('keyword') || ''
  })
  const [loading, setLoading] = useState(false)
  const [detail, setDetail] = useState<Task | null>(null)

  // 顶栏「新建任务」带 ?new=1 跳进来，直接开表单
  useEffect(() => {
    if (searchParams.get('new') === '1') {
      setForm({ ...emptyForm(), is_today: todayMode })
      setLabelInput('')
      setFormError('')
      setShowForm(true)
      const next = new URLSearchParams(searchParams)
      next.delete('new')
      setSearchParams(next, { replace: true })
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [searchParams])

  useEffect(() => {
    loadAll()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [todayMode])

  const loadAll = async () => {
    setLoading(true)
    try {
      const [projectList, labelList] = await Promise.all([
        api.get<any, Project[]>('/projects'),
        labelApi.list()
      ])
      setProjects(projectList || [])
      setLabels(labelList || [])
      await loadTasks()
    } catch (error) {
      console.error('Failed to load tasks:', error)
    } finally {
      setLoading(false)
    }
  }

  const loadTasks = async () => {
    const data = await taskApi.list({
      today: todayMode || undefined,
      category: filters.category || undefined,
      status: filters.status || undefined,
      priority: filters.priority || undefined,
      project_id: filters.project_id ? Number(filters.project_id) : undefined,
      label: filters.label || undefined,
      keyword: filters.keyword || undefined
    })
    setTasks(data || [])
  }

  useEffect(() => {
    loadTasks().catch(err => console.error(err))
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [filters])

  // 顶栏搜索跳过来时只带 query，组件没重挂载，得手动同步关键词
  const urlKeyword = searchParams.get('keyword') || ''
  useEffect(() => {
    setFilters(prev => (prev.keyword === urlKeyword ? prev : { ...prev, keyword: urlKeyword }))
  }, [urlKeyword])

  const grouped = useMemo(() => {
    const map: Record<string, Task[]> = {}
    STATUS_ORDER.forEach(status => (map[status] = []))
    tasks.forEach(task => {
      map[task.status] = map[task.status] || []
      map[task.status].push(task)
    })
    return map
  }, [tasks])

  const submit = async (e: React.FormEvent) => {
    e.preventDefault()
    setFormError('')
    try {
      const payload: Task = {
        ...form,
        labels: labelInput
          .split(/[,，\s]+/)
          .map(name => name.trim())
          .filter(Boolean)
          .map(name => ({ name, color: form.labels?.[0]?.color || DEFAULT_LABEL_COLORS[0] }))
      }
      if (form.id) {
        await taskApi.update(form.id, payload)
      } else {
        await taskApi.create(payload)
      }
      setShowForm(false)
      setForm(emptyForm())
      setLabelInput('')
      await loadAll()
    } catch (error) {
      console.error('Failed to save task:', error)
      const { code, message } = errorCodeOf(error)
      setFormError(te(code, message))
    }
  }

  const edit = (task: Task) => {
    setForm({ ...task })
    setLabelInput((task.labels || []).map(l => l.name).join(', '))
    setFormError('')
    setShowForm(true)
  }

  const remove = async (id: number) => {
    if (!confirm(t('task.confirmDelete'))) return
    await taskApi.remove(id)
    await loadAll()
  }

  const changeStatus = async (task: Task, status: TaskStatus) => {
    await taskApi.update(task.id!, { ...task, status })
    await loadAll()
  }

  const toggleToday = async (task: Task) => {
    await taskApi.setToday(task.id!, !task.is_today)
    await loadAll()
  }

  const openDetail = (task: Task) => setDetail(task)

  // 列表里点「完成」直接收工，弹窗里的同款按钮走这里再关窗，两边行为一致
  const completeTask = async (task: Task) => {
    await changeStatus(task, 'DONE')
    setDetail(prev => (prev?.id === task.id ? null : prev))
  }

  const doneCount = grouped.DONE?.length || 0
  const progress = tasks.length ? Math.round((doneCount / tasks.length) * 100) : 0
  const todayActual = tasks.reduce((sum, t) => sum + (t.actual_hours || 0), 0)
  const todayEstimated = tasks.reduce((sum, t) => sum + (t.estimated_hours || 0), 0)

  const activeFilterCount = [filters.category, filters.status, filters.priority, filters.project_id, filters.label].filter(
    Boolean
  ).length
  const resetFilters = () =>
    setFilters({ category: '', status: '', priority: '', project_id: '', label: '', keyword: '' })

  return (
    <div>
      <PageHeader
        eyebrow={todayMode ? 'TODAY' : 'TASKS'}
        title={todayMode ? t('task.titleToday', { date: todayStr() }) : t('task.title')}
        description={todayMode ? t('task.descriptionToday') : t('task.description')}
        actions={
          <Button
            onClick={() => {
              setForm({ ...emptyForm(), is_today: todayMode })
              setLabelInput('')
              setFormError('')
              setShowForm(true)
            }}
          >
            <Plus />
            {t('task.new')}
          </Button>
        }
      />

      {todayMode && (
        <Card className="mb-4 overflow-hidden">
          <div className="grid gap-4 px-4 py-4 sm:grid-cols-[1fr_1fr_1fr_1.4fr]">
            <div>
              <div className="text-[11px] text-muted-foreground">{t('task.stats.scheduled')}</div>
              <div className="tabular mt-1 text-2xl font-semibold">{tasks.length}</div>
            </div>
            <div>
              <div className="text-[11px] text-muted-foreground">{t('task.stats.done')}</div>
              <div className="tabular mt-1 text-2xl font-semibold text-emerald-600">
                {doneCount}
                <span className="ml-1 text-xs font-normal text-muted-foreground">/ {tasks.length}</span>
              </div>
            </div>
            <div>
              <div className="text-[11px] text-muted-foreground">{t('task.stats.rate')}</div>
              <div className="tabular mt-1 text-2xl font-semibold">{progress}%</div>
            </div>
            <div>
              <div className="flex items-center gap-1 text-[11px] text-muted-foreground">
                <Timer className="size-3" />
                {t('task.stats.hours')}
              </div>
              <div className="mt-1 flex flex-wrap items-center gap-2">
                <span className="tabular text-2xl font-semibold">
                  {formatHours(todayActual) || '0h'}
                  <span className="ml-1 text-xs font-normal text-muted-foreground">
                    / {formatHours(todayEstimated) || t('task.stats.notEstimated')}
                  </span>
                </span>
                <HoursDelta actual={todayActual} estimated={todayEstimated} />
              </div>
            </div>
          </div>
          <div className="h-1 w-full bg-secondary">
            <div className="h-full bg-emerald-500 transition-all" style={{ width: `${progress}%` }} />
          </div>
        </Card>
      )}

      {!todayMode && (
        <Card className="mb-4">
          <div className="flex flex-wrap items-center gap-2 px-3 py-2.5">
            <div className="relative min-w-[180px] flex-1">
              <Search className="pointer-events-none absolute left-2.5 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
              <Input
                placeholder={t('task.filter.keyword')}
                value={filters.keyword}
                onChange={e => setFilters({ ...filters, keyword: e.target.value })}
                className="pl-8"
              />
            </div>
            <Select
              className="w-auto min-w-[104px]"
              value={filters.category}
              onChange={e => setFilters({ ...filters, category: e.target.value })}
            >
              <option value="">{t('task.filter.allCategories')}</option>
              {CATEGORY_ORDER.map(value => (
                <option key={value} value={value}>{t(CATEGORY_KEYS[value])}</option>
              ))}
            </Select>
            <Select
              className="w-auto min-w-[104px]"
              value={filters.status}
              onChange={e => setFilters({ ...filters, status: e.target.value })}
            >
              <option value="">{t('task.filter.allStatuses')}</option>
              {STATUS_ORDER.map(value => (
                <option key={value} value={value}>{t(STATUS_KEYS[value])}</option>
              ))}
            </Select>
            <Select
              className="w-auto min-w-[110px]"
              value={filters.priority}
              onChange={e => setFilters({ ...filters, priority: e.target.value })}
            >
              <option value="">{t('task.filter.allPriorities')}</option>
              {PRIORITY_ORDER.map(value => (
                <option key={value} value={value}>{t(PRIORITY_KEYS[value])}</option>
              ))}
            </Select>
            <Select
              className="w-auto min-w-[110px]"
              value={filters.project_id}
              onChange={e => setFilters({ ...filters, project_id: e.target.value })}
            >
              <option value="">{t('task.filter.allProjects')}</option>
              {projects.map(p => (
                <option key={p.id} value={p.id}>{p.name}</option>
              ))}
            </Select>
            <Select
              className="w-auto min-w-[104px]"
              value={filters.label}
              onChange={e => setFilters({ ...filters, label: e.target.value })}
            >
              <option value="">{t('task.filter.allLabels')}</option>
              {labels.map(l => (
                <option key={l.id} value={l.name}>{l.name}</option>
              ))}
            </Select>
            {activeFilterCount > 0 && (
              <Button variant="ghost" size="sm" onClick={resetFilters}>
                <X />
                {t('task.filter.clear', { n: activeFilterCount })}
              </Button>
            )}
          </div>
        </Card>
      )}

      {loading && (
        <div className="flex items-center gap-2 px-1 py-6 text-xs text-muted-foreground">
          <span className="size-3 animate-spin rounded-full border-2 border-border border-t-primary" />
          {t('common.loading')}
        </div>
      )}

      {!loading && tasks.length === 0 && (
        <Card className="flex flex-col items-center justify-center gap-2 px-6 py-14 text-center">
          <ListFilter className="size-6 text-muted-foreground/60" />
          <div className="text-sm font-medium">{t('task.empty')}</div>
          <div className="text-xs text-muted-foreground">
            {todayMode ? t('task.emptyToday') : t('task.emptyList')}
          </div>
        </Card>
      )}

      {!loading &&
        STATUS_ORDER.map(status => {
          const items = grouped[status] || []
          if (items.length === 0) return null
          const hours = items.reduce((s, t2) => s + (t2.actual_hours || 0), 0)
          const estimates = items.reduce((s, t2) => s + (t2.estimated_hours || 0), 0)
          return (
            <Card key={status} className="mb-3 overflow-hidden">
              <div className="flex items-center gap-2 border-b border-border bg-secondary/30 px-4 py-2">
                <span className={cn('rounded-md px-1.5 py-0.5 text-[11px] font-medium', STATUS_STYLE[status])}>
                  {t(STATUS_KEYS[status])}
                </span>
                <span className="tabular text-[11px] text-muted-foreground">{items.length}</span>
                {(hours > 0 || estimates > 0) && (
                  <span className="tabular ml-auto text-[11px] text-muted-foreground">
                    {t('task.stats.groupHours', {
                      actual: formatHours(hours) || '0h',
                      estimate: formatHours(estimates) || '—'
                    })}
                  </span>
                )}
              </div>
              <ul className="divide-y divide-border">
                {items.map(task => (
                  <TaskRow
                    key={task.id}
                    task={task}
                    onOpen={() => openDetail(task)}
                    onToggle={() => changeStatus(task, task.status === 'DONE' ? 'TODO' : 'DONE')}
                    onComplete={() => completeTask(task)}
                    onToday={() => toggleToday(task)}
                    onEdit={() => edit(task)}
                    onRemove={() => remove(task.id!)}
                  />
                ))}
              </ul>
            </Card>
          )
        })}

      {!todayMode && !loading && labels.length > 0 && (
        <Card className="mt-4 px-4 py-3">
          <div className="mb-2 text-xs font-medium text-muted-foreground">{t('task.labelDictionary')}</div>
          <div className="flex flex-wrap gap-1.5">
            {labels.map(label => (
              <button
                key={label.id}
                type="button"
                onClick={() => setFilters({ ...filters, label: label.name })}
                className="rounded-md px-2 py-1 text-[11px] font-medium transition-opacity hover:opacity-80"
                style={{ background: `${label.color}14`, color: label.color }}
              >
                {label.name} · {label.task_count ?? 0}
              </button>
            ))}
          </div>
        </Card>
      )}

      {/* 新建 / 编辑 */}
      <Dialog
        open={showForm}
        onClose={() => { setShowForm(false); setForm(emptyForm()); setFormError('') }}
        title={form.id ? t('task.editTitle') : t('task.new')}
        description={t('task.formDescription')}
        footer={
          <>
            <Button variant="outline" onClick={() => { setShowForm(false); setForm(emptyForm()); setFormError('') }}>
              {t('common.cancel')}
            </Button>
            <Button type="submit" form="task-form">{t('common.save')}</Button>
          </>
        }
      >
        <form id="task-form" onSubmit={submit} className="space-y-3.5">
          <div>
            <FieldLabel>{t('task.field.title')}</FieldLabel>
            <Input
              value={form.title}
              onChange={e => setForm({ ...form, title: e.target.value })}
              placeholder={t('task.field.titlePlaceholder')}
              required
            />
          </div>

          <div className="grid gap-3 sm:grid-cols-3">
            <div>
              <FieldLabel>{t('task.field.category')}</FieldLabel>
              <Select
                value={form.category}
                onChange={e => setForm({ ...form, category: e.target.value as TaskCategory })}
              >
                {CATEGORY_ORDER.map(value => (
                  <option key={value} value={value}>{t(CATEGORY_KEYS[value])}</option>
                ))}
              </Select>
            </div>
            <div>
              <FieldLabel>{t('task.field.status')}</FieldLabel>
              <Select value={form.status} onChange={e => setForm({ ...form, status: e.target.value as TaskStatus })}>
                {STATUS_ORDER.map(value => (
                  <option key={value} value={value}>{t(STATUS_KEYS[value])}</option>
                ))}
              </Select>
            </div>
            <div>
              <FieldLabel>{t('task.field.priority')}</FieldLabel>
              <Select
                value={form.priority}
                onChange={e => setForm({ ...form, priority: e.target.value as TaskPriority })}
              >
                {PRIORITY_ORDER.map(value => (
                  <option key={value} value={value}>{t(PRIORITY_KEYS[value])}</option>
                ))}
              </Select>
            </div>
          </div>

          <div className="grid gap-3 sm:grid-cols-3">
            <div>
              <FieldLabel>{t('task.field.project')}</FieldLabel>
              <Select
                value={form.project_id ?? ''}
                onChange={e => setForm({ ...form, project_id: e.target.value ? Number(e.target.value) : undefined })}
              >
                <option value="">{t('task.field.projectNone')}</option>
                {projects.map(p => (
                  <option key={p.id} value={p.id}>{p.name}</option>
                ))}
              </Select>
            </div>
            <div>
              <FieldLabel>{t('task.field.scheduled')}</FieldLabel>
              <Input
                type="date"
                value={form.scheduled_date || ''}
                onChange={e => setForm({ ...form, scheduled_date: e.target.value })}
              />
            </div>
            <div>
              <FieldLabel>{t('task.field.due')}</FieldLabel>
              <Input
                type="date"
                value={form.due_date || ''}
                onChange={e => setForm({ ...form, due_date: e.target.value })}
              />
            </div>
          </div>

          <div className="grid gap-3 sm:grid-cols-2">
            <div>
              <FieldLabel>{t('task.field.estimate')}</FieldLabel>
              <Input
                type="number"
                min="0"
                step="0.5"
                placeholder={t('task.field.estimatePlaceholder')}
                value={form.estimated_hours || ''}
                onChange={e => setForm({ ...form, estimated_hours: parseHours(e.target.value) })}
              />
            </div>
            <div>
              <FieldLabel>{t('task.field.actual')}</FieldLabel>
              <Input
                type="number"
                min="0"
                step="0.5"
                placeholder={t('task.field.actualPlaceholder')}
                value={form.actual_hours || ''}
                onChange={e => setForm({ ...form, actual_hours: parseHours(e.target.value) })}
              />
            </div>
          </div>

          <div>
            <FieldLabel>{t('task.field.labels')}</FieldLabel>
            <Input
              value={labelInput}
              onChange={e => setLabelInput(e.target.value)}
              placeholder={t('task.field.labelsPlaceholder')}
            />
          </div>

          {/* 内容用富文本：能贴图、能挂附件，写长一点的背景说明才不至于是一坨纯文本 */}
          <div>
            <FieldLabel>{t('task.field.content')}</FieldLabel>
            <RichTextEditor
              value={form.content || ''}
              onChange={next => setForm({ ...form, content: next })}
              placeholder={t('task.field.contentPlaceholder')}
              minRows={5}
            />
          </div>

          <label className="flex items-center gap-2 text-[13px]">
            <input
              type="checkbox"
              className="size-3.5 accent-[hsl(var(--primary))]"
              checked={!!form.is_today}
              onChange={e => setForm({ ...form, is_today: e.target.checked })}
            />
            {t('task.field.addToday')}
          </label>
          {formError && <div className="text-[11.5px] text-red-600">{formError}</div>}
        </form>
      </Dialog>

      {/* 详情（仪表盘共用同一份组件） */}
      <TaskDetail task={detail} onClose={() => setDetail(null)} onComplete={completeTask} />
    </div>
  )
}
