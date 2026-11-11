import { cn } from '@/lib/utils'

/**
 * 品牌图标。
 * 唯一来源是 public/favicon.svg —— 浏览器标签页、侧边栏、登录页共用同一个文件，
 * 以后换图标只改那一个 svg，这里不用动。
 */
export default function Logo({ className }: { className?: string }) {
  return (
    <img
      src="/favicon.svg"
      alt="umore"
      width={32}
      height={32}
      className={cn('size-8 shrink-0 rounded-lg', className)}
    />
  )
}
