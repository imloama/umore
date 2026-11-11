import type { OpportunityStatus, TaskCategory, TaskPriority, TaskStatus } from '../types'
import type { MessageKey } from '../i18n'

// 枚举值到文案键的映射：值本身是接口契约（落库、筛选全靠它），展示文案查 i18n
export const CATEGORY_KEYS: Record<TaskCategory, MessageKey> = {
  todo: 'category.todo',
  issue: 'category.issue',
  requirement: 'category.requirement',
  opportunity: 'category.opportunity'
}

export const CATEGORY_ORDER: TaskCategory[] = ['todo', 'issue', 'requirement', 'opportunity']

export const STATUS_KEYS: Record<TaskStatus, MessageKey> = {
  TODO: 'status.TODO',
  DOING: 'status.DOING',
  BLOCKED: 'status.BLOCKED',
  DONE: 'status.DONE'
}

export const STATUS_ORDER: TaskStatus[] = ['TODO', 'DOING', 'BLOCKED', 'DONE']

export const PRIORITY_KEYS: Record<TaskPriority, MessageKey> = {
  LOW: 'priority.LOW',
  MEDIUM: 'priority.MEDIUM',
  HIGH: 'priority.HIGH',
  URGENT: 'priority.URGENT'
}

export const PRIORITY_ORDER: TaskPriority[] = ['LOW', 'MEDIUM', 'HIGH', 'URGENT']

export const PRIORITY_COLOR: Record<TaskPriority, string> = {
  LOW: '#52c41a',
  MEDIUM: '#fa8c16',
  HIGH: '#ff4d4f',
  URGENT: '#cf1322'
}

export const OPPORTUNITY_KEYS: Record<OpportunityStatus, MessageKey> = {
  lead: 'opportunity.lead',
  qualifying: 'opportunity.qualifying',
  proposal: 'opportunity.proposal',
  won: 'opportunity.won',
  lost: 'opportunity.lost',
  dropped: 'opportunity.dropped'
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

/** 工时展示：4.5h；未填返回空串（单位不翻译，中日英都认 h） */
export const formatHours = (n?: number) => {
  if (!n || n <= 0) return ''
  const rounded = Math.round(n * 100) / 100
  return `${rounded}h`
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
