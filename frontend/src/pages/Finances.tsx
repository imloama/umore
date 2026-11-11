import { useEffect, useMemo, useState } from 'react'
import { ArrowDownLeft, ArrowUpRight, Plus, Receipt, Trash, Wallet } from 'lucide-react'
import api from '../services/api'
import type { Finance } from '../types'
import { useI18n } from '@/i18n'
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

const today = () => new Date().toISOString().split('T')[0]

const emptyForm = (): Partial<Finance> => ({
  type: 'income',
  amount: 0,
  category: '',
  description: '',
  date: today()
})

export default function Finances() {
  const { t } = useI18n()
  const [finances, setFinances] = useState<Finance[]>([])
  const [summary, setSummary] = useState({ total_income: 0, total_expense: 0, balance: 0 })
  const [startDate, setStartDate] = useState(() => {
    const d = new Date()
    return new Date(d.getFullYear(), d.getMonth(), 1).toISOString().split('T')[0]
  })
  const [endDate, setEndDate] = useState(today)
  const [showForm, setShowForm] = useState(false)
  const [form, setForm] = useState<Partial<Finance>>(emptyForm())

  useEffect(() => {
    loadFinances()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [startDate, endDate])

  const loadFinances = async () => {
    try {
      const [data, summaryData] = await Promise.all([
        api.get<any, Finance[]>('/finances', { params: { start_date: startDate, end_date: endDate } }),
        api.get<any, { total_income: number; total_expense: number; balance: number }>('/finances/summary', {
          params: { start_date: startDate, end_date: endDate }
        })
      ])
      setFinances(data || [])
      setSummary(summaryData)
    } catch (error) {
      console.error('Failed to load finances:', error)
    }
  }

  const submit = async (e: React.FormEvent) => {
    e.preventDefault()
    try {
      if (form.id) {
        await api.put(`/finances/${form.id}`, form)
      } else {
        await api.post('/finances', form)
      }
      setShowForm(false)
      setForm(emptyForm())
      loadFinances()
    } catch (error) {
      console.error('Failed to save finance:', error)
    }
  }

  const remove = async (id: number) => {
    if (!confirm(t('finance.confirmDelete'))) return
    try {
      await api.delete(`/finances/${id}`)
      loadFinances()
    } catch (error) {
      console.error('Failed to delete finance:', error)
    }
  }

  const maxAmount = useMemo(() => Math.max(1, ...finances.map(f => f.amount)), [finances])

  return (
    <div>
      <PageHeader
        eyebrow="FINANCE"
        title={t('finance.title')}
        description={t('finance.description')}
        actions={
          <>
            <div className="flex items-center gap-1.5">
              <Input
                type="date"
                value={startDate}
                onChange={e => setStartDate(e.target.value)}
                className="w-[140px]"
              />
              <span className="text-xs text-muted-foreground">{t('finance.to')}</span>
              <Input type="date" value={endDate} onChange={e => setEndDate(e.target.value)} className="w-[140px]" />
            </div>
            <Button onClick={() => { setForm(emptyForm()); setShowForm(true) }}>
              <Plus />
              {t('finance.new')}
            </Button>
          </>
        }
      />

      <div className="mb-4 grid gap-3 sm:grid-cols-3">
        <Card className="px-4 py-3.5">
          <div className="flex items-center justify-between text-[11px] text-muted-foreground">
            <span>{t('finance.income')}</span>
            <ArrowDownLeft className="size-4 text-emerald-500" />
          </div>
          <div className="tabular mt-1.5 text-2xl font-semibold text-emerald-600">
            ¥{summary.total_income.toFixed(2)}
          </div>
        </Card>
        <Card className="px-4 py-3.5">
          <div className="flex items-center justify-between text-[11px] text-muted-foreground">
            <span>{t('finance.expense')}</span>
            <ArrowUpRight className="size-4 text-red-500" />
          </div>
          <div className="tabular mt-1.5 text-2xl font-semibold text-red-600">
            ¥{summary.total_expense.toFixed(2)}
          </div>
        </Card>
        <Card
          className={cn(
            'px-4 py-3.5',
            summary.balance >= 0 ? 'border-emerald-200 bg-emerald-50/40' : 'border-red-200 bg-red-50/40'
          )}
        >
          <div className="flex items-center justify-between text-[11px] text-muted-foreground">
            <span>{t('finance.balance')}</span>
            <Wallet className="size-4 text-primary" />
          </div>
          <div
            className={cn(
              'tabular mt-1.5 text-2xl font-semibold',
              summary.balance >= 0 ? 'text-emerald-700' : 'text-red-700'
            )}
          >
            ¥{summary.balance.toFixed(2)}
          </div>
        </Card>
      </div>

      <Card className="overflow-hidden">
        <div className="flex items-center justify-between border-b border-border px-4 py-2.5">
          <span className="text-[13px] font-semibold">
            {t('finance.flows')}
            <span className="ml-1 text-[11px] font-normal text-muted-foreground">
              {t('finance.flowCount', { n: finances.length })}
            </span>
          </span>
        </div>
        {finances.length === 0 ? (
          <div className="flex flex-col items-center gap-2 px-4 py-12 text-center">
            <Receipt className="size-6 text-muted-foreground/60" />
            <div className="text-xs text-muted-foreground">{t('finance.empty')}</div>
          </div>
        ) : (
          <ul className="divide-y divide-border">
            {finances.map(finance => (
              <li key={finance.id} className="group flex items-center gap-3 px-4 py-2.5 hover:bg-secondary/40">
                <span
                  className={cn(
                    'flex size-7 shrink-0 items-center justify-center rounded-full',
                    finance.type === 'income' ? 'bg-emerald-50 text-emerald-600' : 'bg-red-50 text-red-600'
                  )}
                >
                  {finance.type === 'income' ? (
                    <ArrowDownLeft className="size-3.5" />
                  ) : (
                    <ArrowUpRight className="size-3.5" />
                  )}
                </span>
                <div className="min-w-0 flex-1">
                  <div className="flex items-center gap-1.5">
                    <span className="truncate text-[13px] font-medium">
                      {finance.description || finance.category || t('finance.unnamed')}
                    </span>
                    {finance.category && <Badge variant="outline">{finance.category}</Badge>}
                  </div>
                  <div className="tabular mt-0.5 text-[11px] text-muted-foreground">{finance.date}</div>
                </div>
                <div className="hidden w-24 shrink-0 sm:block">
                  <div className="h-1.5 overflow-hidden rounded-full bg-secondary">
                    <div
                      className={cn('h-full rounded-full', finance.type === 'income' ? 'bg-emerald-500' : 'bg-red-400')}
                      style={{ width: `${Math.max(4, (finance.amount / maxAmount) * 100)}%` }}
                    />
                  </div>
                </div>
                <span
                  className={cn(
                    'tabular w-24 shrink-0 text-right text-[13px] font-semibold',
                    finance.type === 'income' ? 'text-emerald-600' : 'text-red-600'
                  )}
                >
                  {finance.type === 'income' ? '+' : '-'}¥{finance.amount.toFixed(2)}
                </span>
                <Button
                  variant="ghost"
                  size="icon-sm"
                  className="shrink-0 text-muted-foreground opacity-0 hover:bg-red-50 hover:text-red-600 group-hover:opacity-100"
                  onClick={() => remove(finance.id!)}
                >
                  <Trash />
                </Button>
              </li>
            ))}
          </ul>
        )}
      </Card>

      <Dialog
        open={showForm}
        onClose={() => setShowForm(false)}
        title={form.id ? t('finance.editTitle') : t('finance.new')}
        className="max-w-lg"
        footer={
          <>
            <Button variant="outline" onClick={() => setShowForm(false)}>{t('common.cancel')}</Button>
            <Button type="submit" form="finance-form">{t('common.save')}</Button>
          </>
        }
      >
        <form id="finance-form" onSubmit={submit} className="space-y-3.5">
          <div className="grid gap-3 sm:grid-cols-2">
            <div>
              <Label>{t('finance.form.type')}</Label>
              <Select value={form.type} onChange={e => setForm({ ...form, type: e.target.value as Finance['type'] })}>
                <option value="income">{t('finance.income')}</option>
                <option value="expense">{t('finance.expense')}</option>
              </Select>
            </div>
            <div>
              <Label>{t('finance.form.amount')}</Label>
              <Input
                type="number"
                step="0.01"
                value={form.amount}
                onChange={e => setForm({ ...form, amount: parseFloat(e.target.value) })}
                required
              />
            </div>
          </div>
          <div className="grid gap-3 sm:grid-cols-2">
            <div>
              <Label>{t('finance.form.category')}</Label>
              <Input value={form.category || ''} onChange={e => setForm({ ...form, category: e.target.value })} />
            </div>
            <div>
              <Label>{t('finance.form.date')}</Label>
              <Input type="date" value={form.date} onChange={e => setForm({ ...form, date: e.target.value })} required />
            </div>
          </div>
          <div>
            <Label>{t('finance.form.description')}</Label>
            <Textarea rows={2} value={form.description || ''} onChange={e => setForm({ ...form, description: e.target.value })} />
          </div>
        </form>
      </Dialog>
    </div>
  )
}
