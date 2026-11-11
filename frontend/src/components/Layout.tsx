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
import { useI18n, type MessageKey } from '../i18n'
import Avatar from './Avatar'
import QuickCreate from './QuickCreate'
import LanguageSwitcher from './LanguageSwitcher'
import { Button } from '@/components/ui/button'
import { cn } from '@/lib/utils'
import Logo from './Logo'

interface NavItem {
  path: string
  labelKey: MessageKey
  icon: typeof LayoutDashboard
}

interface NavGroup {
  titleKey: MessageKey
  items: NavItem[]
}

// 分组按「一天怎么用」排，不按模块类型排：
// 工作台是每天进门先看的地方，项目是接单主线（商机赢单才变项目，不能拆两处），
// 收支是钱的两头，设置放不常点但必须找得到的东西。
const NAV_GROUPS: NavGroup[] = [
  {
    titleKey: 'nav.group.workspace',
    items: [
      { path: '/', labelKey: 'nav.dashboard', icon: LayoutDashboard },
      { path: '/today', labelKey: 'nav.today', icon: Sun },
      { path: '/tasks', labelKey: 'nav.tasks', icon: ListTodo },
      { path: '/calendar', labelKey: 'nav.calendar', icon: CalendarDays },
      { path: '/ideas', labelKey: 'nav.ideas', icon: Lightbulb }
    ]
  },
  {
    titleKey: 'nav.group.projects',
    items: [
      { path: '/opportunities', labelKey: 'nav.opportunities', icon: Briefcase },
      { path: '/projects', labelKey: 'nav.projects', icon: FolderKanban }
    ]
  },
  {
    titleKey: 'nav.group.money',
    items: [
      { path: '/receivables', labelKey: 'nav.receivables', icon: HandCoins },
      { path: '/finances', labelKey: 'nav.finances', icon: Wallet }
    ]
  },
  {
    titleKey: 'nav.group.settings',
    items: [{ path: '/profile', labelKey: 'nav.profile', icon: UserRound }]
  }
]

const ALL_ITEMS = NAV_GROUPS.flatMap(g => g.items)

export default function Layout({ onLogout }: { onLogout: () => void }) {
  const location = useLocation()
  const navigate = useNavigate()
  const { t } = useI18n()
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
  const crumbGroupKey = currentGroup && currentGroup.titleKey !== current?.labelKey ? currentGroup.titleKey : null

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
            <Logo />
            <span className="min-w-0">
              <span className="block text-[15px] font-semibold leading-tight tracking-tight">umore</span>
              <span className="block text-[10px] leading-tight text-muted-foreground">{t('common.appTagline')}</span>
            </span>
          </Link>
          <button
            type="button"
            className="rounded-md p-1 text-muted-foreground hover:bg-accent md:hidden"
            onClick={() => setMenuOpen(false)}
            aria-label={t('nav.collapseMenu')}
          >
            <X className="size-4" />
          </button>
        </div>

        <nav className="scrollbar-thin flex h-[calc(100vh-3.5rem-3.75rem)] flex-col gap-4 overflow-y-auto px-3 py-4">
          {NAV_GROUPS.map(group => (
            <div key={group.titleKey}>
              <div className="mb-1.5 px-2 text-[10px] font-semibold uppercase tracking-widest text-muted-foreground/70">
                {t(group.titleKey)}
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
                      <span>{t(item.labelKey)}</span>
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
            title={t('nav.profile')}
          >
            <Avatar user={user} size="size-8" />
            <div className="min-w-0 flex-1">
              <div className="truncate text-[13px] font-medium leading-tight">{user?.nickname || '—'}</div>
              <div className="truncate text-[10px] leading-tight text-muted-foreground">{user?.email || ''}</div>
            </div>
          </Link>
          <Button variant="ghost" size="icon-sm" onClick={handleLogout} title={t('nav.logout')}>
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
            aria-label={t('nav.openMenu')}
          >
            <Menu className="size-5" />
          </button>

          <div className="flex min-w-0 items-center gap-1.5 text-[13px] text-muted-foreground">
            {/* 分组名和页面名撞了（项目 > 项目）就只留一层，省得看着像 bug */}
            {crumbGroupKey && (
              <>
                <span className="hidden sm:inline">{t(crumbGroupKey)}</span>
                <ChevronRight className="hidden size-3.5 sm:inline" />
              </>
            )}
            <span className="truncate font-medium text-foreground">{current ? t(current.labelKey) : t('nav.page')}</span>
          </div>

          <div className="ml-auto flex items-center gap-2">
            <div className="relative hidden sm:block">
              <Search className="pointer-events-none absolute left-2.5 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
              <input
                value={keyword}
                onChange={e => setKeyword(e.target.value)}
                onKeyDown={e => e.key === 'Enter' && search()}
                placeholder={t('nav.searchTasks')}
                className="h-8 w-52 rounded-md border border-input bg-card pl-8 pr-2.5 text-[13px] shadow-sm transition-colors placeholder:text-muted-foreground/70 focus-visible:border-ring focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/50 lg:w-64"
              />
            </div>
            <LanguageSwitcher />
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
