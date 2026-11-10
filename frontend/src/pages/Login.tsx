import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { CheckCircle2, KeyRound, Lock, Mail, ShieldCheck } from 'lucide-react'
import { authApi, storeSession } from '../services/api'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { cn } from '@/lib/utils'

interface LoginProps {
  /** 库里已有账号：默认停在登录，否则默认停在注册 */
  hasUsers: boolean
  onAuth: () => void
}

type Mode = 'login' | 'register'

export default function Login({ hasUsers, onAuth }: LoginProps) {
  const [mode, setMode] = useState<Mode>(hasUsers ? 'login' : 'register')
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')
  const [code, setCode] = useState('')
  const [codeLength, setCodeLength] = useState(6)
  const [error, setError] = useState('')
  const [notice, setNotice] = useState('')
  const [loading, setLoading] = useState(false)
  const [sending, setSending] = useState(false)
  // 重发倒计时，秒；归零之后按钮才可点
  const [countdown, setCountdown] = useState(0)
  const navigate = useNavigate()

  const isRegister = mode === 'register'

  // 倒计时只在 >0 时跑一个定时器，归零就把定时器收掉
  const counting = countdown > 0
  useEffect(() => {
    if (!counting) return
    const id = window.setInterval(() => {
      setCountdown(prev => (prev > 0 ? prev - 1 : 0))
    }, 1000)
    return () => window.clearInterval(id)
  }, [counting])

  const switchMode = (next: Mode) => {
    setMode(next)
    setError('')
    setNotice('')
    setConfirmPassword('')
    setCode('')
  }

  const sendCode = async () => {
    setError('')
    setNotice('')
    const trimmed = email.trim()
    if (!trimmed) {
      setError('先填邮箱，验证码发到邮箱里')
      return
    }
    setSending(true)
    try {
      const res = await authApi.sendCode(trimmed)
      setCodeLength(res.code_length || 6)
      setCountdown(res.resend_after || 60)
      setNotice(
        res.delivery === 'log'
          ? '验证码已生成。当前是本地调试模式，没配 Resend，请到服务端日志里看验证码。'
          : `验证码已发到 ${trimmed}，${Math.round((res.expires_in || 600) / 60)} 分钟内有效。`
      )
    } catch (err: any) {
      const status = err.response?.status
      setError(
        err.response?.data?.error ||
          (status === 429 ? '发得太频繁了，等一会儿再试' : '验证码发送失败，检查邮箱或稍后重试')
      )
    } finally {
      setSending(false)
    }
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError('')
    setNotice('')

    if (isRegister) {
      if (password.length < 8) {
        setError('密码长度至少 8 位')
        return
      }
      if (password !== confirmPassword) {
        setError('两次密码输入不一致')
        return
      }
      if (code.trim().length !== codeLength) {
        setError(`请填入 ${codeLength} 位验证码`)
        return
      }
    }

    setLoading(true)
    try {
      const trimmed = email.trim()
      const res = isRegister
        ? await authApi.register(trimmed, password, code.trim())
        : await authApi.login(trimmed, password)
      storeSession(res.user, res.token)
      onAuth()
      navigate('/')
    } catch (err: any) {
      setError(err.response?.data?.error || (isRegister ? '注册失败' : '登录失败'))
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="relative flex min-h-screen items-center justify-center overflow-hidden bg-background px-4">
      {/* 背景装饰：一层柔和渐变，避免登录页太空 */}
      <div className="pointer-events-none absolute -left-32 -top-32 size-[420px] rounded-full bg-primary/10 blur-3xl" />
      <div className="pointer-events-none absolute -bottom-40 -right-24 size-[380px] rounded-full bg-violet-500/10 blur-3xl" />

      <div className="relative w-full max-w-[380px]">
        <div className="mb-6 flex items-center gap-2.5">
          <span className="flex size-9 items-center justify-center rounded-lg bg-primary text-base font-bold text-primary-foreground shadow-sm">
            u
          </span>
          <div>
            <div className="text-[17px] font-semibold leading-tight tracking-tight">umore</div>
            <div className="text-[11px] leading-tight text-muted-foreground">独立开发工作台</div>
          </div>
        </div>

        <div className="rounded-xl border border-border bg-card p-6 shadow-sm">
          {/* 登录 / 注册切换 */}
          <div className="mb-4 flex rounded-lg bg-secondary p-0.5 text-[13px]">
            {([
              { key: 'login', label: '登录' },
              { key: 'register', label: '注册' }
            ] as { key: Mode; label: string }[]).map(item => (
              <button
                key={item.key}
                type="button"
                onClick={() => switchMode(item.key)}
                className={cn(
                  'flex-1 rounded-md py-1.5 transition-colors',
                  mode === item.key
                    ? 'bg-card font-semibold text-foreground shadow-sm'
                    : 'text-muted-foreground hover:text-foreground'
                )}
              >
                {item.label}
              </button>
            ))}
          </div>

          <p className="text-xs leading-relaxed text-muted-foreground">
            {isRegister ? '邮箱验证码通过后才会创建账号' : '邮箱 + 密码登录'}
          </p>

          <form onSubmit={handleSubmit} className="mt-4 space-y-3.5">
            <div>
              <Label htmlFor="email">邮箱</Label>
              <div className="relative">
                <Mail className="pointer-events-none absolute left-2.5 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
                <Input
                  id="email"
                  type="email"
                  value={email}
                  onChange={e => setEmail(e.target.value)}
                  placeholder="you@example.com"
                  className="pl-8"
                  autoComplete="email"
                  autoFocus
                  required
                />
              </div>
            </div>

            {isRegister && (
              <div>
                <Label htmlFor="code">邮箱验证码</Label>
                <div className="flex gap-2">
                  <div className="relative flex-1">
                    <ShieldCheck className="pointer-events-none absolute left-2.5 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
                    <Input
                      id="code"
                      inputMode="numeric"
                      maxLength={codeLength}
                      value={code}
                      onChange={e => setCode(e.target.value.replace(/\D/g, ''))}
                      placeholder={`${codeLength} 位数字`}
                      className="pl-8 tracking-[0.3em]"
                      autoComplete="one-time-code"
                      required
                    />
                  </div>
                  <Button
                    type="button"
                    variant="outline"
                    className="w-[92px] shrink-0 text-[12px]"
                    onClick={sendCode}
                    disabled={sending || countdown > 0}
                  >
                    {sending ? '发送中…' : countdown > 0 ? `${countdown}s` : '获取验证码'}
                  </Button>
                </div>
              </div>
            )}

            <div>
              <Label htmlFor="password">密码</Label>
              <div className="relative">
                <Lock className="pointer-events-none absolute left-2.5 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
                <Input
                  id="password"
                  type="password"
                  value={password}
                  onChange={e => setPassword(e.target.value)}
                  placeholder={isRegister ? '至少 8 位' : '请输入密码'}
                  className="pl-8"
                  autoComplete={isRegister ? 'new-password' : 'current-password'}
                  required
                />
              </div>
            </div>

            {isRegister && (
              <div>
                <Label htmlFor="confirm-password">确认密码</Label>
                <div className="relative">
                  <KeyRound className="pointer-events-none absolute left-2.5 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
                  <Input
                    id="confirm-password"
                    type="password"
                    value={confirmPassword}
                    onChange={e => setConfirmPassword(e.target.value)}
                    placeholder="再次输入密码"
                    className="pl-8"
                    autoComplete="new-password"
                    required
                  />
                </div>
              </div>
            )}

            {notice && (
              <div className="flex items-start gap-1.5 rounded-md bg-emerald-50 px-3 py-2 text-xs leading-relaxed text-emerald-700">
                <CheckCircle2 className="mt-0.5 size-3.5 shrink-0" />
                <span>{notice}</span>
              </div>
            )}
            {error && <div className="rounded-md bg-red-50 px-3 py-2 text-xs text-red-700">{error}</div>}

            <Button type="submit" className="w-full" disabled={loading}>
              {loading ? '处理中…' : isRegister ? '验证并注册' : '登录'}
            </Button>
          </form>

          {/*<p className="mt-4 border-t border-border pt-3 text-[11px] leading-relaxed text-muted-foreground">
            {isRegister
              ? '验证码通过才算注册成功，邮箱记为本机已验证。密码同时用于派生数据加密密钥，忘了只能重建库。'
              : '密码同时用于派生本机数据加密密钥，数据全部存在本机。'}
          </p>*/}
        </div>
      </div>
    </div>
  )
}
