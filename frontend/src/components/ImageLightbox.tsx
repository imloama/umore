import { useCallback, useEffect, useRef, useState, type TouchEvent } from 'react'
import { createPortal } from 'react-dom'
import { ChevronLeft, ChevronRight, X } from 'lucide-react'
import { useI18n } from '@/i18n'
import { cn } from '@/lib/utils'

/**
 * 图片查看器：点富文本里的图 → 全屏放大；同一作用域有多张就能左右翻。
 *
 * 为什么是「模块级转交 + 全局挂一次」而不是 Context：
 * 看图这件事从任意深度的 RichText 里发起，但查看器永远只有一个，
 * 挂在 main.tsx 里最省事，也不用在每个页面套 Provider。
 *
 * 作用域怎么定：RichText 点图时找最近的 [data-image-gallery] 祖先，
 * 把里面的 <img> 全收进来 —— 任务详情把正文和评论都算一个作用域，
 * 所以在一张单子里点开任何一张图，都能把这张单子的图从头翻到尾。
 */

export interface LightboxImage {
  src: string
  alt?: string
}

type Viewer = (images: LightboxImage[], index: number) => void

let viewer: Viewer | null = null

/** 打开查看器；images 是同一作用域里的全部图片，index 是被点中的那张 */
export const openImageLightbox = (images: LightboxImage[], index = 0) => {
  const usable = images.filter(item => !!item.src)
  if (!viewer || usable.length === 0) return
  viewer(usable, Math.min(Math.max(index, 0), usable.length - 1))
}

interface State {
  images: LightboxImage[]
  index: number
}

export default function ImageLightbox() {
  const { t } = useI18n()
  const [state, setState] = useState<State | null>(null)
  const closeButton = useRef<HTMLButtonElement>(null)
  const touchStart = useRef<number | null>(null)
  const open = !!state

  const step = useCallback((delta: number) => {
    setState(prev => {
      if (!prev) return prev
      const count = prev.images.length
      // 首尾相连：看到最后一张再按右就回到第一张
      return { ...prev, index: (prev.index + delta + count) % count }
    })
  }, [])

  useEffect(() => {
    viewer = (images, index) => setState({ images, index })
    return () => {
      viewer = null
    }
  }, [])

  // Esc / 左右键。捕获阶段拦下：查看器盖在详情弹窗上面，
  // 按 Esc 只该关图片，不该把底下的任务详情一起关掉。
  useEffect(() => {
    if (!open) return
    const onKey = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        event.stopPropagation()
        setState(null)
      } else if (event.key === 'ArrowLeft') {
        event.preventDefault()
        event.stopPropagation()
        step(-1)
      } else if (event.key === 'ArrowRight') {
        event.preventDefault()
        event.stopPropagation()
        step(1)
      }
    }
    document.addEventListener('keydown', onKey, true)
    return () => document.removeEventListener('keydown', onKey, true)
  }, [open, step])

  // 打开时锁滚动，关掉还原（详情弹窗自己那层锁不受影响）
  useEffect(() => {
    if (!open) return
    const prev = document.body.style.overflow
    document.body.style.overflow = 'hidden'
    closeButton.current?.focus()
    return () => {
      document.body.style.overflow = prev
    }
  }, [open])

  if (!state || typeof document === 'undefined') return null

  const current = state.images[state.index]
  const many = state.images.length > 1

  const onTouchStart = (event: TouchEvent<HTMLDivElement>) => {
    touchStart.current = event.touches[0]?.clientX ?? null
  }

  const onTouchEnd = (event: TouchEvent<HTMLDivElement>) => {
    const start = touchStart.current
    touchStart.current = null
    if (start === null || !many) return
    const end = event.changedTouches[0]?.clientX ?? start
    const delta = end - start
    // 移动端够一根手指的滑动距离才翻页，避免误触
    if (Math.abs(delta) > 48) step(delta > 0 ? -1 : 1)
  }

  return createPortal(
    <div
      className="fixed inset-0 z-[130] bg-slate-950/90 backdrop-blur-sm"
      role="dialog"
      aria-modal="true"
      aria-label={t('common.viewImage')}
    >
      <div className="absolute inset-0" onClick={() => setState(null)} aria-hidden />

      <div className="relative flex h-full flex-col" onTouchStart={onTouchStart} onTouchEnd={onTouchEnd}>
        <div className="flex items-center justify-between gap-3 px-4 py-3 text-slate-200">
          {many ? (
            <span className="tabular text-[12px] tabular-nums">
              {state.index + 1} / {state.images.length}
            </span>
          ) : (
            <span />
          )}
          <button
            ref={closeButton}
            type="button"
            onClick={() => setState(null)}
            aria-label={t('common.close')}
            className="rounded-md p-1.5 text-slate-300 transition-colors hover:bg-white/10 hover:text-white focus-visible:outline-2 focus-visible:outline-white/60"
          >
            <X className="size-4.5" />
          </button>
        </div>

        <div
          className="relative flex min-h-0 flex-1 items-center justify-center px-3 sm:px-16"
          onClick={event => {
            // 点图片以外的空白也关掉：全屏看图时这比找那个小叉子顺手
            if (event.target === event.currentTarget) setState(null)
          }}
        >
          {many && (
            <button
              type="button"
              onClick={() => step(-1)}
              aria-label={t('common.prevImage')}
              className="absolute left-2 z-10 flex size-9 items-center justify-center rounded-full bg-white/10 text-white backdrop-blur transition-colors hover:bg-white/20 sm:left-4 sm:size-11"
            >
              <ChevronLeft className="size-5" />
            </button>
          )}

          <img
            key={current.src}
            src={current.src}
            alt={current.alt || ''}
            draggable={false}
            className="max-h-full max-w-full select-none rounded-lg object-contain shadow-2xl"
          />

          {many && (
            <button
              type="button"
              onClick={() => step(1)}
              aria-label={t('common.nextImage')}
              className="absolute right-2 z-10 flex size-9 items-center justify-center rounded-full bg-white/10 text-white backdrop-blur transition-colors hover:bg-white/20 sm:right-4 sm:size-11"
            >
              <ChevronRight className="size-5" />
            </button>
          )}
        </div>

        <div className="px-4 pb-3 pt-1">
          {many && (
            <div className="scrollbar-thin flex justify-start gap-2 overflow-x-auto pb-1 sm:justify-center">
              {state.images.map((image, index) => (
                <button
                  key={`${image.src}-${index}`}
                  type="button"
                  onClick={() => setState(prev => (prev ? { ...prev, index } : prev))}
                  aria-label={t('common.viewImage')}
                  aria-current={index === state.index}
                  className={cn(
                    'size-12 shrink-0 overflow-hidden rounded-md border-2 transition-opacity',
                    index === state.index
                      ? 'border-white opacity-100'
                      : 'border-transparent opacity-55 hover:opacity-90'
                  )}
                >
                  <img src={image.src} alt="" draggable={false} className="size-full object-cover" />
                </button>
              ))}
            </div>
          )}
          {many && (
            <p className="mt-1 hidden text-center text-[11px] text-slate-400 sm:block">{t('common.imageViewerHint')}</p>
          )}
        </div>
      </div>
    </div>,
    document.body
  )
}
