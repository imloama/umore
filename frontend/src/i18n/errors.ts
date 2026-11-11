/**
 * 后端错误码 → 界面文案。
 *
 * 后端只回 `{ error, code }`：error 是英文兜底（排查用），code 是稳定标识。
 * 这里按码查表渲染，所以加一个后端错误码时，两边都要补一条。
 */
import type { Locale } from './locales'

type ErrorTable = Record<string, { zh: string; en: string }>

export const errorMessages: ErrorTable = {
  // 通用
  unauthorized: { zh: '登录状态已失效，请重新登录', en: 'Session expired, please sign in again' },
  access_denied: { zh: '没有权限', en: 'Not allowed' },
  internal_error: { zh: '服务端出错了，稍后再试', en: 'Server error, please try again later' },
  not_found: { zh: '记录不存在', en: 'Not found' },
  invalid_params: { zh: '参数格式不对', en: 'Invalid request' },
  invalid_id: { zh: '记录编号不对', en: 'Invalid id' },
  missing_field: { zh: '必填项没填完', en: 'Required fields are missing' },
  reload_failed: { zh: '保存成功，但回读失败，刷新一下看看', en: 'Saved, but reloading failed — refresh the page' },

  // 账号
  auth_email_required: { zh: '邮箱必填', en: 'Email is required' },
  auth_email_invalid: { zh: '邮箱格式不对', en: 'Invalid email address' },
  auth_email_taken: { zh: '该邮箱已注册', en: 'This email is already registered' },
  auth_password_too_short: { zh: '密码至少 8 位', en: 'Password must be at least 8 characters' },
  auth_password_same: { zh: '新密码与原密码相同', en: 'New password is the same as the current one' },
  auth_password_wrong: { zh: '邮箱或密码不正确', en: 'Email or password is incorrect' },
  auth_code_wrong: { zh: '验证码不正确', en: 'Verification code is incorrect' },
  auth_code_too_frequent: { zh: '验证码发送太频繁，请稍后再试', en: 'Too many codes requested, try again later' },
  auth_nickname_too_long: { zh: '昵称最多 32 个字', en: 'Nickname is limited to 32 characters' },
  auth_mail_not_configured: { zh: '邮件服务未配置，无法发送验证码', en: 'Mail service is not configured' },
  auth_mail_send_failed: { zh: '验证码发送失败，请稍后重试', en: 'Could not send the code, please retry' },

  // 附件
  attachment_file_required: { zh: '没有选择文件', en: 'No file selected' },
  attachment_file_too_large: { zh: '文件太大', en: 'File is too large' },
  attachment_file_type_rejected: { zh: '不支持这种文件类型（图片、PDF、txt、csv、md、json、Office、zip）', en: 'Unsupported file type' },
  attachment_read_failed: { zh: '文件读取失败', en: 'Could not read the file' },
  attachment_save_failed: { zh: '文件保存失败', en: 'Could not save the file' },

  // 业务
  project_not_found: { zh: '关联的项目不存在', en: 'Project not found' },
  project_name_taken: { zh: '同名项目已存在', en: 'A project with that name already exists' },
  label_name_taken: { zh: '同名标签已存在', en: 'A tag with that name already exists' },
  label_name_required: { zh: '标签名必填', en: 'Tag name is required' },
  task_not_found: { zh: '任务不存在', en: 'Task not found' },
  task_title_required: { zh: '标题必填', en: 'Title is required' },
  idea_not_found: { zh: '灵感不存在', en: 'Idea not found' },
  idea_content_required: { zh: '内容不能为空', en: 'Content cannot be empty' },
  comment_content_required: { zh: '内容不能为空', en: 'Content cannot be empty' },
  amount_invalid: { zh: '金额要填个非负数字', en: 'Amount must be a non-negative number' },
  date_range_required: { zh: '开始和结束日期必填', en: 'Start and end dates are required' }
}

/** 把后端错误翻成本地文案；认不出码就先用后端给的英文兜底 */
export const translateError = (code: string | undefined, fallback: string | undefined, locale: Locale) => {
  if (!code) return fallback || ''
  const entry = errorMessages[code]
  if (!entry) return fallback || code
  return entry[locale]
}
