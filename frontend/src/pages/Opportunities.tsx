import { useEffect, useState } from 'react'
import { ArrowRight, Briefcase, GripVertical, Plus, Trash } from 'lucide-react'
import { projectApi, taskApi } from '../services/api'
import type { OpportunityStatus, Project, Task } from '../types'
import PageHeader from '../components/PageHeader'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { Dialog } from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Select } from '@/components/ui/select'
import { Textarea } from '@/components/ui/textarea'
import { cn } from '@/lib/utils'
import { OPPORTUNITY_LABEL, OPPORTUNITY_ORDER, PRIORITY_COLOR, PRIORITY_LABEL } from '../components/taskMeta'

const STAGE_STYLE: Record<OpportunityStatus, string> = {
  lead: 'border-t-slate-400',
  qualifying: 'border-t-blue-500',
  proposal: 'border-t-amber-500',
  won: 'border-t-emerald-500',
  lost: 'border-t-red-400',
  dropped: 'border-t-slate-300'
}

const emptyForm = (): Task => ({
  title: '',
  content: '',
  category: 'opportunity',
  status: 'TODO',
  opportunity_status: 'lead',
  priority: 'MEDIUM'
})

export default function Opportunities() {
  const [items, setItems] = useState<Task[]>([])
  const [projects, setProjects] = useState<Project[]>([])
  const [showForm, setShowForm] = useState(false)
  const [form, setForm] = useState<Task>(emptyForm())
  // 拖拽改阶段：dragging 是被拖的商机 id，dragOver 是当前悬停的阶段
  const [dragging, setDragging] = useState<number | null>(null)
  const [dragOver, setDragOver] = useState<OpportunityStatus | null>(null)

  useEffect(() => {
    loadAll()
  }, [])

  const loadAll = async () => {
    const [tasks, projectList] = await Promise.all([
      taskApi.list({ category: 'opportunity' }),
      projectApi.list()
    ])
    setItems(tasks || [])
    setProjects(projectList || [])
  }

  const submit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (form.id) {
      await taskApi.update(form.id, form)
    } else {
      await taskApi.create(form)
    }
    setShowForm(false)
    setForm(emptyForm())
    await loadAll()
  }

  const moveTo = async (task: Task, opportunity_status: OpportunityStatus) => {
    const current = task.opportunity_status || 'lead'
    if (current === opportunity_status) return
    // 赢单自动置为已完成，从赢单拖回来则恢复未开始
    const status =
      opportunity_status === 'won' ? 'DONE' : current === 'won' && task.status === 'DONE' ? 'TODO' : task.status
    // 先本地改，拖完立刻见到效果；后端失败再整体重载纠偏
    setItems(prev =>
      prev.map(item => (item.id === task.id ? { ...item, opportunity_status, status } : item))
    )
    try {
      await taskApi.update(task.id!, { ...task, opportunity_status, status })
    } catch (error) {
      console.error('Failed to move opportunity:', error)
    } finally {
      await loadAll()
    }
  }

  const remove = async (id: number) => {
    if (!confirm('确定删除该商机?')) return
    await taskApi.remove(id)
    await loadAll()
  }

  const convertToProject = async (task: Task) => {
    const name = prompt('新项目名称', task.title)
    if (!name) return
    const project = await projectApi.create({
      name,
      customer: '',
      description: task.content || '',
      status: 'active',
      amount: 0
    })
    await taskApi.update(task.id!, {
      ...task,
      project_id: project.id,
      opportunity_status: 'won',
      status: 'DONE'
    })
    alert(`已创建项目「${project.name}」并关联`)
    await loadAll()
  }

  const won = items.filter(i => i.opportunity_status === 'won').length
  const open = items.filter(i => !['won', 'lost', 'dropped'].includes(i.opportunity_status || 'lead')).length

  return (
    <div>
      <PageHeader
        eyebrow="OPPORTUNITIES"
        title="商机"
        description="线索 → 跟进 → 报价 → 赢单，卡片直接拖到下一列，赢单后一键转成项目。"
        actions={
          <Button onClick={() => { setForm(emptyForm()); setShowForm(true) }}>
            <Plus />
            新建商机
          </Button>
        }
      />

      <div className="mb-4 grid gap-3 sm:grid-cols-3">
        <Card className="px-4 py-3">
          <div className="text-[11px] text-muted-foreground">在跟</div>
          <div className="tabular mt-1 text-2xl font-semibold text-blue-600">{open}</div>
        </Card>
        <Card className="px-4 py-3">
          <div className="text-[11px] text-muted-foreground">已赢单</div>
          <div className="tabular mt-1 text-2xl font-semibold text-emerald-600">{won}</div>
        </Card>
        <Card className="px-4 py-3">
          <div className="text-[11px] text-muted-foreground">转化率</div>
          <div className="tabular mt-1 text-2xl font-semibold">
            {items.length ? Math.round((won / items.length) * 100) : 0}%
          </div>
        </Card>
      </div>

      <div className="grid grid-cols-2 gap-3 xl:grid-cols-3 2xl:grid-cols-6">
        {OPPORTUNITY_ORDER.map(stage => {
          const stageItems = items.filter(i => (i.opportunity_status || 'lead') === stage)
          return (
            <div key={stage} className="min-w-0">
              <div
                className={cn(
                  'mb-2 flex items-center justify-between rounded-t-md border-t-2 bg-card px-2.5 py-1.5',
                  STAGE_STYLE[stage]
                )}
              >
                <span className="text-[12px] font-semibold">{OPPORTUNITY_LABEL[stage]}</span>
                <span className="tabular text-[11px] text-muted-foreground">{stageItems.length}</span>
              </div>
              <div
                className={cn(
                  'flex min-h-[64px] flex-col gap-2 rounded-md border border-transparent p-1 transition-colors',
                  dragOver === stage && dragging != null && 'border-dashed border-primary/50 bg-accent/60'
                )}
                onDragOver={e => {
                  e.preventDefault()
                  setDragOver(stage)
                }}
                onDragLeave={() => setDragOver(prev => (prev === stage ? null : prev))}
                onDrop={e => {
                  e.preventDefault()
                  setDragOver(null)
                  // 拖得快的时候 state 还没生效，优先从 dataTransfer 取 id
                  const id = Number(e.dataTransfer.getData('text/plain')) || dragging
                  setDragging(null)
                  const dragged = items.find(i => i.id === id)
                  if (dragged) moveTo(dragged, stage)
                }}
              >
                {stageItems.map(item => (
                  <Card
                    key={item.id}
                    draggable
                    onDragStart={e => {
                      e.dataTransfer.effectAllowed = 'move'
                      e.dataTransfer.setData('text/plain', String(item.id))
                      setDragging(item.id!)
                    }}
                    onDragEnd={() => {
                      setDragging(null)
                      setDragOver(null)
                    }}
                    className={cn(
                      'group cursor-grab p-2.5 active:cursor-grabbing',
                      dragging === item.id && 'opacity-50 ring-2 ring-primary/40'
                    )}
                  >
                    <div className="flex items-start gap-1.5">
                      <GripVertical className="mt-0.5 size-3 shrink-0 text-muted-foreground/40 group-hover:text-muted-foreground" />
                      <span className="tabular mt-0.5 text-[10.5px] text-muted-foreground/70">#{item.number}</span>
                      <span className="min-w-0 flex-1 text-[12.5px] font-medium leading-snug">{item.title}</span>
                      <Button
                        variant="ghost"
                        size="icon-sm"
                        className="shrink-0 text-muted-foreground opacity-0 hover:bg-red-50 hover:text-red-600 group-hover:opacity-100"
                        onClick={() => remove(item.id!)}
                      >
                        <Trash />
                      </Button>
                    </div>
                    <div className="mt-1.5 flex flex-wrap items-center gap-1.5">
                      <span
                        className="inline-flex items-center gap-1 text-[10.5px]"
                        style={{ color: PRIORITY_COLOR[item.priority] }}
                      >
                        <i className="size-1.5 rounded-full bg-current" />
                        {PRIORITY_LABEL[item.priority]}
                      </span>
                      {item.project_name && <Badge variant="muted">{item.project_name}</Badge>}
                    </div>
                    <Select
                      className="mt-2 h-7 text-[11px]"
                      value={item.opportunity_status || 'lead'}
                      onChange={e => moveTo(item, e.target.value as OpportunityStatus)}
                    >
                      {OPPORTUNITY_ORDER.map(value => (
                        <option key={value} value={value}>{OPPORTUNITY_LABEL[value]}</option>
                      ))}
                    </Select>
                    {['lead', 'qualifying', 'proposal'].includes(stage) && (
                      <Button
                        variant="outline"
                        size="xs"
                        className="mt-1.5 w-full justify-center"
                        onClick={() => convertToProject(item)}
                      >
                        <Briefcase />
                        转为项目
                      </Button>
                    )}
                  </Card>
                ))}
                {stageItems.length === 0 && (
                  <div className="rounded-md border border-dashed border-border py-5 text-center text-[11px] text-muted-foreground/70">
                    空
                  </div>
                )}
              </div>
            </div>
          )
        })}
      </div>

      <Dialog
        open={showForm}
        onClose={() => setShowForm(false)}
        title="新建商机"
        description="先记下一句话需求，转化率是这里唯一的指标。"
        footer={
          <>
            <Button variant="outline" onClick={() => setShowForm(false)}>取消</Button>
            <Button type="submit" form="opp-form">
              保存
              <ArrowRight />
            </Button>
          </>
        }
      >
        <form id="opp-form" onSubmit={submit} className="space-y-3.5">
          <div>
            <Label>标题 *</Label>
            <Input value={form.title} onChange={e => setForm({ ...form, title: e.target.value })} required />
          </div>
          <div className="grid gap-3 sm:grid-cols-3">
            <div>
              <Label>阶段</Label>
              <Select
                value={form.opportunity_status || 'lead'}
                onChange={e => setForm({ ...form, opportunity_status: e.target.value as OpportunityStatus })}
              >
                {OPPORTUNITY_ORDER.map(value => (
                  <option key={value} value={value}>{OPPORTUNITY_LABEL[value]}</option>
                ))}
              </Select>
            </div>
            <div>
              <Label>优先级</Label>
              <Select
                value={form.priority}
                onChange={e => setForm({ ...form, priority: e.target.value as Task['priority'] })}
              >
                {Object.entries(PRIORITY_LABEL).map(([value, label]) => (
                  <option key={value} value={value}>{label}</option>
                ))}
              </Select>
            </div>
            <div>
              <Label>关联项目</Label>
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
          </div>
          <div>
            <Label>备注</Label>
            <Textarea rows={4} value={form.content || ''} onChange={e => setForm({ ...form, content: e.target.value })} />
          </div>
        </form>
      </Dialog>
    </div>
  )
}
