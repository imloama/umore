import { useRef, useState } from 'react'
import {
  Bold,
  Code,
  Eye,
  Heading,
  Italic,
  Link as LinkIcon,
  List,
  ListOrdered,
  Loader,
  Paperclip,
  Pencil,
  Quote,
  SquareCode,
  Strikethrough
} from 'lucide-react'
import RichText from './RichText'
import { uploadApi } from '../services/api'
import { useI18n } from '@/i18n'
import { Button } from '@/components/ui/button'
import { cn } from '@/lib/utils'

export interface QuickInsert {
  label: string
  text: string
}

interface Props {
  value: string
  onChange: (next: string) => void
  onSubmit?: () => void
  placeholder?: string
  quickInserts?: QuickInsert[]
  submitLabel?: string
  minRows?: number
  /** 图片上传实现，默认走后端 /api/attachments；返回可访问的图片地址 */
  uploadImage?: (file: File) => Promise<string>
  /** 附件上传实现，默认同上；返回可访问地址与原始文件名 */
  uploadFile?: (file: File) => Promise<{ url: string; name: string }>
}

type ToolKey =
  | 'bold'
  | 'italic'
  | 'strike'
  | 'code'
  | 'codeBlock'
  | 'heading'
  | 'quote'
  | 'ul'
  | 'ol'
  | 'link'

const TOOLS: {
  key: ToolKey
  icon: typeof Bold
  wrap?: [string, string]
  prefix?: string
  block?: string
}[] = [
  { key: 'bold', icon: Bold, wrap: ['**', '**'] },
  { key: 'italic', icon: Italic, wrap: ['*', '*'] },
  { key: 'strike', icon: Strikethrough, wrap: ['~~', '~~'] },
  { key: 'code', icon: Code, wrap: ['`', '`'] },
  { key: 'codeBlock', icon: SquareCode, block: '```\n\n```' },
  { key: 'heading', icon: Heading, prefix: '## ' },
  { key: 'quote', icon: Quote, prefix: '> ' },
  { key: 'ul', icon: List, prefix: '- ' },
  { key: 'ol', icon: ListOrdered, prefix: '1. ' },
  { key: 'link', icon: LinkIcon, wrap: ['[', '](https://)'] }
]

const IMAGE_LIMIT = 10 * 1024 * 1024
const FILE_LIMIT = 20 * 1024 * 1024

export default function RichTextEditor({
  value,
  onChange,
  onSubmit,
  placeholder = '',
  quickInserts = [],
  submitLabel,
  minRows = 4,
  uploadImage,
  uploadFile
}: Props) {
  const { t } = useI18n()
  const [tab, setTab] = useState<'write' | 'preview'>('write')
  const [uploading, setUploading] = useState(0)
  const [error, setError] = useState('')
  const ref = useRef<HTMLTextAreaElement>(null)
  const fileInput = useRef<HTMLInputElement>(null)
  // 连续粘贴多张图时不能依赖 props.value 的闭包值，否则后一张会覆盖前一张
  const valueRef = useRef(value)
  valueRef.current = value

  const insertAtCursor = (snippet: string) => {
    const el = ref.current
    const current = valueRef.current
    const start = el ? el.selectionStart : current.length
    const end = el ? el.selectionEnd : current.length
    const next = current.slice(0, start) + snippet + current.slice(end)
    valueRef.current = next
    onChange(next)
    requestAnimationFrame(() => {
      if (!el) return
      el.focus()
      const cursor = start + snippet.length
      el.setSelectionRange(cursor, cursor)
    })
  }

  const doUploadImage = async (file: File) => {
    const uploader = uploadImage || (async (f: File) => (await uploadApi.image(f)).url)
    return uploader(file)
  }

  const doUploadFile = async (file: File) => {
    if (uploadFile) return uploadFile(file)
    const res = await uploadApi.file(file)
    return { url: res.url, name: res.original_name || file.name }
  }

  // 粘贴/拖入图片 -> 上传 -> 插入 Markdown 图片（渲染后就是图，不是源码）
  const uploadFiles = async (files: File[]) => {
    const images = files.filter(f => f.type.startsWith('image/'))
    if (images.length === 0) return
    setError('')
    for (const file of images) {
      if (file.size > IMAGE_LIMIT) {
        setError(t('editor.imageTooLarge'))
        continue
      }
      setUploading(n => n + 1)
      try {
        const url = await doUploadImage(file)
        const name = file.name && file.name !== 'image.png' ? file.name : 'image'
        insertAtCursor(`![${name}](${url})\n`)
      } catch (err) {
        console.error('Failed to upload image:', err)
        setError(t('editor.imageUploadFailed'))
      } finally {
        setUploading(n => n - 1)
      }
    }
  }

  // 手动选文件：图片走内联渲染，其它文件落成 Markdown 链接，
  // 渲染后由 RichText 换成附件卡片 —— 用户看到的是「文件」，不是一串地址。
  const uploadAttachments = async (fileList?: FileList | null) => {
    const files = Array.from(fileList || [])
    if (files.length === 0) return
    setError('')
    for (const file of files) {
      if (file.type.startsWith('image/')) {
        await uploadFiles([file])
        continue
      }
      if (file.size > FILE_LIMIT) {
        setError(t('editor.fileTooLarge'))
        continue
      }
      setUploading(n => n + 1)
      try {
        const uploaded = await doUploadFile(file)
        insertAtCursor(`[${uploaded.name}](${uploaded.url})\n`)
      } catch (err) {
        console.error('Failed to upload file:', err)
        setError(t('editor.fileUploadFailed'))
      } finally {
        setUploading(n => n - 1)
      }
    }
    if (fileInput.current) fileInput.current.value = ''
  }

  const handlePaste = (e: React.ClipboardEvent<HTMLTextAreaElement>) => {
    const files = Array.from(e.clipboardData?.files || [])
    if (files.some(f => f.type.startsWith('image/'))) {
      e.preventDefault()
      uploadFiles(files)
    }
  }

  const handleDrop = (e: React.DragEvent<HTMLTextAreaElement>) => {
    const files = Array.from(e.dataTransfer?.files || [])
    if (files.length > 0) {
      e.preventDefault()
      uploadAttachments(files as unknown as FileList)
    }
  }

  const apply = (tool: (typeof TOOLS)[number]) => {
    const el = ref.current
    if (!el) return
    const start = el.selectionStart
    const end = el.selectionEnd
    const selected = value.slice(start, end)

    let next = value
    let cursor = end

    if (tool.wrap) {
      const [open, close] = tool.wrap
      next = value.slice(0, start) + open + selected + close + value.slice(end)
      cursor = start + open.length + selected.length
    } else if (tool.prefix) {
      const lineStart = value.lastIndexOf('\n', start - 1) + 1
      next = value.slice(0, lineStart) + tool.prefix + value.slice(lineStart)
      cursor = end + tool.prefix.length
    } else if (tool.block) {
      const body = tool.block.replace('\n\n', `\n${selected}\n`)
      next = value.slice(0, start) + body + value.slice(end)
      cursor = start + body.length
    }

    onChange(next)
    requestAnimationFrame(() => {
      el.focus()
      el.setSelectionRange(cursor, cursor)
    })
  }

  const handleKeyDown = (e: React.KeyboardEvent<HTMLTextAreaElement>) => {
    if ((e.metaKey || e.ctrlKey) && e.key === 'Enter' && onSubmit) {
      e.preventDefault()
      onSubmit()
      return
    }
    if (e.key === 'Tab') {
      e.preventDefault()
      const el = e.currentTarget
      const start = el.selectionStart
      onChange(value.slice(0, start) + '  ' + value.slice(el.selectionEnd))
      requestAnimationFrame(() => el.setSelectionRange(start + 2, start + 2))
    }
  }

  return (
    <div className="overflow-hidden rounded-lg border border-input bg-card shadow-sm">
      <div className="flex flex-wrap items-center gap-0.5 border-b border-border bg-secondary/50 px-1.5 py-1">
        {TOOLS.map(tool => {
          const Icon = tool.icon
          return (
            <button
              key={tool.key}
              type="button"
              title={t(`editor.${tool.key}`)}
              onClick={() => apply(tool)}
              className="rounded p-1.5 text-muted-foreground transition-colors hover:bg-card hover:text-primary"
            >
              <Icon className="size-3.5" />
            </button>
          )
        })}
        <span className="mx-1 h-4 w-px bg-border" />
        <button
          type="button"
          title={t('editor.attach')}
          onClick={() => fileInput.current?.click()}
          className="rounded p-1.5 text-muted-foreground transition-colors hover:bg-card hover:text-primary"
        >
          <Paperclip className="size-3.5" />
        </button>
        <input
          ref={fileInput}
          type="file"
          multiple
          className="hidden"
          onChange={e => uploadAttachments(e.target.files)}
        />
        <span className="mx-1 h-4 w-px bg-border" />
        <div className="flex rounded-md bg-card/70 p-0.5">
          {(['write', 'preview'] as const).map(name => (
            <button
              key={name}
              type="button"
              onClick={() => setTab(name)}
              className={cn(
                'flex items-center gap-1 rounded px-2 py-1 text-[11px] transition-colors',
                tab === name ? 'bg-card font-medium text-primary shadow-sm' : 'text-muted-foreground hover:text-foreground'
              )}
            >
              {name === 'write' ? <Pencil className="size-3" /> : <Eye className="size-3" />}
              {name === 'write' ? t('editor.write') : t('editor.preview')}
            </button>
          ))}
        </div>
        <span className="ml-auto pr-1 text-[10px] text-muted-foreground/70">{t('editor.markdown')}</span>
      </div>

      {tab === 'write' ? (
        <textarea
          ref={ref}
          rows={minRows}
          value={value}
          placeholder={placeholder}
          onChange={e => onChange(e.target.value)}
          onKeyDown={handleKeyDown}
          onPaste={handlePaste}
          onDrop={handleDrop}
          onDragOver={e => {
            if (Array.from(e.dataTransfer?.items || []).some(i => i.kind === 'file')) e.preventDefault()
          }}
          className="w-full resize-y border-0 bg-transparent px-3 py-2.5 text-[13.5px] leading-relaxed outline-none placeholder:text-muted-foreground/70"
        />
      ) : (
        <div className="min-h-[110px] px-3 py-2.5">
          {value.trim() ? <RichText source={value} /> : <span className="text-xs text-muted-foreground">{t('editor.noContent')}</span>}
        </div>
      )}

      <div className="flex flex-wrap items-center gap-2 border-t border-border bg-secondary/30 px-2.5 py-1.5">
        <span className="flex items-center gap-1 text-[11px] text-muted-foreground">
          {uploading > 0 ? (
            <>
              <Loader className="size-3 animate-spin" /> {t('editor.uploading')}
            </>
          ) : error ? (
            <span className="text-red-600">{error}</span>
          ) : (
            t('editor.hint')
          )}
        </span>
        <div className="ml-auto flex items-center gap-1.5">
          {quickInserts.map(item => (
            <Button
              key={item.label}
              type="button"
              variant="ghost"
              size="xs"
              onClick={() => onChange(value ? `${value}\n${item.text}` : item.text)}
            >
              {item.label}
            </Button>
          ))}
          {submitLabel && onSubmit && (
            <Button type="button" size="sm" disabled={!value.trim()} onClick={onSubmit}>
              {submitLabel}
            </Button>
          )}
        </div>
      </div>
    </div>
  )
}
