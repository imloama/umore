import { cn } from '@/lib/utils'
import { useI18n } from '@/i18n'
import { formatHours } from './taskMeta'

/**
 * 今日工时偏差提示：评估是计划，实际是当天真正投进去的。
 * 只在两边都有数的时候才给结论，缺一边就只报数不下判断。
 */
export function hoursDeviation(actual: number, estimated: number) {
  const a = actual || 0
  const e = estimated || 0
  const delta = Math.round((a - e) * 100) / 100
  const ratio = e > 0 ? a / e : 0
  const hasBoth = a > 0 && e > 0

  let level: 'none' | 'ok' | 'over' | 'far-over' = 'none'
  if (hasBoth) {
    // 超 25% 以上算明显失控，标红；刚超一点只提醒
    if (ratio >= 1.25) level = 'far-over'
    else if (ratio > 1) level = 'over'
    else level = 'ok'
  }
  return { actual: a, estimated: e, delta, ratio, hasBoth, level }
}

export default function HoursDelta({
  actual,
  estimated,
  compact = false
}: {
  actual: number
  estimated: number
  compact?: boolean
}) {
  const { t } = useI18n()
  const { delta, ratio, hasBoth, level } = hoursDeviation(actual, estimated)

  if (!hasBoth) {
    return (
      <span className="text-[11px] text-muted-foreground">
        {t('hours.actual', { hours: formatHours(actual) || '0h' })} /{' '}
        {t('hours.estimate', { hours: formatHours(estimated) || t('hours.notSet') })}
      </span>
    )
  }

  const over = delta > 0
  const percent = Math.round(Math.abs(ratio - 1) * 100)
  const hours = formatHours(Math.abs(delta))
  const label = over ? t('hours.over', { hours }) : t('hours.saved', { hours })

  return (
    <span
      className={cn(
        'inline-flex items-center gap-1 rounded-md px-1.5 py-0.5 text-[11px] font-medium',
        level === 'far-over' && 'bg-red-50 text-red-700',
        level === 'over' && 'bg-amber-50 text-amber-700',
        level === 'ok' && 'bg-emerald-50 text-emerald-700'
      )}
      title={t('hours.deviationTitle', {
        estimate: formatHours(estimated),
        actual: formatHours(actual),
        delta: `${over ? '+' : '-'}${percent}%`
      })}
    >
      {label}
      {!compact && <span className="opacity-70">({over ? '+' : '-'}{percent}%)</span>}
    </span>
  )
}
