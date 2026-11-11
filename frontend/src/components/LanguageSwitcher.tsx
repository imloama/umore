import { useEffect, useRef, useState } from 'react'
import { Check, Languages } from 'lucide-react'
import { LOCALE_LABEL, SUPPORTED_LOCALES, useI18n } from '@/i18n'
import { cn } from '@/lib/utils'

/**
 * 语言切换。选完立刻生效并写进 localStorage——下次打开就用这个，
 * 没选过才看浏览器语言，再没有就英文。
 */
export default function LanguageSwitcher({ compact = false }: { compact?: boolean }) {
  const { locale, setLocale, t } = useI18n()
  const [open, setOpen] = useState(false)
  const box = useRef<HTMLDivElement>(null)

  useEffect(() => {
    if (!open) return
    const onClick = (e: MouseEvent) => {
      if (box.current && !box.current.contains(e.target as Node)) setOpen(false)
    }
    const onKey = (e: KeyboardEvent) => e.key === 'Escape' && setOpen(false)
    document.addEventListener('mousedown', onClick)
    document.addEventListener('keydown', onKey)
    return () => {
      document.removeEventListener('mousedown', onClick)
      document.removeEventListener('keydown', onKey)
    }
  }, [open])

  return (
    <div className="relative" ref={box}>
      <button
        type="button"
        onClick={() => setOpen(v => !v)}
        title={t('nav.switchLanguage')}
        aria-label={t('nav.switchLanguage')}
        className={cn(
          'flex h-8 items-center gap-1.5 rounded-md border border-input bg-card px-2 text-[12.5px] text-muted-foreground shadow-sm transition-colors hover:text-foreground',
          open && 'border-ring text-foreground'
        )}
      >
        <Languages className="size-3.5" />
        {!compact && <span>{LOCALE_LABEL[locale]}</span>}
      </button>
      {open && (
        <div className="absolute right-0 top-9 z-[90] w-32 overflow-hidden rounded-md border border-border bg-card py-1 shadow-lg">
          {SUPPORTED_LOCALES.map(code => (
            <button
              key={code}
              type="button"
              onClick={() => {
                setLocale(code)
                setOpen(false)
              }}
              className="flex w-full items-center gap-1.5 px-2.5 py-1.5 text-left text-[12.5px] transition-colors hover:bg-secondary"
            >
              <Check className={cn('size-3.5 text-primary', locale === code ? 'opacity-100' : 'opacity-0')} />
              {LOCALE_LABEL[code]}
            </button>
          ))}
        </div>
      )}
    </div>
  )
}
