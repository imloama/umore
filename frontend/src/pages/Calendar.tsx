import { useEffect, useMemo, useState } from 'react'
import { ChevronLeft, ChevronRight } from 'lucide-react'
import api, { taskApi } from '../services/api'
import type { Project, Task, TaskStatus } from '../types'
import PageHeader from '../components/PageHeader'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { Dialog } from '@/components/ui/dialog'
import { Select } from '@/components/ui/select'
import { cn } from '@/lib/utils'
import {
  CATEGORY_LABEL,
  CATEGORY_STYLE,
  PRIORITY_COLOR,
  PRIORITY_LABEL,
  STATUS_LABEL,
  STATUS_ORDER,
  hoursPair,
  todayStr
} from '../components/taskMeta'

// ---------- 日期工具（不引第三方库，避免只为日历加依赖） ----------
const pad = (n: number) => `${n}`.padStart(2, '0')
const toKey = (d: Date) => `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`
const parseKey = (key: string) => {
  const [y, m, d] = key.split('-').map(Number)
  return new Date(y, m - 1, d)
}
const addDays = (key: string, n: number) => {
  const d = parseKey(key)
  d.setDate(d.getDate() + n)
  return toKey(d)
}
const startOfWeek = (key: string) => {
  const d = parseKey(key)
  const offset = (d.getDay() + 6) % 7 // 周一为一周第一天
  d.setDate(d.getDate() - offset)
  return toKey(d)
}
const WEEKDAY = ['一', '二', '三', '四', '五', '六', '日']

interface DayCell {
  key: string
  day: number
  inMonth: boolean
  isToday: boolean
  weekend: boolean
}

const buildMonth = (cursor: string, view: 'month' | 'week'): DayCell[] => {
  const base = parseKey(cursor)
  const first = view === 'month' ? `${base.getFullYear()}-${pad(base.getMonth() + 1)}-01` : startOfWeek(cursor)
  const start = startOfWeek(first)
  const days = view === 'month' ? 42 : 7
  const today = todayStr()
  return Array.from({ length: days }, (_, i) => {
    const key = addDays(start, i)
    const d = parseKey(key)
    return {
      key,
      day: d.getDate(),
      inMonth: view === 'week' ? true : d.getMonth() === base.getMonth(),
      isToday: key === today,
      weekend: d.getDay() === 0 || d.getDay() === 6
    }
  })
}

const taskDate = (task: Task) => task.scheduled_date || task.due_date || ''

export default function Calendar() {
  const [cursor, setCursor] = useState(todayStr())
  const [view, setView] = useState<'month' | 'week'>('month')
  const [tasks, setTasks] = useState<Task[]>([])
  const [projects, setProjects] = useState<Project[]>([])
  const [filters, setFilters] = useState({ category: '', status: '', priority: '', project_id: '' })
  const [dragging, setDragging] = useState<number | null>(null)
  const [dragOver, setDragOver] = useState<string | null>(null)
  const [selectedDate, setSelectedDate] = useState<string | null>(null)
  const [loading, setLoading] = useState(false)

  const cells = useMemo(() => buildMonth(cursor, view), [cursor, view])
  const rangeStart = cells[0]?.key
  const rangeEnd = cells[cells.length - 1]?.key

  useEffect(() => {
    api.get<any, Project[]>('/projects').then(res => setProjects(res || [])).catch(console.error)
  }, [])

  const loadTasks = async () => {
    if (!rangeStart || !rangeEnd) return
    setLoading(true)
    try {
      const params = new URLSearchParams({ start_date: rangeStart, end_date: rangeEnd })
      if (filters.category) params.set('category', filters.category)
      if (filters.status) params.set('status', filters.status)
      if (filters.priority) params.set('priority', filters.priority)
      if (filters.project_id) params.set('project_id', filters.project_id)
      const data = await api.get<any, Task[]>(`/tasks?${params.toString()}`)
      setTasks(data || [])
    } catch (error) {
      console.error('Failed to load calendar tasks:', error)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    loadTasks()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [rangeStart, rangeEnd, filters])

  const byDate = useMemo(() => {
    const map: Record<string, Task[]> = {}
    tasks.forEach(task => {
      const key = taskDate(task)
      if (!key) return
      map[key] = map[key] || []
      map[key].push(task)
    })
    Object.values(map).forEach(list =>
      list.sort((a, b) => {
        const rank = (t: Task) => (t.status === 'DONE' ? 4 : STATUS_ORDER.indexOf(t.status))
        const diff = rank(a) - rank(b)
        return diff !== 0 ? diff : (a.number || 0) - (b.number || 0)
      })
    )
    return map
  }, [tasks])

  const shift = (step: number) => {
    const d = parseKey(cursor)
    if (view === 'month') {
      d.setDate(1)
      d.setMonth(d.getMonth() + step)
    } else {
      d.setDate(d.getDate() + step * 7)
    }
    setCursor(toKey(d))
  }

  const moveTask = async (taskId: number, date: string) => {
    const task = tasks.find(t => t.id === taskId)
    if (!task || taskDate(task) === date) return
    try {
      await taskApi.update(taskId, { ...task, scheduled_date: date })
      await loadTasks()
    } catch (error) {
      console.error('Failed to reschedule task:', error)
    }
  }

  const toggleDone = async (task: Task) => {
    await taskApi.update(task.id!, { ...task, status: task.status === 'DONE' ? 'TODO' : 'DONE' })
    await loadTasks()
  }

  const monthLabel = (() => {
    const d = parseKey(cursor)
    return view === 'month' ? `${d.getFullYear()} 年 ${d.getMonth() + 1} 月` : `${rangeStart} ~ ${rangeEnd}`
  })()

  const selectedTasks = selectedDate ? byDate[selectedDate] || [] : []

  return (
    <div>
      <PageHeader
        eyebrow="CALENDAR"
        title="日历"
        description="拖卡片到别的日期就改排期，没排期的按截止日期落位。"
        actions={
          <>
            <div className="flex rounded-md border border-input bg-card p-0.5 shadow-sm">
              {(['month', 'week'] as const).map(v => (
                <button
                  key={v}
                  type="button"
                  onClick={() => setView(v)}
                  className={cn(
                    'rounded px-2.5 py-1 text-[12px] transition-colors',
                    view === v ? 'bg-primary text-primary-foreground' : 'text-muted-foreground hover:text-foreground'
                  )}
                >
                  {v === 'month' ? '月' : '周'}
                </button>
              ))}
            </div>
            <div className="flex items-center gap-1">
              <Button variant="outline" size="icon-sm" onClick={() => shift(-1)} title="上一页">
                <ChevronLeft />
              </Button>
              <Button variant="outline" size="sm" onClick={() => setCursor(todayStr())}>今天</Button>
              <Button variant="outline" size="icon-sm" onClick={() => shift(1)} title="下一页">
                <ChevronRight />
              </Button>
            </div>
          </>
        }
      />

      <div className="mb-3 flex flex-wrap items-center gap-2">
        <span className="tabular mr-1 text-[13px] font-semibold">{monthLabel}</span>
        <Select className="w-auto min-w-[100px]" value={filters.category} onChange={e => setFilters({ ...filters, category: e.target.value })}>
          <option value="">全部类型</option>
          {Object.entries(CATEGORY_LABEL).map(([value, label]) => (
            <option key={value} value={value}>{label}</option>
          ))}
        </Select>
        <Select className="w-auto min-w-[100px]" value={filters.status} onChange={e => setFilters({ ...filters, status: e.target.value })}>
          <option value="">全部状态</option>
          {STATUS_ORDER.map(value => (
            <option key={value} value={value}>{STATUS_LABEL[value]}</option>
          ))}
        </Select>
        <Select className="w-auto min-w-[108px]" value={filters.priority} onChange={e => setFilters({ ...filters, priority: e.target.value })}>
          <option value="">全部优先级</option>
          {Object.entries(PRIORITY_LABEL).map(([value, label]) => (
            <option key={value} value={value}>{label}</option>
          ))}
        </Select>
        <Select className="w-auto min-w-[108px]" value={filters.project_id} onChange={e => setFilters({ ...filters, project_id: e.target.value })}>
          <option value="">全部项目</option>
          {projects.map(p => (
            <option key={p.id} value={p.id}>{p.name}</option>
          ))}
        </Select>
        {loading && <span className="text-[11px] text-muted-foreground">加载中…</span>}
      </div>

      <Card className="overflow-hidden">
        <div className="grid grid-cols-7 border-b border-border bg-secondary/40">
          {WEEKDAY.map((w, i) => (
            <div
              key={w}
              className={cn(
                'px-2 py-1.5 text-[11px] font-medium text-muted-foreground',
                i >= 5 && 'text-muted-foreground/60'
              )}
            >
              周{w}
            </div>
          ))}
        </div>
        <div className="grid grid-cols-7">
          {cells.map(cell => {
            const items = byDate[cell.key] || []
            const done = items.filter(t => t.status === 'DONE').length
            const limit = view === 'week' ? 12 : 3
            return (
              <div
                key={cell.key}
                className={cn(
                  'min-h-[100px] cursor-pointer border-b border-r border-border p-1.5 transition-colors last:border-r-0',
                  view === 'week' && 'min-h-[280px]',
                  !cell.inMonth && 'bg-secondary/30',
                  cell.weekend && 'bg-secondary/20',
                  dragOver === cell.key && 'bg-accent ring-2 ring-inset ring-primary/40'
                )}
                onDragOver={e => {
                  e.preventDefault()
                  setDragOver(cell.key)
                }}
                onDragLeave={() => setDragOver(prev => (prev === cell.key ? null : prev))}
                onDrop={() => {
                  setDragOver(null)
                  if (dragging != null) moveTask(dragging, cell.key)
                  setDragging(null)
                }}
                onClick={() => setSelectedDate(cell.key)}
              >
                <div className="mb-1 flex items-center justify-between">
                  <span
                    className={cn(
                      'tabular flex size-5 items-center justify-center rounded-full text-[11px] font-medium',
                      cell.isToday ? 'bg-primary text-primary-foreground' : cell.inMonth ? 'text-foreground' : 'text-muted-foreground/50'
                    )}
                  >
                    {cell.day}
                  </span>
                  {items.length > 0 && (
                    <span className="tabular text-[10px] text-muted-foreground/70">
                      {done}/{items.length}
                    </span>
                  )}
                </div>
                <div className="flex flex-col gap-1">
                  {items.slice(0, limit).map(task => {
                    const style = CATEGORY_STYLE[task.category]
                    return (
                      <div
                        key={task.id}
                        draggable
                        onDragStart={e => {
                          e.stopPropagation()
                          setDragging(task.id!)
                        }}
                        onDragEnd={() => setDragging(null)}
                        onClick={e => e.stopPropagation()}
                        title={`#${task.number} ${task.title}`}
                        className={cn(
                          'flex items-center gap-1 rounded px-1.5 py-0.5 text-[11px] transition-opacity hover:opacity-80',
                          style.badge,
                          task.status === 'DONE' && 'opacity-50 line-through'
                        )}
                      >
                        <span className="size-1.5 shrink-0 rounded-full" style={{ background: PRIORITY_COLOR[task.priority] }} />
                        <span className="truncate">{task.title}</span>
                        {task.is_today && <span className="ml-auto shrink-0 rounded bg-white/70 px-1 text-[9px]">今</span>}
                      </div>
                    )
                  })}
                  {items.length > limit && (
                    <div className="pl-1 text-[10px] text-muted-foreground">+{items.length - limit} 更多</div>
                  )}
                </div>
              </div>
            )
          })}
        </div>
      </Card>

      <div className="mt-3 flex flex-wrap items-center gap-3 text-[11px] text-muted-foreground">
        {Object.entries(CATEGORY_LABEL).map(([key, label]) => (
          <span key={key} className="inline-flex items-center gap-1.5">
            <i className={cn('size-2 rounded-sm', CATEGORY_STYLE[key as keyof typeof CATEGORY_STYLE].dot)} />
            {label}
          </span>
        ))}
        <span className="ml-auto">拖动卡片到其他日期即可改排期 · 点击日期查看当天清单</span>
      </div>

      <Dialog
        open={!!selectedDate}
        onClose={() => setSelectedDate(null)}
        title={selectedDate ? `${selectedDate} · ${selectedTasks.length} 项` : ''}
        footer={<Button variant="outline" onClick={() => setSelectedDate(null)}>关闭</Button>}
      >
        {selectedTasks.length === 0 && <p className="text-xs text-muted-foreground">这天没有安排。</p>}
        <ul className="divide-y divide-border">
          {selectedTasks.map(task => (
            <li key={task.id} className="flex items-start gap-2.5 py-2.5">
              <input
                type="checkbox"
                className="mt-1 size-3.5 accent-[hsl(var(--primary))]"
                checked={task.status === 'DONE'}
                onChange={() => toggleDone(task)}
              />
              <div className="min-w-0 flex-1">
                <div className="flex flex-wrap items-center gap-1.5">
                  <span className="tabular text-[11px] text-muted-foreground/70">#{task.number}</span>
                  <span className={cn('text-[13px] font-medium', task.status === 'DONE' && 'text-muted-foreground line-through')}>
                    {task.title}
                  </span>
                  <Badge className={CATEGORY_STYLE[task.category].badge}>{CATEGORY_LABEL[task.category]}</Badge>
                </div>
                <div className="mt-1 flex flex-wrap items-center gap-x-3 text-[11px] text-muted-foreground">
                  {task.project_name && <span>{task.project_name}</span>}
                  {task.due_date && <span className="tabular">截止 {task.due_date}</span>}
                  {hoursPair(task.actual_hours, task.estimated_hours) && (
                    <span className="tabular text-amber-700">{hoursPair(task.actual_hours, task.estimated_hours)}</span>
                  )}
                </div>
              </div>
              <span className="shrink-0 text-[11px] text-muted-foreground">{STATUS_LABEL[task.status as TaskStatus]}</span>
            </li>
          ))}
        </ul>
      </Dialog>
    </div>
  )
}
