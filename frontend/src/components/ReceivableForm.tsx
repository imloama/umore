import { useEffect, useState } from 'react'
import type { Project, Receivable } from '../types'
import { receivableApi } from '../services/api'
import { Button } from '@/components/ui/button'
import { Dialog } from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Select } from '@/components/ui/select'
import { Textarea } from '@/components/ui/textarea'
import { emptyReceivable } from './receivableMeta'

/**
 * 记一笔待收 / 改这一笔。v1 就三个字段有用：金额、到期时间、备注。
 * 备注本来最常改，所以这里随便改，随时保存。
 */
export default function ReceivableForm({
  open,
  item,
  projects,
  defaultProjectId,
  lockProject = false,
  onClose,
  onSaved
}: {
  open: boolean
  item?: Receivable | null
  projects?: Project[]
  defaultProjectId?: number
  lockProject?: boolean
  onClose: () => void
  onSaved: (saved: Receivable) => void
}) {
  const [form, setForm] = useState<Partial<Receivable>>(emptyReceivable(defaultProjectId))
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')

  useEffect(() => {
    if (!open) return
    setForm(item ? { ...item } : emptyReceivable(defaultProjectId))
    setError('')
  }, [open, item, defaultProjectId])

  const submit = async (e: React.FormEvent) => {
    e.preventDefault()
    const amount = Number(form.amount)
    if (!Number.isFinite(amount) || amount < 0) {
      setError('金额要填个非负数字')
      return
    }
    setSaving(true)
    setError('')
    try {
      const payload: Partial<Receivable> = {
        project_id: form.project_id ?? undefined,
        title: (form.title || '').trim() || '待收款',
        amount,
        due_date: form.due_date || '',
        note: (form.note || '').trim(),
        status: form.status || 'pending'
      }
      const saved = form.id
        ? await receivableApi.update(form.id, payload)
        : await receivableApi.create(payload)
      onSaved(saved)
      onClose()
    } catch (err) {
      console.error('Failed to save receivable:', err)
      setError('保存失败，看看后端还在不在')
    } finally {
      setSaving(false)
    }
  }

  return (
    <Dialog
      open={open}
      onClose={onClose}
      className="max-w-lg"
      title={form.id ? '改这一笔' : '记一笔待收'}
      description="只是记一下：多少钱、什么时候该收、备注。到期我来提醒你。"
      footer={
        <>
          <Button variant="outline" onClick={onClose}>取消</Button>
          <Button type="submit" form="receivable-form" disabled={saving}>
            {saving ? '保存中…' : '保存'}
          </Button>
        </>
      }
    >
      <form id="receivable-form" onSubmit={submit} className="space-y-3.5">
        <div>
          <Label>收什么钱</Label>
          <Input
            value={form.title || ''}
            onChange={e => setForm({ ...form, title: e.target.value })}
            placeholder="首付款 / 二期款 / 尾款"
            autoFocus
          />
        </div>
        <div className="grid gap-3 sm:grid-cols-2">
          <div>
            <Label>金额（元）*</Label>
            <Input
              type="number"
              step="0.01"
              min="0"
              value={form.amount ?? 0}
              onChange={e => setForm({ ...form, amount: parseFloat(e.target.value) || 0 })}
            />
          </div>
          <div>
            <Label>到期时间</Label>
            <Input
              type="date"
              value={form.due_date || ''}
              onChange={e => setForm({ ...form, due_date: e.target.value })}
            />
          </div>
        </div>
        {!lockProject && projects && (
          <div>
            <Label>关联项目</Label>
            <Select
              value={form.project_id ? String(form.project_id) : ''}
              onChange={e => setForm({ ...form, project_id: e.target.value ? Number(e.target.value) : undefined })}
            >
              <option value="">不关联（私人垫款也行）</option>
              {projects.map(p => (
                <option key={p.id} value={p.id}>
                  {p.name}
                </option>
              ))}
            </Select>
          </div>
        )}
        <div>
          <Label>备注</Label>
          <Textarea
            rows={3}
            value={form.note || ''}
            onChange={e => setForm({ ...form, note: e.target.value })}
            placeholder="客户说月底前给 / 要先开发票"
          />
        </div>
        {error && <div className="text-[11px] text-red-600">{error}</div>}
      </form>
    </Dialog>
  )
}
