// 把 vite build 产物内联成单个 html，双击即可离线看界面骨架
// 用法: npm run build:preview  （产物写到 ../preview/umore-preview.html）
import { readFileSync, writeFileSync, mkdirSync } from 'node:fs'
import { dirname, join, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

const here = dirname(fileURLToPath(import.meta.url))
const dist = resolve(here, '..', 'dist')
const outFile = resolve(here, '..', '..', 'preview', 'umore-preview.html')

let html = readFileSync(join(dist, 'index.html'), 'utf8')

// 内联 JS / CSS，去掉 crossorigin 之类的加载属性
html = html.replace(/<script type="module"[^>]*src="([^"]+)"[^>]*><\/script>/g, (_m, src) => {
  const code = readFileSync(join(dist, src.replace(/^\//, '')), 'utf8')
  return `<script type="module">\n${code}\n</script>`
})
html = html.replace(/<link rel="stylesheet"[^>]*href="([^"]+)"[^>]*>/g, (_m, href) => {
  const css = readFileSync(join(dist, href.replace(/^\//, '')), 'utf8')
  return `<style>\n${css}\n</style>`
})
// 离线没有图标，去掉引用避免 404
html = html.replace(/\s*<link rel="icon"[^>]*>/g, '')

mkdirSync(dirname(outFile), { recursive: true })
writeFileSync(outFile, html)
console.log(`preview written: ${outFile} (${(html.length / 1024).toFixed(0)} KB)`)
