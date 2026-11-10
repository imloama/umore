import { useEffect, useMemo, useState } from 'react'
import { useSearchParams } from 'react-router-dom'
import { ListFilter, Plus, Search, Timer, X } from 'lucide-react'
import { labelApi, taskApi } from '../services/api'
import api from '../services/api'
import type { Label, Project, Task, TaskCategory, TaskPriority, TaskStatus } from '../types'
import TaskDetail from '../components/TaskDetail'
import TaskRow from '../components/TaskRow'
import PageHeader from '../components/PageHeader'
import HoursDelta from '../components/HoursDelta'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { Dialog } from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label as FieldLabel } from '@/components/ui/label'
import { Select } from '@/components/ui/select'
import { Textarea } from '@/components/ui/textarea'
import { cn } from '@/lib/utils'
import {
  CATEGORY_LABEL,
  DEFAULT_LABEL_COLORS,
  PRIORITY_LABEL,
  STATUS_LABEL,
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
  const [searchParams, setSearchParams] = useSearchParams()
  const [tasks, setTasks] = useState<Task[]>([])
  const [projects, setProjects] = useState<Project[]>([])
  const [labels, setLabels] = useState<Label[]>([])
  const [form, setForm] = useState<Task>(emptyForm())
  const [showForm, setShowForm] = useState(false)
  const [labelInput, setLabelInput] = useState('')
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
    }
  }

  const edit = (task: Task) => {
    setForm({ ...task })
    setLabelInput((task.labels || []).map(l => l.name).join(', '))
    setShowForm(true)
  }

  const remove = async (id: number) => {
    if (!confirm('确定删除该任务?')) return
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
        title={todayMode ? `今日 · ${todayStr()}` : '任务'}
        description={
          todayMode
            ? '今天要推进的事都在这里，勾掉一件算一件。'
            : '待办、问题、需求、商机都在一张表里，靠类型和标签区分。'
        }
        actions={
          <Button
            onClick={() => {
              setForm({ ...emptyForm(), is_today: todayMode })
              setLabelInput('')
              setShowForm(true)
            }}
          >
            <Plus />
            新建任务
          </Button>
        }
      />

      {todayMode && (
        <Card className="mb-4 overflow-hidden">
          <div className="grid gap-4 px-4 py-4 sm:grid-cols-[1fr_1fr_1fr_1.4fr]">
            <div>
              <div className="text-[11px] text-muted-foreground">今日排期</div>
              <div className="tabular mt-1 text-2xl font-semibold">{tasks.length}</div>
            </div>
            <div>
              <div className="text-[11px] text-muted-foreground">已完成</div>
              <div className="tabular mt-1 text-2xl font-semibold text-emerald-600">
                {doneCount}
                <span className="ml-1 text-xs font-normal text-muted-foreground">/ {tasks.length}</span>
              </div>
            </div>
            <div>
              <div className="text-[11px] text-muted-foreground">完成率</div>
              <div className="tabular mt-1 text-2xl font-semibold">{progress}%</div>
            </div>
            <div>
              <div className="flex items-center gap-1 text-[11px] text-muted-foreground">
                <Timer className="size-3" />
                工时 实际 / 评估
              </div>
              <div className="mt-1 flex flex-wrap items-center gap-2">
                <span className="tabular text-2xl font-semibold">
                  {formatHours(todayActual) || '0h'}
                  <span className="ml-1 text-xs font-normal text-muted-foreground">
                    / {formatHours(todayEstimated) || '未评估'}
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
                placeholder="搜索标题或内容"
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
              <option value="">全部类型</option>
              {Object.entries(CATEGORY_LABEL).map(([value, label]) => (
                <option key={value} value={value}>{label}</option>
              ))}
            </Select>
            <Select
              className="w-auto min-w-[104px]"
              value={filters.status}
              onChange={e => setFilters({ ...filters, status: e.target.value })}
            >
              <option value="">全部状态</option>
              {STATUS_ORDER.map(value => (
                <option key={value} value={value}>{STATUS_LABEL[value]}</option>
              ))}
            </Select>
            <Select
              className="w-auto min-w-[110px]"
              value={filters.priority}
              onChange={e => setFilters({ ...filters, priority: e.target.value })}
            >
              <option value="">全部优先级</option>
              {Object.entries(PRIORITY_LABEL).map(([value, label]) => (
                <option key={value} value={value}>{label}</option>
              ))}
            </Select>
            <Select
              className="w-auto min-w-[110px]"
              value={filters.project_id}
              onChange={e => setFilters({ ...filters, project_id: e.target.value })}
            >
              <option value="">全部项目</option>
              {projects.map(p => (
                <option key={p.id} value={p.id}>{p.name}</option>
              ))}
            </Select>
            <Select
              className="w-auto min-w-[104px]"
              value={filters.label}
              onChange={e => setFilters({ ...filters, label: e.target.value })}
            >
              <option value="">全部标签</option>
              {labels.map(l => (
                <option key={l.id} value={l.name}>{l.name}</option>
              ))}
            </Select>
            {activeFilterCount > 0 && (
              <Button variant="ghost" size="sm" onClick={resetFilters}>
                <X />
                清空 {activeFilterCount}
              </Button>
            )}
          </div>
        </Card>
      )}

      {loading && (
        <div className="flex items-center gap-2 px-1 py-6 text-xs text-muted-foreground">
          <span className="size-3 animate-spin rounded-full border-2 border-border border-t-primary" />
          加载中…
        </div>
      )}

      {!loading && tasks.length === 0 && (
        <Card className="flex flex-col items-center justify-center gap-2 px-6 py-14 text-center">
          <ListFilter className="size-6 text-muted-foreground/60" />
          <div className="text-sm font-medium">这里还没有任务</div>
          <div className="text-xs text-muted-foreground">
            {todayMode ? '去任务列表挑几件加进今日，或者直接新建。' : '点右上角「新建任务」开始记录。'}
          </div>
        </Card>
      )}

      {!loading &&
        STATUS_ORDER.map(status => {
          const items = grouped[status] || []
          if (items.length === 0) return null
          const hours = items.reduce((s, t) => s + (t.actual_hours || 0), 0)
          const estimates = items.reduce((s, t) => s + (t.estimated_hours || 0), 0)
          return (
            <Card key={status} className="mb-3 overflow-hidden">
              <div className="flex items-center gap-2 border-b border-border bg-secondary/30 px-4 py-2">
                <span className={cn('rounded-md px-1.5 py-0.5 text-[11px] font-medium', STATUS_STYLE[status])}>
                  {STATUS_LABEL[status]}
                </span>
                <span className="tabular text-[11px] text-muted-foreground">{items.length}</span>
                {(hours > 0 || estimates > 0) && (
                  <span className="tabular ml-auto text-[11px] text-muted-foreground">
                    工时 {formatHours(hours) || '0h'} / 评估 {formatHours(estimates) || '—'}
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
          <div className="mb-2 text-xs font-medium text-muted-foreground">标签字典</div>
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
        onClose={() => { setShowForm(false); setForm(emptyForm()) }}
        title={form.id ? '编辑任务' : '新建任务'}
        description="工时是两个手填的数字，收工后回来补实际用时就行。"
        footer={
          <>
            <Button variant="outline" onClick={() => { setShowForm(false); setForm(emptyForm()) }}>
              取消
            </Button>
            <Button type="submit" form="task-form">保存</Button>
          </>
        }
      >
        <form id="task-form" onSubmit={submit} className="space-y-3.5">
          <div>
            <FieldLabel>标题 *</FieldLabel>
            <Input
              value={form.title}
              onChange={e => setForm({ ...form, title: e.target.value })}
              placeholder="一句话说清要做什么"
              required
            />
          </div>

          <div className="grid gap-3 sm:grid-cols-3">
            <div>
              <FieldLabel>类型</FieldLabel>
              <Select
                value={form.category}
                onChange={e => setForm({ ...form, category: e.target.value as TaskCategory })}
              >
                {Object.entries(CATEGORY_LABEL).map(([value, label]) => (
                  <option key={value} value={value}>{label}</option>
                ))}
              </Select>
            </div>
            <div>
              <FieldLabel>状态</FieldLabel>
              <Select value={form.status} onChange={e => setForm({ ...form, status: e.target.value as TaskStatus })}>
                {STATUS_ORDER.map(value => (
                  <option key={value} value={value}>{STATUS_LABEL[value]}</option>
                ))}
              </Select>
            </div>
            <div>
              <FieldLabel>优先级</FieldLabel>
              <Select
                value={form.priority}
                onChange={e => setForm({ ...form, priority: e.target.value as TaskPriority })}
              >
                {Object.entries(PRIORITY_LABEL).map(([value, label]) => (
                  <option key={value} value={value}>{label}</option>
                ))}
              </Select>
            </div>
          </div>

          <div className="grid gap-3 sm:grid-cols-3">
            <div>
              <FieldLabel>项目</FieldLabel>
              <Select
                value={form.project_id ?? ''}
                onChange={e => setForm({ ...form, project_id: e.target.value ? Number(e.target.value) : undefined })}
              >
                <option value="">不关联</option>
                {projects.map(p => (
                  <option key={p.id} value={p.id}>{p.name}</option>
                ))}
              </Select>
            </div>
            <div>
              <FieldLabel>计划日期</FieldLabel>
              <Input
                type="date"
                value={form.scheduled_date || ''}
                onChange={e => setForm({ ...form, scheduled_date: e.target.value })}
              />
            </div>
            <div>
              <FieldLabel>截止日期</FieldLabel>
              <Input
                type="date"
                value={form.due_date || ''}
                onChange={e => setForm({ ...form, due_date: e.target.value })}
              />
            </div>
          </div>

          <div className="grid gap-3 sm:grid-cols-2">
            <div>
              <FieldLabel>工作量评估（小时）</FieldLabel>
              <Input
                type="number"
                min="0"
                step="0.5"
                placeholder="例如 4"
                value={form.estimated_hours || ''}
                onChange={e => setForm({ ...form, estimated_hours: parseHours(e.target.value) })}
              />
            </div>
            <div>
              <FieldLabel>实际用时（小时）</FieldLabel>
              <Input
                type="number"
                min="0"
                step="0.5"
                placeholder="收工后填"
                value={form.actual_hours || ''}
                onChange={e => setForm({ ...form, actual_hours: parseHours(e.target.value) })}
              />
            </div>
          </div>

          <div>
            <FieldLabel>标签（逗号分隔）</FieldLabel>
            <Input value={labelInput} onChange={e => setLabelInput(e.target.value)} placeholder="客户A, 紧急" />
          </div>

          <div>
            <FieldLabel>内容</FieldLabel>
            <Textarea
              rows={5}
              value={form.content || ''}
              onChange={e => setForm({ ...form, content: e.target.value })}
              placeholder="背景、验收标准、相关链接…"
            />
          </div>

          <label className="flex items-center gap-2 text-[13px]">
            <input
              type="checkbox"
              className="size-3.5 accent-[hsl(var(--primary))]"
              checked={!!form.is_today}
              onChange={e => setForm({ ...form, is_today: e.target.checked })}
            />
            加入今日
          </label>
        </form>
      </Dialog>

      {/* 详情（仪表盘共用同一份组件） */}
      <TaskDetail task={detail} onClose={() => setDetail(null)} />
    </div>
  )
}
