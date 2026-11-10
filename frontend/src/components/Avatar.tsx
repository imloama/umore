import type { AuthUser } from '../types'

/** 头像：有图显示图，没图就用昵称首字顶一下 */
export default function Avatar({ user, size = 'size-9' }: { user?: AuthUser | null; size?: string }) {
  const initial = (user?.nickname || user?.email || 'u').slice(0, 1).toUpperCase()
  if (user?.avatar) {
    return <img src={user.avatar} alt="" className={`${size} shrink-0 rounded-full object-cover`} />
  }
  return (
    <span
      className={`${size} flex shrink-0 items-center justify-center rounded-full bg-accent text-xs font-semibold text-accent-foreground`}
    >
      {initial}
    </span>
  )
}
