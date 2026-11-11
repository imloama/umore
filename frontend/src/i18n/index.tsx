import { createContext, useCallback, useContext, useEffect, useMemo, useState, type ReactNode } from 'react'
import { resources, type Locale, type MessageKey } from './locales'
import { translateError } from './errors'

/**
 * 界面语言：决定「这次打开用哪种语言」的顺序是
 *   1. 用户上次手动选过的（localStorage）
 *   2. 浏览器语言（navigator.languages，zh 开头算中文）
 *   3. 都没有就用英文
 *
 * 语言只影响前端文案：后端只回错误码，数据库里也不存多语内容。
 */

const STORAGE_KEY = 'umore_locale'
export const SUPPORTED_LOCALES: Locale[] = ['zh', 'en']

export const LOCALE_LABEL: Record<Locale, string> = {
  zh: '中文',
  en: 'English'
}

/** 浏览器语言 → 支持的语言码；判不出来给 null，交给调用方兜底 */
const detectLocale = (): Locale | null => {
  if (typeof navigator === 'undefined') return null
  const candidates = [...(navigator.languages || []), navigator.language].filter(Boolean)
  for (const raw of candidates) {
    const tag = raw.toLowerCase()
    if (tag.startsWith('zh')) return 'zh'
    if (tag.startsWith('en')) return 'en'
  }
  return null
}

const readStoredLocale = (): Locale | null => {
  try {
    const saved = localStorage.getItem(STORAGE_KEY)
    if (saved && (SUPPORTED_LOCALES as string[]).includes(saved)) return saved as Locale
  } catch {
    // 隐私模式下 localStorage 会抛错，当作没存过
  }
  return null
}

/** 用户上次的设置 > 浏览器语言 > 英文 */
export const resolveInitialLocale = (): Locale => readStoredLocale() || detectLocale() || 'en'

type Vars = Record<string, string | number>

export interface I18nValue {
  locale: Locale
  setLocale: (next: Locale) => void
  /** 取文案，{name} 占位按 vars 替换 */
  t: (key: MessageKey, vars?: Vars) => string
  /** 后端错误码 → 本地文案；认不出码时用后端兜底文案 */
  te: (code?: string, fallback?: string) => string
}

const I18nContext = createContext<I18nValue | null>(null)

const format = (template: string, vars?: Vars) => {
  if (!vars) return template
  return template.replace(/\{(\w+)\}/g, (match, name: string) => (name in vars ? String(vars[name]) : match))
}

export function I18nProvider({ children }: { children: ReactNode }) {
  const [locale, setLocaleState] = useState<Locale>(() => resolveInitialLocale())

  // html lang 跟着变：影响字体回退、无障碍读屏，也方便调试
  useEffect(() => {
    document.documentElement.lang = locale === 'zh' ? 'zh-CN' : 'en'
    document.title = locale === 'zh' ? 'umore · 独立开发工作台' : 'umore · Solo developer workspace'
  }, [locale])

  const setLocale = useCallback((next: Locale) => {
    // 记住这次的选择，下次打开直接用
    try {
      localStorage.setItem(STORAGE_KEY, next)
    } catch {
      // 存不了也不影响本次生效
    }
    setLocaleState(next)
  }, [])

  const value = useMemo<I18nValue>(() => {
    const table = resources[locale]
    return {
      locale,
      setLocale,
      t: (key, vars) => format(table[key] ?? resources.en[key] ?? key, vars),
      te: (code, fallback) => translateError(code, fallback, locale)
    }
  }, [locale, setLocale])

  return <I18nContext.Provider value={value}>{children}</I18nContext.Provider>
}

/** 组件里取文案：const { t, locale, setLocale } = useI18n() */
export function useI18n(): I18nValue {
  const ctx = useContext(I18nContext)
  if (!ctx) throw new Error('useI18n 必须在 <I18nProvider> 里用')
  return ctx
}

export type { Locale, MessageKey }
