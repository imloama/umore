// 轻量 Markdown 渲染：与后端 model.RenderMarkdown 语法保持一致。
// 前端自行渲染是为了编辑时实时预览，无需往返请求。

const escapeHtml = (s: string) =>
  s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;')

const safeUrl = (u: string) => /^(https?:\/\/|\/|mailto:)/i.test(u.trim())

const inline = (text: string) =>
  text
    .replace(/`([^`\n]+)`/g, '<code>$1</code>')
    .replace(/!\[([^\]]*)\]\(([^)\s]+)\)/g, (_m, alt: string, url: string) =>
      safeUrl(url) ? `<img src="${url}" alt="${alt}" />` : alt
    )
    .replace(/\[([^\]]+)\]\(([^)\s]+)\)/g, (_m, label: string, url: string) =>
      safeUrl(url)
        ? `<a href="${url}" target="_blank" rel="noopener noreferrer">${label}</a>`
        : label
    )
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
  return <div className={`rich-text ${className}`} dangerouslySetInnerHTML={{ __html: renderMarkdown(source) }} />
}
