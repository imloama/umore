import * as React from 'react'
import { createPortal } from 'react-dom'
import { X } from 'lucide-react'
import { useI18n } from '@/i18n'
import { cn } from '@/lib/utils'

/**
 * 轻量弹窗：遮罩点击关闭 + Esc 关闭，内容区自己滚动。
 *
 * 一定要 portal 到 document.body：以前直接渲染在调用点，被页面里带 transform /
 * z-index 的容器一夹，弹窗就钻到侧边栏和内容区底下了。挂到 body 上再配 z-[100]，
 * 遮罩才是真的「盖住全部」——侧边栏（z-40）、顶栏（z-20）都在这层下面。
 */
export function Dialog({
  open,
  onClose,
  title,
  description,
  className,
  children,
  footer
}: {
  open: boolean
  onClose: () => void
  title?: React.ReactNode
  description?: React.ReactNode
  className?: string
  children: React.ReactNode
  footer?: React.ReactNode
}) {
  const { t } = useI18n()

  React.useEffect(() => {
    if (!open) return
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose()
    }
    document.addEventListener('keydown', onKey)
    const prev = document.body.style.overflow
    document.body.style.overflow = 'hidden'
    return () => {
      document.removeEventListener('keydown', onKey)
      document.body.style.overflow = prev
    }
  }, [open, onClose])

  if (!open || typeof document === 'undefined') return null

  return createPortal(
    <div className="fixed inset-0 z-[100] flex items-start justify-center overflow-y-auto bg-slate-950/45 p-4 backdrop-blur-[2px] sm:p-8">
      <div className="absolute inset-0" onClick={onClose} aria-hidden />
      <div
        role="dialog"
        aria-modal="true"
        className={cn(
          'relative my-auto w-full max-w-2xl rounded-xl border border-border bg-card shadow-2xl',
          className
        )}
      >
        {(title || description) && (
          <div className="flex items-start justify-between gap-4 border-b border-border px-5 py-4">
            <div className="min-w-0">
              {title && <div className="text-base font-semibold tracking-tight">{title}</div>}
              {description && <div className="mt-0.5 text-xs text-muted-foreground">{description}</div>}
            </div>
            <button
              type="button"
              onClick={onClose}
              className="rounded-md p-1 text-muted-foreground transition-colors hover:bg-accent hover:text-accent-foreground"
              aria-label={t('common.close')}
            >
              <X className="size-4" />
            </button>
          </div>
        )}
        <div className="scrollbar-thin max-h-[calc(100vh-13rem)] overflow-y-auto px-5 py-4">{children}</div>
        {footer && <div className="flex justify-end gap-2 border-t border-border px-5 py-3.5">{footer}</div>}
      </div>
    </div>,
    document.body
  )
}
