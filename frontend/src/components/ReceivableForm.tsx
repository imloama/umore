import { useEffect, useState } from 'react'
import type { Project, Receivable } from '../types'
import { errorCodeOf, receivableApi } from '../services/api'
import { useI18n } from '@/i18n'
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
  const { t, te } = useI18n()
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
      setError(t('receivable.form.amountInvalid'))
      return
    }
    setSaving(true)
    setError('')
    try {
      const payload: Partial<Receivable> = {
        project_id: form.project_id ?? undefined,
        // 标题留空就用默认名，默认名也跟着界面语言走
        title: (form.title || '').trim() || t('receivable.form.defaultTitle'),
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
      const { code } = errorCodeOf(err)
      setError(code ? te(code) : t('receivable.form.saveFailed'))
    } finally {
      setSaving(false)
    }
  }

  return (
    <Dialog
      open={open}
      onClose={onClose}
      className="max-w-lg"
      title={form.id ? t('receivable.form.editTitle') : t('receivable.form.title')}
      description={t('receivable.form.description')}
      footer={
        <>
          <Button variant="outline" onClick={onClose}>{t('common.cancel')}</Button>
          <Button type="submit" form="receivable-form" disabled={saving}>
            {saving ? t('common.saving') : t('common.save')}
          </Button>
        </>
      }
    >
      <form id="receivable-form" onSubmit={submit} className="space-y-3.5">
        <div>
          <Label>{t('receivable.form.what')}</Label>
          <Input
            value={form.title || ''}
            onChange={e => setForm({ ...form, title: e.target.value })}
            placeholder={t('receivable.form.whatPlaceholder')}
            autoFocus
          />
        </div>
        <div className="grid gap-3 sm:grid-cols-2">
          <div>
            <Label>{t('receivable.form.amount')}</Label>
            <Input
              type="number"
              step="0.01"
              min="0"
              value={form.amount ?? 0}
              onChange={e => setForm({ ...form, amount: parseFloat(e.target.value) || 0 })}
            />
          </div>
          <div>
            <Label>{t('receivable.form.due')}</Label>
            <Input
              type="date"
              value={form.due_date || ''}
              onChange={e => setForm({ ...form, due_date: e.target.value })}
            />
          </div>
        </div>
        {!lockProject && projects && (
          <div>
            <Label>{t('receivable.form.project')}</Label>
            <Select
              value={form.project_id ? String(form.project_id) : ''}
              onChange={e => setForm({ ...form, project_id: e.target.value ? Number(e.target.value) : undefined })}
            >
              <option value="">{t('receivable.form.projectNone')}</option>
              {projects.map(p => (
                <option key={p.id} value={p.id}>
                  {p.name}
                </option>
              ))}
            </Select>
          </div>
        )}
        <div>
          <Label>{t('receivable.form.note')}</Label>
          <Textarea
            rows={3}
            value={form.note || ''}
            onChange={e => setForm({ ...form, note: e.target.value })}
            placeholder={t('receivable.form.notePlaceholder')}
          />
        </div>
        {error && <div className="text-[11px] text-red-600">{error}</div>}
      </form>
    </Dialog>
  )
}
