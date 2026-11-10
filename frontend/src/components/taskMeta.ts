import type { OpportunityStatus, TaskCategory, TaskPriority, TaskStatus } from '../types'

export const CATEGORY_LABEL: Record<TaskCategory, string> = {
  todo: '待办',
  issue: '问题',
  requirement: '需求',
  opportunity: '商机'
}

export const STATUS_LABEL: Record<TaskStatus, string> = {
  TODO: '未开始',
  DOING: '进行中',
  BLOCKED: '阻塞',
  DONE: '已完成'
}

export const STATUS_ORDER: TaskStatus[] = ['TODO', 'DOING', 'BLOCKED', 'DONE']

export const PRIORITY_LABEL: Record<TaskPriority, string> = {
  LOW: '低',
  MEDIUM: '中',
  HIGH: '高',
  URGENT: '紧急'
}

export const PRIORITY_COLOR: Record<TaskPriority, string> = {
  LOW: '#52c41a',
  MEDIUM: '#fa8c16',
  HIGH: '#ff4d4f',
  URGENT: '#cf1322'
}

export const OPPORTUNITY_LABEL: Record<OpportunityStatus, string> = {
  lead: '线索',
  qualifying: '跟进中',
  proposal: '已报价',
  won: '赢单',
  lost: '输单',
  dropped: '放弃'
}

export const OPPORTUNITY_ORDER: OpportunityStatus[] = [
  'lead',
  'qualifying',
  'proposal',
  'won',
  'lost',
  'dropped'
]

export const DEFAULT_LABEL_COLORS = [
  '#1890ff',
  '#52c41a',
  '#fa8c16',
  '#ff4d4f',
  '#722ed1',
  '#13c2c2',
  '#eb2f96'
]

export const todayStr = () => {
  const d = new Date()
  const m = `${d.getMonth() + 1}`.padStart(2, '0')
  const day = `${d.getDate()}`.padStart(2, '0')
  return `${d.getFullYear()}-${m}-${day}`
}

/** 工时展示：4.5h；未填返回空串 */
export const formatHours = (n?: number) => {
  if (!n || n <= 0) return ''
  const rounded = Math.round(n * 100) / 100
  return `${rounded}h`
}

/** 实际/评估，如「3h / 5h」；两边都没填返回空串 */
export const hoursPair = (actual?: number, estimated?: number) => {
  const a = formatHours(actual)
  const e = formatHours(estimated)
  if (!a && !e) return ''
  if (!e) return `实际 ${a}`
  if (!a) return `评估 ${e}`
  return `${a} / ${e}`
}

/** 类型标记：按语义色给底色，日历/列表/看板共用一套 */
export const CATEGORY_STYLE: Record<TaskCategory, { dot: string; badge: string; bar: string }> = {
  todo: { dot: 'bg-primary', badge: 'bg-blue-50 text-blue-700', bar: 'bg-blue-500' },
  issue: { dot: 'bg-red-500', badge: 'bg-red-50 text-red-700', bar: 'bg-red-500' },
  requirement: { dot: 'bg-amber-500', badge: 'bg-amber-50 text-amber-700', bar: 'bg-amber-500' },
  opportunity: { dot: 'bg-violet-500', badge: 'bg-violet-50 text-violet-700', bar: 'bg-violet-500' }
}

export const STATUS_STYLE: Record<TaskStatus, string> = {
  TODO: 'bg-slate-100 text-slate-600',
  DOING: 'bg-blue-50 text-blue-700',
  BLOCKED: 'bg-red-50 text-red-700',
  DONE: 'bg-emerald-50 text-emerald-700'
}

export const STATUS_DOT: Record<TaskStatus, string> = {
  TODO: 'bg-slate-400',
  DOING: 'bg-blue-500',
  BLOCKED: 'bg-red-500',
  DONE: 'bg-emerald-500'
}
