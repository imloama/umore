export interface AuthUser {
  id: number
  email: string
  /** 验证码通过才建号，所以注册出来就是 true */
  email_verified: boolean
  /** 注册时按「MORE + ID」自动生成，可在个人资料页改 */
  nickname: string
  /** 头像地址（/uploads/avatars/xxx.png），空着就显示昵称首字母 */
  avatar?: string
}

/** 发验证码的返回：前端靠它设倒计时与输入框长度 */
export interface SendCodeResult {
  message: string
  /** 多少秒后允许重发 */
  resend_after: number
  /** 验证码有效期（秒） */
  expires_in: number
  code_length: number
  /** email = 真发信；log = 没配 Resend，码只打在服务端日志里 */
  delivery: 'email' | 'log'
}

export interface AuthResult {
  message: string
  user: AuthUser
  /** 登录 token，后续请求放在 Authorization: Bearer 里；服务端存缓存，7 天过期 */
  token: string
  /** token 过期时间（RFC3339） */
  expires_at: string
}

export interface DailyLog {
  id?: number
  date: string
  content: string
  status: 'todo' | 'doing' | 'done'
  feedback?: string
  project_id?: number
  created_at?: string
  updated_at?: string
}

export interface Project {
  id?: number
  name: string
  description?: string
  git_repo?: string
  customer?: string
  status: 'active' | 'archived'
  /** 合同总金额，改它会落一条金额变更记录 */
  amount?: number
  /** 以下字段只读，实时汇总而来 */
  task_count?: number
  estimated_hours?: number
  actual_hours?: number
  /** 已收款：关联到本项目的收入流水合计 */
  received?: number
  /** 待收款：项目下还没收到的收款计划笔数与金额 */
  receivable_count?: number
  receivable_amount?: number
  receivable_overdue_amount?: number
  next_due_date?: string
  amount_change_count?: number
  amount_changed_at?: string
  created_at?: string
  updated_at?: string
}

export interface ProjectAmountChange {
  id?: number
  project_id?: number
  old_amount: number
  new_amount: number
  delta: number
  reason?: string
  created_at?: string
}

/** 待收款：v1 只记「一笔钱 + 到期时间 + 备注」，备注随时可改 */
export interface Receivable {
  id?: number
  project_id?: number
  project_name?: string
  title: string
  amount: number
  due_date?: string
  note?: string
  status: 'pending' | 'done'
  done_at?: string
  /** 只读：未收且已过期 */
  overdue?: boolean
  /** 只读：距到期剩余天数，负数表示已过期 */
  days_left?: number
  created_at?: string
  updated_at?: string
}

export interface ReceivableSummary {
  pending_count: number
  pending_amount: number
  overdue_count: number
  overdue_amount: number
  due_soon_count: number
  due_soon_amount: number
  done_count: number
  done_amount: number
}

export type TaskCategory = 'todo' | 'issue' | 'requirement' | 'opportunity'
export type TaskStatus = 'TODO' | 'DOING' | 'BLOCKED' | 'DONE'
export type TaskPriority = 'LOW' | 'MEDIUM' | 'HIGH' | 'URGENT'
export type OpportunityStatus = 'lead' | 'qualifying' | 'proposal' | 'won' | 'lost' | 'dropped'

export interface Label {
  id?: number
  name: string
  color: string
  task_count?: number
  created_at?: string
}

export interface TaskComment {
  id?: number
  target_type?: string
  target_id?: number
  content: string
  content_html?: string
  created_at?: string
}

export interface Task {
  id?: number
  number?: number
  title: string
  content?: string
  category: TaskCategory
  status: TaskStatus
  opportunity_status?: OpportunityStatus | ''
  priority: TaskPriority
  project_id?: number
  project_name?: string
  labels?: Label[]
  due_date?: string
  scheduled_date?: string
  /** 工作量评估（小时），手动填写 */
  estimated_hours?: number
  /** 实际用时（小时），手动填写 */
  actual_hours?: number
  is_today?: boolean
  today_order?: number
  closed_at?: string
  created_at?: string
  updated_at?: string
}

export interface TaskStats {
  by_category: Record<string, number>
  by_status: Record<string, number>
  today_total: number
  today_done: number
  today_estimated_hours: number
  today_actual_hours: number
}

export interface Attachment {
  id: number
  url: string
  original_name?: string
  mime_type?: string
  size?: number
  created_at?: string
}

export interface TaskFilter {
  category?: string
  status?: string
  priority?: string
  project_id?: number
  label?: string
  keyword?: string
  today?: boolean
  date?: string
  start_date?: string
  end_date?: string
}

export interface Issue {
  id?: number
  title: string
  type: 'issue' | 'requirement' | 'opportunity'
  description?: string
  status: 'pending' | 'processing' | 'resolved' | 'closed'
  priority: 'low' | 'medium' | 'high'
  project_id?: number
  created_at?: string
  updated_at?: string
}

export interface Finance {
  id?: number
  type: 'income' | 'expense'
  amount: number
  category?: string
  description?: string
  date: string
  project_id?: number
  created_at?: string
  updated_at?: string
}

/** 灵感状态：待整理 / 已成项目 / 已归档 */
export type IdeaStatus = 'inbox' | 'project' | 'archived'

/** 灵感：想到什么随手记一条，正文就是全部内容，之后要么转项目要么归档 */
export interface Idea {
  id?: number
  content: string
  status: IdeaStatus
  /** 转成项目后指回去 */
  project_id?: number
  /** 只读：关联项目名 */
  project_name?: string
  created_at?: string
  updated_at?: string
}
