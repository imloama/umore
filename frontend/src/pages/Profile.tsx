import { useEffect, useRef, useState } from 'react'
import { Camera, CheckCircle2, LockKeyhole, Mail, UserRound } from 'lucide-react'
import { authApi, currentUser, storeUser } from '../services/api'
import type { AuthUser } from '../types'
import PageHeader from '../components/PageHeader'
import Avatar from '../components/Avatar'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Separator } from '@/components/ui/separator'

export default function Profile() {
  const [user, setUser] = useState<AuthUser | null>(currentUser())
  const [nickname, setNickname] = useState(currentUser()?.nickname || '')
  const [profileMsg, setProfileMsg] = useState('')
  const [profileErr, setProfileErr] = useState('')
  const [savingProfile, setSavingProfile] = useState(false)
  const [uploading, setUploading] = useState(false)

  const [oldPassword, setOldPassword] = useState('')
  const [newPassword, setNewPassword] = useState('')
  const [repeatPassword, setRepeatPassword] = useState('')
  const [pwdMsg, setPwdMsg] = useState('')
  const [pwdErr, setPwdErr] = useState('')
  const [savingPwd, setSavingPwd] = useState(false)

  const fileRef = useRef<HTMLInputElement>(null)

  // 侧边栏、本页共用同一份快照：这里改完立刻生效
  useEffect(() => {
    authApi
      .me()
      .then(res => {
        setUser(res.user)
        setNickname(res.user.nickname || '')
        storeUser(res.user)
      })
      .catch(() => undefined)
  }, [])

  const saveProfile = async (e: React.FormEvent) => {
    e.preventDefault()
    const next = nickname.trim()
    if (!next) {
      setProfileErr('昵称不能空着')
      return
    }
    setSavingProfile(true)
    setProfileErr('')
    setProfileMsg('')
    try {
      const res = await authApi.updateProfile({ nickname: next })
      setUser(res.user)
      storeUser(res.user)
      setProfileMsg('昵称已保存')
    } catch (err: any) {
      setProfileErr(err.response?.data?.error || '保存失败')
    } finally {
      setSavingProfile(false)
    }
  }

  const pickAvatar = async (file?: File) => {
    if (!file) return
    setUploading(true)
    setProfileErr('')
    setProfileMsg('')
    try {
      const res = await authApi.uploadAvatar(file)
      const next = { ...(user || ({} as AuthUser)), avatar: res.avatar } as AuthUser
      setUser(next)
      storeUser(next)
      setProfileMsg('头像已更新')
    } catch (err: any) {
      setProfileErr(err.response?.data?.error || '头像上传失败（只支持 png/jpeg/gif/webp，10MB 以内）')
    } finally {
      setUploading(false)
      if (fileRef.current) fileRef.current.value = ''
    }
  }

  const savePassword = async (e: React.FormEvent) => {
    e.preventDefault()
    setPwdErr('')
    setPwdMsg('')
    if (newPassword.length < 8) {
      setPwdErr('新密码至少 8 位')
      return
    }
    if (newPassword !== repeatPassword) {
      setPwdErr('两次输入的新密码不一致')
      return
    }
    setSavingPwd(true)
    try {
      await authApi.changePassword(oldPassword, newPassword)
      setPwdMsg('密码已更新。下次登录用新密码。')
      setOldPassword('')
      setNewPassword('')
      setRepeatPassword('')
    } catch (err: any) {
      setPwdErr(err.response?.data?.error || '改密码失败')
    } finally {
      setSavingPwd(false)
    }
  }

  return (
    <div className="max-w-2xl">
      <PageHeader eyebrow="PROFILE" title="个人资料" description="改昵称、换头像、换密码都在这儿。" />

      <Card className="p-5">
        <div className="flex items-center gap-4">
          <div className="relative">
            <Avatar user={user} size="size-16" />
            <button
              type="button"
              onClick={() => fileRef.current?.click()}
              disabled={uploading}
              className="absolute -bottom-1 -right-1 flex size-6 items-center justify-center rounded-full border border-border bg-card text-muted-foreground shadow-sm transition-colors hover:text-foreground disabled:opacity-50"
              title="换头像"
            >
              <Camera className="size-3.5" />
            </button>
            <input
              ref={fileRef}
              type="file"
              accept="image/png,image/jpeg,image/gif,image/webp"
              className="hidden"
              onChange={e => pickAvatar(e.target.files?.[0])}
            />
          </div>
          <div className="min-w-0">
            <div className="truncate text-[15px] font-semibold">{user?.nickname || '—'}</div>
            <div className="mt-0.5 flex items-center gap-1 truncate text-[12px] text-muted-foreground">
              <Mail className="size-3.5 shrink-0" />
              {user?.email || '—'}
            </div>
            {uploading && <div className="mt-1 text-[11px] text-muted-foreground">上传中…</div>}
          </div>
        </div>

        <Separator className="my-4" />

        <form onSubmit={saveProfile} className="space-y-3">
          <div>
            <Label>
              <UserRound className="mr-1 inline size-3.5 align-[-2px]" />
              昵称
            </Label>
            <div className="flex gap-2">
              <Input value={nickname} onChange={e => setNickname(e.target.value)} maxLength={32} placeholder="比如 MORE1" />
              <Button type="submit" disabled={savingProfile || nickname.trim() === (user?.nickname || '')}>
                保存
              </Button>
            </div>
            <p className="mt-1 text-[11px] text-muted-foreground">注册时默认给一个「MORE + ID」，想改就改，最多 32 个字。</p>
          </div>
        </form>

        {profileMsg && (
          <div className="mt-2 flex items-center gap-1.5 text-[12px] text-emerald-700">
            <CheckCircle2 className="size-3.5" />
            {profileMsg}
          </div>
        )}
        {profileErr && <div className="mt-2 text-[12px] text-red-600">{profileErr}</div>}
      </Card>

      <Card className="mt-4 p-5">
        <div className="mb-3 flex items-center gap-1.5 text-[13px] font-semibold">
          <LockKeyhole className="size-4 text-muted-foreground" />
          修改密码
        </div>
        <form onSubmit={savePassword} className="space-y-3">
          <div>
            <Label htmlFor="old-password">原密码</Label>
            <Input
              id="old-password"
              type="password"
              autoComplete="current-password"
              value={oldPassword}
              onChange={e => setOldPassword(e.target.value)}
              required
            />
          </div>
          <div className="grid gap-3 sm:grid-cols-2">
            <div>
              <Label htmlFor="new-password">新密码</Label>
              <Input
                id="new-password"
                type="password"
                autoComplete="new-password"
                value={newPassword}
                onChange={e => setNewPassword(e.target.value)}
                placeholder="至少 8 位"
                required
              />
            </div>
            <div>
              <Label htmlFor="repeat-password">确认新密码</Label>
              <Input
                id="repeat-password"
                type="password"
                autoComplete="new-password"
                value={repeatPassword}
                onChange={e => setRepeatPassword(e.target.value)}
                required
              />
            </div>
          </div>
          {pwdMsg && (
            <div className="flex items-center gap-1.5 text-[12px] text-emerald-700">
              <CheckCircle2 className="size-3.5" />
              {pwdMsg}
            </div>
          )}
          {pwdErr && <div className="text-[12px] text-red-600">{pwdErr}</div>}
          <Button type="submit" disabled={savingPwd}>
            {savingPwd ? '保存中…' : '更新密码'}
          </Button>
        </form>
      </Card>
    </div>
  )
}
