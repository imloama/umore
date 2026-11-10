import { useEffect, useState } from 'react'
import { Link, Outlet, useLocation, useNavigate } from 'react-router-dom'
import {
  Briefcase,
  CalendarDays,
  ChevronRight,
  FolderKanban,
  HandCoins,
  LayoutDashboard,
  Lightbulb,
  ListTodo,
  LogOut,
  Menu,
  Search,
  Sun,
  UserRound,
  Wallet,
  X
} from 'lucide-react'
import { currentUser, subscribeUser } from '../services/api'
import type { AuthUser } from '../types'
import Avatar from './Avatar'
import QuickCreate from './QuickCreate'
import { Button } from '@/components/ui/button'
import { cn } from '@/lib/utils'

interface NavItem {
  path: string
  label: string
  icon: typeof LayoutDashboard
}

// 分组按「一天怎么用」排，不按模块类型排：
// 工作台是每天进门先看的地方，项目是接单主线（商机赢单才变项目，不能拆两处），
// 收支是钱的两头，设置放不常点但必须找得到的东西。
const NAV_GROUPS: { title: string; items: NavItem[] }[] = [
  {
    title: '工作台',
    items: [
      { path: '/', label: '仪表盘', icon: LayoutDashboard },
      { path: '/today', label: '今日', icon: Sun },
      { path: '/tasks', label: '任务', icon: ListTodo },
      { path: '/calendar', label: '日历', icon: CalendarDays },
      { path: '/ideas', label: '灵感', icon: Lightbulb }
    ]
  },
  {
    title: '项目',
    items: [
      { path: '/opportunities', label: '商机', icon: Briefcase },
      { path: '/projects', label: '项目', icon: FolderKanban }
    ]
  },
  {
    title: '收支',
    items: [
      { path: '/receivables', label: '回款', icon: HandCoins },
      { path: '/finances', label: '财务', icon: Wallet }
    ]
  },
  {
    title: '设置',
    items: [{ path: '/profile', label: '个人资料', icon: UserRound }]
  }
]

const ALL_ITEMS = NAV_GROUPS.flatMap(g => g.items)

export default function Layout({ onLogout }: { onLogout: () => void }) {
  const location = useLocation()
  const navigate = useNavigate()
  const [menuOpen, setMenuOpen] = useState(false)
  const [keyword, setKeyword] = useState('')
  // 底部显示当前用户的昵称，改完资料会跟着变
  const [user, setUser] = useState<AuthUser | null>(() => currentUser())
  useEffect(() => subscribeUser(setUser), [])

  // 路由变化时收起移动端侧边栏
  useEffect(() => setMenuOpen(false), [location.pathname])

  // 面包屑用「分组 / 页面」两级，找不到时兜个底，免得顶栏空着
  const currentGroup = NAV_GROUPS.find(group => group.items.some(item => item.path === location.pathname))
  const current = ALL_ITEMS.find(item => item.path === location.pathname)
  const crumbGroup = currentGroup && currentGroup.title !== current?.label ? currentGroup.title : null

  const handleLogout = () => {
    // 退出登录的活交给 App：先通知服务端作废会话，再清本地并置空登录态。
    // 这里只负责跳页，免得两处各清一半。
    onLogout()
    navigate('/login')
  }

  const search = () => {
    const kw = keyword.trim()
    navigate(kw ? `/tasks?keyword=${encodeURIComponent(kw)}` : '/tasks')
  }

  return (
    <div className="min-h-screen bg-background">
      {/* 侧边栏 */}
      <aside
        className={cn(
          'fixed inset-y-0 left-0 z-40 w-60 border-r border-border bg-sidebar transition-transform md:translate-x-0',
          menuOpen ? 'translate-x-0' : '-translate-x-full'
        )}
      >
        <div className="flex h-14 items-center justify-between border-b border-border px-4">
          <Link to="/" className="flex items-center gap-2.5">
            <span className="flex size-8 items-center justify-center rounded-lg bg-primary text-sm font-bold text-primary-foreground shadow-sm">
              u
            </span>
            <span className="min-w-0">
              <span className="block text-[15px] font-semibold leading-tight tracking-tight">umore</span>
              <span className="block text-[10px] leading-tight text-muted-foreground">独立开发工作台</span>
            </span>
          </Link>
          <button
            type="button"
            className="rounded-md p-1 text-muted-foreground hover:bg-accent md:hidden"
            onClick={() => setMenuOpen(false)}
            aria-label="收起菜单"
          >
            <X className="size-4" />
          </button>
        </div>

        <nav className="scrollbar-thin flex h-[calc(100vh-3.5rem-3.75rem)] flex-col gap-4 overflow-y-auto px-3 py-4">
          {NAV_GROUPS.map(group => (
            <div key={group.title}>
              <div className="mb-1.5 px-2 text-[10px] font-semibold uppercase tracking-widest text-muted-foreground/70">
                {group.title}
              </div>
              <div className="flex flex-col gap-0.5">
                {group.items.map(item => {
                  const active = location.pathname === item.path
                  const Icon = item.icon
                  return (
                    <Link
                      key={item.path}
                      to={item.path}
                      className={cn(
                        'group flex items-center gap-2.5 rounded-md px-2.5 py-2 text-[13px] transition-colors',
                        active
                          ? 'bg-accent font-semibold text-accent-foreground'
                          : 'text-sidebar-foreground hover:bg-secondary hover:text-foreground'
                      )}
                    >
                      <Icon className={cn('size-4', active ? 'text-primary' : 'text-muted-foreground')} />
                      <span>{item.label}</span>
                    </Link>
                  )
                })}
              </div>
            </div>
          ))}
        </nav>

        <div className="absolute inset-x-0 bottom-0 flex items-center gap-2 border-t border-border px-3 py-2.5">
          <Link
            to="/profile"
            className="flex min-w-0 flex-1 items-center gap-2 rounded-md px-1 py-1 transition-colors hover:bg-accent"
            title="个人资料"
          >
            <Avatar user={user} size="size-8" />
            <div className="min-w-0 flex-1">
              <div className="truncate text-[13px] font-medium leading-tight">{user?.nickname || '未登录'}</div>
              <div className="truncate text-[10px] leading-tight text-muted-foreground">{user?.email || ''}</div>
            </div>
          </Link>
          <Button variant="ghost" size="icon-sm" onClick={handleLogout} title="退出登录">
            <LogOut />
          </Button>
        </div>
      </aside>

      {menuOpen && (
        <div className="fixed inset-0 z-30 bg-slate-950/30 md:hidden" onClick={() => setMenuOpen(false)} />
      )}

      {/* 主区域 */}
      <div className="md:pl-60">
        <header className="sticky top-0 z-20 flex h-14 items-center gap-3 border-b border-border bg-background/85 px-4 backdrop-blur-md sm:px-6">
          <button
            type="button"
            className="rounded-md p-1.5 text-muted-foreground hover:bg-accent md:hidden"
            onClick={() => setMenuOpen(true)}
            aria-label="打开菜单"
          >
            <Menu className="size-5" />
          </button>

          <div className="flex min-w-0 items-center gap-1.5 text-[13px] text-muted-foreground">
            {/* 分组名和页面名撞了（项目 > 项目）就只留一层，省得看着像 bug */}
            {crumbGroup && (
              <>
                <span className="hidden sm:inline">{crumbGroup}</span>
                <ChevronRight className="hidden size-3.5 sm:inline" />
              </>
            )}
            <span className="truncate font-medium text-foreground">{current?.label ?? '页面'}</span>
          </div>

          <div className="ml-auto flex items-center gap-2">
            <div className="relative hidden sm:block">
              <Search className="pointer-events-none absolute left-2.5 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
              <input
                value={keyword}
                onChange={e => setKeyword(e.target.value)}
                onKeyDown={e => e.key === 'Enter' && search()}
                placeholder="搜索任务…"
                className="h-8 w-52 rounded-md border border-input bg-card pl-8 pr-2.5 text-[13px] shadow-sm transition-colors placeholder:text-muted-foreground/70 focus-visible:border-ring focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/50 lg:w-64"
              />
            </div>
            {/* 一个入口管所有随手记：任务、问题、需求、商机、灵感、项目 */}
            <QuickCreate />
          </div>
        </header>

        <main className="mx-auto w-full max-w-[1240px] px-4 py-6 sm:px-6 lg:px-8">
          <Outlet />
        </main>
      </div>
    </div>
  )
}
