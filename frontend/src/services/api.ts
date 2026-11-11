import type {
  AuthResult,
  SendCodeResult,
  AuthUser,
  Idea,
  Label,
  Project,
  ProjectAmountChange,
  Receivable,
  ReceivableSummary,
  Task,
  TaskComment,
  TaskFilter,
  TaskStats
} from '../types'
import axios from 'axios'

const api = axios.create({
  baseURL: '/api',
  timeout: 10000
})

// file:// 直接打开没有后端，读取接口一律挂起，页面停在加载态而不是报错跳转
export const isStaticPreview = typeof window !== 'undefined' && window.location.protocol === 'file:'

if (isStaticPreview) {
  // 离线预览：按接口形状回空数据，页面骨架和空状态都能看到，不报错也不跳登录
  const previewPayload = (url = '') => {
    if (url.includes('/auth/check-setup')) return { setup_completed: true, has_users: true }
    if (url.includes('/tasks/stats')) {
      return { by_category: {}, by_status: {}, today_total: 0, today_done: 0, today_estimated_hours: 0, today_actual_hours: 0 }
    }
    if (url.includes('/finances/summary')) return { total_income: 0, total_expense: 0, balance: 0 }
    if (url.includes('/receivables/summary')) {
      return {
        pending_count: 0,
        pending_amount: 0,
        overdue_count: 0,
        overdue_amount: 0,
        due_soon_count: 0,
        due_soon_amount: 0,
        done_count: 0,
        done_amount: 0
      }
    }
    return []
  }
  api.interceptors.request.use(config => {
    if ((config.method || 'get').toLowerCase() === 'get') {
      return {
        ...config,
        adapter: async () => ({
          data: previewPayload(config.url),
          status: 200,
          statusText: 'OK',
          headers: {},
          config
        })
      }
    }
    return config
  })
}

// 请求拦截器 - 带上登录 token
// token 放 localStorage：刷新页面、重开标签页都还算登录着（默认 7 天过期）。
api.interceptors.request.use(config => {
  const token = localStorage.getItem('auth_token')
  if (token) {
    config.headers.Authorization = `Bearer ${token}`
  }
  return config
})

// 响应拦截器
api.interceptors.response.use(
  response => response.data,
  error => {
    // 登录/注册接口的 401 是「密码不对」，交给页面提示，别当成掉登录态重定向
    const url = error.config?.url || ''
    if (error.response?.status === 401 && !url.includes('/auth/')) {
      clearSession()
      window.location.href = '/login'
    }
    return Promise.reject(error)
  }
)

export default api

// ---- 认证接口封装 ----

export const authApi = {
  checkSetup: () => api.get<any, { setup_completed: boolean; has_users: boolean }>('/auth/check-setup'),
  // 注册前先发验证码，没码注册不了
  sendCode: (email: string) => api.post<any, SendCodeResult>('/auth/send-code', { email }),
  register: (email: string, password: string, code: string) =>
    api.post<any, AuthResult>('/auth/register', { email, password, code }),
  login: (email: string, password: string) => api.post<any, AuthResult>('/auth/login', { email, password }),
  // 退出登录：服务端删掉那条会话，token 立刻失效。
  // token 显式传入：调用方通常会先把本地 token 清掉，靠拦截器去读就读不到了。
  logout: (token?: string) =>
    api.post('/auth/logout', null, token ? { headers: { Authorization: `Bearer ${token}` } } : undefined),
  me: () => api.get<any, { user: AuthUser }>('/auth/me'),
  /** 改昵称 / 头像地址：没传的字段保持原样 */
  updateProfile: (payload: { nickname?: string; avatar?: string }) =>
    api.put<any, { user: AuthUser }>('/auth/profile', payload),
  changePassword: (oldPassword: string, newPassword: string) =>
    api.post<any, { message: string }>('/auth/password', { old_password: oldPassword, new_password: newPassword }),
  /** 头像走上传，返回落库后的地址 */
  uploadAvatar: (file: File) => {
    const form = new FormData()
    form.append('file', file)
    return api.post<any, { avatar: string }>('/auth/avatar', form)
  }
}

/**
 * 当前账号快照：登录时写一份到 sessionStorage，
 * 页面里到处要显示昵称/头像，别每次渲染都去请求 /auth/me。
 */
const readUser = (): AuthUser | null => {
  const raw = sessionStorage.getItem('auth_user')
  if (!raw) return null
  try {
    return JSON.parse(raw) as AuthUser
  } catch {
    return null
  }
}

let userSnapshot: AuthUser | null = readUser()
const userListeners = new Set<(user: AuthUser | null) => void>()

/** 订阅账号变化（改昵称、换头像后侧边栏要立刻跟着变） */
export const subscribeUser = (listener: (user: AuthUser | null) => void) => {
  userListeners.add(listener)
  return () => {
    userListeners.delete(listener)
  }
}

const setUserSnapshot = (user: AuthUser | null) => {
  userSnapshot = user
  if (user) {
    sessionStorage.setItem('auth_user', JSON.stringify(user))
  } else {
    sessionStorage.removeItem('auth_user')
  }
  userListeners.forEach(listener => listener(user))
}

/** 登录态两件套：token（localStorage）+ 当前账号快照（sessionStorage） */
export const storeSession = (user: AuthUser, token?: string) => {
  if (token) {
    localStorage.setItem('auth_token', token)
  }
  setUserSnapshot(user)
}

/** 账号信息变了（改昵称、换头像）就地更新快照 */
export const storeUser = (user: AuthUser) => setUserSnapshot(user)

export const currentUser = () => userSnapshot

export const clearSession = () => {
  localStorage.removeItem('auth_token')
  setUserSnapshot(null)
}

export const authToken = () => localStorage.getItem('auth_token') || ''

// ---- Task / Label 接口封装 ----

const buildTaskQuery = (filter: TaskFilter = {}) => {
  const params = new URLSearchParams()
  if (filter.category) params.set('category', filter.category)
  if (filter.status) params.set('status', filter.status)
  if (filter.priority) params.set('priority', filter.priority)
  if (filter.project_id) params.set('project_id', String(filter.project_id))
  if (filter.label) params.set('label', filter.label)
  if (filter.keyword) params.set('keyword', filter.keyword)
  if (filter.today) params.set('today', '1')
  if (filter.date) params.set('date', filter.date)
  if (filter.start_date) params.set('start_date', filter.start_date)
  if (filter.end_date) params.set('end_date', filter.end_date)
  const query = params.toString()
  return query ? `?${query}` : ''
}

export const taskApi = {
  list: (filter?: TaskFilter) => api.get<any, Task[]>(`/tasks${buildTaskQuery(filter)}`),
  get: (id: number) => api.get<any, Task>(`/tasks/${id}`),
  create: (task: Task) => api.post<any, Task>('/tasks', task),
  update: (id: number, task: Task) => api.put<any, Task>(`/tasks/${id}`, task),
  remove: (id: number) => api.delete(`/tasks/${id}`),
  setToday: (id: number, isToday: boolean) =>
    api.post<any, Task>(`/tasks/${id}/today`, { is_today: isToday }),
  reorderToday: (items: { id: number; today_order: number }[]) =>
    api.put('/tasks/reorder', items),
  stats: () => api.get<any, TaskStats>('/tasks/stats'),
  comments: (id: number) => api.get<any, TaskComment[]>(`/tasks/${id}/comments`),
  addComment: (id: number, content: string) =>
    api.post<any, TaskComment>(`/tasks/${id}/comments`, { content }),
  removeComment: (commentId: number) => api.delete(`/tasks/comments/${commentId}`)
}

/** 灵感：随手记一条，之后转项目或归档 */
export const ideaApi = {
  list: (status?: Idea['status']) =>
    api.get<any, Idea[]>(`/ideas${status ? `?status=${status}` : ''}`),
  create: (content: string) => api.post<any, Idea>('/ideas', { content }),
  update: (id: number, patch: { content?: string; status?: Idea['status'] }) =>
    api.put<any, Idea>(`/ideas/${id}`, patch),
  /** 转成项目：后端建项目并把这条灵感标成「已成项目」，返回更新后的灵感与项目 */
  promote: (id: number) => api.post<any, { idea: Idea; project: Project }>(`/ideas/${id}/project`),
  remove: (id: number) => api.delete(`/ideas/${id}`)
}

/**
 * 附件上传。
 * 后端按用户隔离落盘，读取走 GET /api/attachments/<id>（同一个 axios 实例，
 * 带上 Authorization 头才拿得到），所以 url 不是公开地址，别人拿不走。
 */
export interface UploadedAttachment {
  id: number
  url: string
  original_name?: string
  mime_type: string
  size: number
}

const postFile = (file: File) => {
  const form = new FormData()
  form.append('file', file)
  return api.post<any, UploadedAttachment>('/attachments', form)
}

export const uploadApi = {
  /** 粘贴截图走这里 */
  image: (file: File) => postFile(file),
  /** 富文本里的文件附件（pdf / txt / office / zip…） */
  file: (file: File) => postFile(file)
}

/**
 * 富文本里的图片怎么显示。
 *
 * 附件接口要 Authorization 头，而 <img src> 带不了头 —— 浏览器会直接 401。
 * 所以这里用同一个 axios 实例把图拉成 blob，再换成 objectURL 给 <img>：
 * 图片和文本一样受鉴权保护，不会因为渲染而变成公开资源。
 * 同一个 URL 只拉一次，多个组件共用结果。
 */
const imageCache = new Map<string, string>()

export const attachmentImageUrl = (url: string) => imageCache.get(url) ?? ''

// 同一张图可能同时被列表里的多个组件要，只发一次请求
const pending = new Map<string, Promise<string>>()

export const loadAttachmentImage = (url: string) => {
  const hit = imageCache.get(url)
  if (hit) return Promise.resolve(hit)
  const inflight = pending.get(url)
  if (inflight) return inflight

  // baseURL 已经是 /api，所以 /api/attachments/12 传给 axios 时要把前缀摘掉；
  // 老地址 /uploads/... 不在 /api 下，原样发
  const path = url.startsWith('/api/') ? url.slice(4) : url
  const task = api
    .get<any, Blob>(path, { responseType: 'blob' })
    .then(res => {
      const objectUrl = URL.createObjectURL(res)
      imageCache.set(url, objectUrl)
      return objectUrl
    })
    .finally(() => pending.delete(url))
  pending.set(url, task)
  return task
}

/** 后端错误对象 → 错误码，配合 i18n 的 te() 出本地文案 */
export const errorCodeOf = (err: unknown) => {
  const e = err as { response?: { data?: { code?: string; error?: string } } }
  return { code: e?.response?.data?.code, message: e?.response?.data?.error }
}

export const projectApi = {
  list: () => api.get<any, Project[]>('/projects'),
  get: (id: number) => api.get<any, Project>(`/projects/${id}`),
  create: (project: Partial<Project>) => api.post<any, Project>('/projects', project),
  update: (id: number, project: Partial<Project>, amountChangeReason?: string) =>
    api.put<any, Project>(`/projects/${id}`, { ...project, amount_change_reason: amountChangeReason }),
  remove: (id: number) => api.delete(`/projects/${id}`),
  amountChanges: (id: number) => api.get<any, ProjectAmountChange[]>(`/projects/${id}/amount-changes`),
  addAmountChange: (id: number, newAmount: number, reason: string) =>
    api.post<any, ProjectAmountChange>(`/projects/${id}/amount-changes`, { new_amount: newAmount, reason }),
  removeAmountChange: (changeId: number) => api.delete(`/projects/amount-changes/${changeId}`),
  comments: (id: number) => api.get<any, TaskComment[]>(`/projects/${id}/comments`),
  addComment: (id: number, content: string) =>
    api.post<any, TaskComment>(`/projects/${id}/comments`, { content }),
  // 项目评论复用任务的删除接口（comments 表同一张）
  removeComment: (commentId: number) => api.delete(`/tasks/comments/${commentId}`)
}

export const receivableApi = {
  list: (params: { status?: string; project_id?: number } = {}) => {
    const query = new URLSearchParams()
    if (params.status) query.set('status', params.status)
    if (params.project_id) query.set('project_id', String(params.project_id))
    const qs = query.toString()
    return api.get<any, Receivable[]>(`/receivables${qs ? `?${qs}` : ''}`)
  },
  summary: () => api.get<any, ReceivableSummary>('/receivables/summary'),
  create: (item: Partial<Receivable>) => api.post<any, Receivable>('/receivables', item),
  // 备注、金额、到期时间随时改，走同一个更新接口
  update: (id: number, item: Partial<Receivable>) => api.put<any, Receivable>(`/receivables/${id}`, item),
  setDone: (id: number, done: boolean) => api.post<any, Receivable>(`/receivables/${id}/status`, { done }),
  remove: (id: number) => api.delete(`/receivables/${id}`)
}

export const labelApi = {
  list: () => api.get<any, Label[]>('/labels'),
  create: (label: Label) => api.post<any, Label>('/labels', label),
  update: (id: number, label: Label) => api.put<any, Label>(`/labels/${id}`, label),
  remove: (id: number) => api.delete(`/labels/${id}`)
}
