import type { Receivable } from '../types'

/**
 * 金额展示：只保留有效小数位。
 * 整数不写 .00（合同 50000 看着就是 5 万），角分不清零；浮点尾巴一律截到分。
 */
export const money = (n?: number) => {
  const v = Math.round((n || 0) * 100) / 100
  return `¥${Number.isInteger(v) ? v : v.toFixed(2).replace(/0+$/, '').replace(/\.$/, '')}`
}

/**
 * 金额求和：先按分取整再累加。
 * 直接 reduce 浮点会攒出 0.30000000000000004 这种尾巴，合计和明细对不上账。
 */
export const sumMoney = (list: { amount?: number }[]) =>
  list.reduce((s, item) => s + Math.round((item.amount || 0) * 100), 0) / 100

export const sumAmounts = (amounts: number[]) => sumMoney(amounts.map(amount => ({ amount })))

/** 到期描述：逾期的说人话，别让人自己算 */
export const dueText = (item: Receivable) => {
  if (!item.due_date) return '未设到期时间'
  if (item.status === 'done') return item.due_date
  const left = item.days_left ?? 0
  if (item.overdue) return `逾期 ${Math.abs(left)} 天`
  if (left === 0) return '今天到期'
  if (left === 1) return '明天到期'
  return `还有 ${left} 天`
}

/** 到期配色：逾期红、7 天内橙、其余灰 */
export const dueTone = (item: Receivable) => {
  if (item.status === 'done') return 'text-muted-foreground'
  if (item.overdue) return 'text-red-600'
  if (item.due_date && (item.days_left ?? 99) <= 7) return 'text-amber-600'
  return 'text-muted-foreground'
}

/** ISO 时间戳取日期部分，已收那行显示「哪天收到的」 */
export const dayOf = (value?: string) => {
  if (!value) return ''
  const d = new Date(value)
  if (Number.isNaN(d.getTime())) return value.slice(0, 10)
  const p = (n: number) => `${n}`.padStart(2, '0')
  return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())}`
}

export const emptyReceivable = (projectId?: number): Partial<Receivable> => ({
  project_id: projectId,
  title: '',
  amount: 0,
  due_date: '',
  note: '',
  status: 'pending'
})
