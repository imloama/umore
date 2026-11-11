import { BrowserRouter, HashRouter, Routes, Route, Navigate } from 'react-router-dom'
import { useEffect, useState } from 'react'
import Layout from './components/Layout'
import Login from './pages/Login'
import Dashboard from './pages/Dashboard'
import Tasks from './pages/Tasks'
import Calendar from './pages/Calendar'
import Opportunities from './pages/Opportunities'
import Projects from './pages/Projects'
import Finances from './pages/Finances'
import Receivables from './pages/Receivables'
import Profile from './pages/Profile'
import Ideas from './pages/Ideas'
import { authApi, authToken, clearSession, currentUser, isStaticPreview, storeSession } from './services/api'
import { useI18n } from './i18n'

function App() {
  // 加载态文案也要跟着语言走
  const { t } = useI18n()
  // null = 还在查；true/false = 库里有没有账号（决定默认停在登录还是注册）
  const [hasUsers, setHasUsers] = useState<boolean | null>(null)
  const [isAuthenticated, setIsAuthenticated] = useState(false)

  useEffect(() => {
    checkSetup()
    checkAuth()
  }, [])

  const checkSetup = async () => {
    // 离线预览包没有后端：直接给到登录页，比一直转圈有用
    if (isStaticPreview) {
      setHasUsers(true)
      return
    }
    try {
      const res = await authApi.checkSetup()
      setHasUsers(res.has_users)
    } catch (error) {
      console.error('Failed to check setup:', error)
      // 查询失败也不要把人卡在加载页，给到注册/登录都能走的界面
      setHasUsers(false)
    }
  }

  const checkAuth = () => {
    // 离线预览包没有后端，也就没有登录环节，直接进工作台；
    // 顺手放一个假账号，左下角的昵称和头像才有东西可显示
    if (isStaticPreview) {
      if (!currentUser()) {
        storeSession({ id: 1, email: 'preview@umore.local', email_verified: true, nickname: 'MORE1' })
      }
      setIsAuthenticated(true)
      return
    }
    // token 还在（最长 7 天）+ 手上有账号快照 -> 算登录着
    setIsAuthenticated(!!currentUser() && !!authToken())
  }

  if (hasUsers === null) {
    return (
      <div className="flex min-h-screen items-center justify-center gap-2 text-xs text-muted-foreground">
        <span className="size-3 animate-spin rounded-full border-2 border-border border-t-primary" />
        {t('common.loading')}
      </div>
    )
  }

  // file:// 直接打开（离线预览包）时使用 Hash 路由，否则用 Browser 路由
  const Router = window.location.protocol === 'file:' ? HashRouter : BrowserRouter

  return (
    <Router>
      <Routes>
        <Route
          path="/login"
          element={
            isAuthenticated ? (
              <Navigate to="/" replace />
            ) : (
              <Login
                hasUsers={hasUsers}
                onAuth={() => {
                  setIsAuthenticated(true)
                  // 注册成功后库里就有账号了，再回登录页该停在「登录」
                  setHasUsers(true)
                }}
              />
            )
          }
        />
        <Route
          path="/"
          element={
            isAuthenticated ? (
              <Layout
                onLogout={() => {
                  // 先记下 token：本地一清，拦截器就读不到了
                  const token = authToken()
                  clearSession()
                  setIsAuthenticated(false)
                  // 通知服务端作废这条会话，失败也不拦着本地退出
                  authApi.logout(token).catch(() => undefined)
                }}
              />
            ) : (
              <Navigate to="/login" />
            )
          }
        >
          <Route index element={<Dashboard />} />
          <Route path="today" element={<Tasks todayMode />} />
          <Route path="tasks" element={<Tasks />} />
          <Route path="calendar" element={<Calendar />} />
          <Route path="opportunities" element={<Opportunities />} />
          <Route path="projects" element={<Projects />} />
          <Route path="logs" element={<Navigate to="/tasks" replace />} />
          <Route path="issues" element={<Navigate to="/tasks" replace />} />
          <Route path="finances" element={<Finances />} />
          <Route path="receivables" element={<Receivables />} />
          <Route path="profile" element={<Profile />} />
          <Route path="ideas" element={<Ideas />} />
          {/* 知识库下线，换成了灵感：老书签别 404 */}
          <Route path="knowledge" element={<Navigate to="/ideas" replace />} />
        </Route>
      </Routes>
    </Router>
  )
}

export default App
