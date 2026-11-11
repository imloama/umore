import { useEffect, useMemo, useRef, useState } from 'react'
import { Download, FileText } from 'lucide-react'
import { useI18n } from '@/i18n'
import { attachmentImageUrl, loadAttachmentImage } from '../services/api'

// 轻量 Markdown 渲染：与后端 model.RenderMarkdown 语法保持一致。
// 前端自行渲染是为了编辑时实时预览，无需往返请求。
//
// 附件是这里唯一的特例：图片照常 <img>，其它文件（**链接** 形式、地址是 /api/attachments/<id>）
// 渲染成一张附件卡片 —— 源码里就是普通 Markdown，用户看到的却是「文件」，点开即原文件。
// 图片要带 token 才拉得到，所以带鉴权的图片在 DOM 里挂 data-auth-src，渲染后换成 blob。

const escapeHtml = (s: string) => s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;')

const safeUrl = (u: string) => /^(https?:\/\/|\/|mailto:)/i.test(u.trim())

/** 附件地址形如 /api/attachments/12，返回 id；不是附件返回 null */
export const attachmentIdOf = (url?: string) => {
  if (!url) return null
  const m = url.trim().match(/^\/api\/attachments\/(\d+)$/)
  return m ? Number(m[1]) : null
}

/**
 * 老版本的上传地址：/uploads/2026/11/1762xxx.png。
 * 那时候是公开静态目录，正文里写死了这种地址；现在文件已经搬到按人隔离的目录、
 * 读取要鉴权，但地址还留在历史文本里。后端留了一条兼容路由（按人反查后鉴权返回），
 * 这里把它也当成附件处理：图片走鉴权拉取，其它文件落成附件卡片。
 */
const LEGACY_PATH = /^\/uploads\/\d{4}\/\d{2}\/[A-Za-z0-9._-]+$/
const isLegacyUpload = (url?: string) => !!url && LEGACY_PATH.test(url.trim())

/** 是不是要走鉴权才能拿到的资源（新老附件地址都算） */
const needsAuth = (url: string) => !!attachmentIdOf(url) || isLegacyUpload(url)

const FILE_MARK = '\u0000FILE'

const inline = (text: string) =>
  text
    .replace(/`([^`\n]+)`/g, '<code>$1</code>')
    .replace(/!\[([^\]]*)\]\(([^)\s]+)\)/g, (_m, alt: string, url: string) => {
      if (!safeUrl(url)) return alt
      // 附件图片请求要带 token，<img> 带不了 —— 先留个带地址的占位，渲染后异步换成 blob
      if (needsAuth(url)) return `<img data-auth-src="${url}" alt="${alt}" />`
      return `<img src="${url}" alt="${alt}" />`
    })
    .replace(/\[([^\]]+)\]\(([^)\s]+)\)/g, (_m, label: string, url: string) => {
      if (!safeUrl(url)) return label
      // 附件链接先留个占位，块级拼完再换成卡片组件（老地址 /uploads/... 一样处理）
      if (needsAuth(url)) return `${FILE_MARK}${url}\u0000${label}\u0000${FILE_MARK}`
      return `<a href="${url}" target="_blank" rel="noopener noreferrer">${label}</a>`
    })
    .replace(/\*\*([^*]+)\*\*/g, '<strong>$1</strong>')
    .replace(/(^|[^*])\*([^*\n]+)\*/g, '$1<em>$2</em>')
    .replace(/~~([^~]+)~~/g, '<del>$1</del>')

export function renderMarkdown(src: string): string {
  if (!src.trim()) return ''

  const codes: string[] = []
  const withoutCode = src.replace(/```([a-zA-Z0-9+#-]*)\n?([\s\S]*?)```/g, (_m, lang: string, body: string) => {
    const cls = lang ? ` class="lang-${escapeHtml(lang)}"` : ''
    codes.push(`<pre><code${cls}>${escapeHtml(body.replace(/\n$/, ''))}</code></pre>`)
    return `\u0000CODE${codes.length - 1}\u0000`
  })

  const out: string[] = []
  let listType: '' | 'ul' | 'ol' = ''
  const closeList = () => {
    if (listType) {
      out.push(`</${listType}>`)
      listType = ''
    }
  }

  for (const rawLine of escapeHtml(withoutCode).split('\n')) {
    const line = rawLine.trim()
    const heading = line.match(/^(#{1,4})\s+(.*)$/)
    const quote = line.match(/^(&gt;|>)\s?(.*)$/)
    const task = line.match(/^[-*]\s+\[([ xX])\]\s+(.*)$/)
    const ul = line.match(/^[-*]\s+(.*)$/)
    const ol = line.match(/^\d+\.\s+(.*)$/)

    if (!line) {
      closeList()
      continue
    }
    if (heading) {
      closeList()
      const level = heading[1].length
      out.push(`<h${level}>${inline(heading[2])}</h${level}>`)
    } else if (quote) {
      closeList()
      out.push(`<blockquote>${inline(quote[2])}</blockquote>`)
    } else if (task) {
      // 与后端 model.RenderMarkdown 对齐：- [ ] / - [x] 渲染成清单
      if (listType !== 'ul') {
        closeList()
        listType = 'ul'
        out.push('<ul class="task-list">')
      }
      const done = task[1].toLowerCase() === 'x'
      out.push(
        `<li class="${done ? 'task-item is-done' : 'task-item'}">${done ? '&#9745;' : '&#9744;'} ${inline(task[2])}</li>`
      )
    } else if (ul) {
      if (listType !== 'ul') {
        closeList()
        listType = 'ul'
        out.push('<ul>')
      }
      out.push(`<li>${inline(ul[1])}</li>`)
    } else if (ol) {
      if (listType !== 'ol') {
        closeList()
        listType = 'ol'
        out.push('<ol>')
      }
      out.push(`<li>${inline(ol[1])}</li>`)
    } else {
      closeList()
      out.push(`<p>${inline(rawLine)}</p>`)
    }
  }
  closeList()

  return out.join('\n').replace(/\u0000CODE(\d+)\u0000/g, (_m, i: string) => codes[Number(i)] ?? '')
}

/** 只读富文本展示 */
export default function RichText({ source, className = '' }: { source: string; className?: string }) {
  const { t } = useI18n()
  const html = renderMarkdown(source)
  const container = useRef<HTMLDivElement>(null)
  const [, bumpImages] = useState(0)

  // 带鉴权的图片：用 axios 拉 blob 后填回 src。缓存命中的同步填，避免闪一下空白。
  const authImages = useMemo(() => {
    const found: string[] = []
    const pattern = /data-auth-src="([^"]+)"/g
    let m: RegExpExecArray | null
    while ((m = pattern.exec(html))) found.push(m[1])
    return found
  }, [html])

  useEffect(() => {
    const host = container.current
    if (!host || authImages.length === 0) return
    let alive = true
    const fill = (url: string, objectUrl: string) => {
      host.querySelectorAll(`img[data-auth-src="${CSS.escape(url)}"]`).forEach(img => {
        ;(img as HTMLImageElement).src = objectUrl
        img.removeAttribute('data-auth-src')
      })
    }
    authImages.forEach(url => {
      const cached = attachmentImageUrl(url)
      if (cached) {
        fill(url, cached)
        return
      }
      loadAttachmentImage(url)
        .then(objectUrl => {
          if (!alive) return
          fill(url, objectUrl)
          bumpImages(n => n + 1)
        })
        .catch(() => {
          /* 拉不到就保持空图：说明文件没了或不是本人，界面不额外报错 */
        })
    })
    return () => {
      alive = false
    }
  }, [authImages, html])

  // 附件卡片在渲染后按占位切片插入：用真实组件而不是 innerHTML，
  // 这样图标、下载链接、无障碍标签都能跟着语言走。
  const parts: { html?: string; url?: string; label?: string }[] = []
  const pattern = new RegExp(`${FILE_MARK}([^\\u0000]*)\\u0000([^\\u0000]*)\\u0000${FILE_MARK}`, 'g')
  let last = 0
  let match: RegExpExecArray | null
  while ((match = pattern.exec(html))) {
    if (match.index > last) parts.push({ html: html.slice(last, match.index) })
    parts.push({ url: match[1], label: match[2] })
    last = match.index + match[0].length
  }
  if (last < html.length) parts.push({ html: html.slice(last) })

  if (parts.length === 1 && parts[0].html !== undefined) {
    return <div ref={container} className={`rich-text ${className}`} dangerouslySetInnerHTML={{ __html: html }} />
  }

  return (
    <div ref={container} className={`rich-text ${className}`}>
      {parts.map((part, index) =>
        part.html !== undefined ? (
          <span key={index} dangerouslySetInnerHTML={{ __html: part.html }} />
        ) : (
          <a
            key={index}
            href={part.url}
            target="_blank"
            rel="noopener noreferrer"
            className="my-1.5 flex items-center gap-2.5 rounded-lg border border-border bg-secondary/40 px-3 py-2 no-underline transition-colors hover:border-primary/40 hover:bg-accent/50"
          >
            <span className="flex size-7 shrink-0 items-center justify-center rounded-md bg-card text-muted-foreground shadow-sm">
              <FileText className="size-3.5" />
            </span>
            <span className="min-w-0 flex-1">
              <span className="block truncate text-[12.5px] font-medium text-foreground">{part.label}</span>
              <span className="block text-[10.5px] text-muted-foreground">{t('editor.attachment')}</span>
            </span>
            <Download className="size-3.5 shrink-0 text-muted-foreground" />
          </a>
        )
      )}
    </div>
  )
}
