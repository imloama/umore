import { useEffect, useRef, useState } from 'react'
import { Camera, CheckCircle2, Languages as LanguagesIcon, LockKeyhole, Mail, UserRound } from 'lucide-react'
import { authApi, currentUser, errorCodeOf, storeUser } from '../services/api'
import type { AuthUser } from '../types'
import { LOCALE_LABEL, SUPPORTED_LOCALES, useI18n } from '@/i18n'
import PageHeader from '../components/PageHeader'
import Avatar from '../components/Avatar'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Separator } from '@/components/ui/separator'

export default function Profile() {
  const { t, te, locale, setLocale } = useI18n()
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
      setProfileErr(t('profile.nicknameEmpty'))
      return
    }
    setSavingProfile(true)
    setProfileErr('')
    setProfileMsg('')
    try {
      const res = await authApi.updateProfile({ nickname: next })
      setUser(res.user)
      storeUser(res.user)
      setProfileMsg(t('profile.nicknameSaved'))
    } catch (err: unknown) {
      const { code } = errorCodeOf(err)
      setProfileErr(code ? te(code) : t('profile.saveFailed'))
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
      setProfileMsg(t('profile.avatarUpdated'))
    } catch (err: unknown) {
      const { code } = errorCodeOf(err)
      setProfileErr(code ? te(code) : t('profile.avatarFailed'))
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
      setPwdErr(t('profile.newPasswordTooShort'))
      return
    }
    if (newPassword !== repeatPassword) {
      setPwdErr(t('profile.newPasswordMismatch'))
      return
    }
    setSavingPwd(true)
    try {
      await authApi.changePassword(oldPassword, newPassword)
      setPwdMsg(t('profile.passwordUpdated'))
      setOldPassword('')
      setNewPassword('')
      setRepeatPassword('')
    } catch (err: unknown) {
      const { code } = errorCodeOf(err)
      setPwdErr(code ? te(code) : t('profile.passwordFailed'))
    } finally {
      setSavingPwd(false)
    }
  }

  return (
    <div className="max-w-2xl">
      <PageHeader eyebrow="PROFILE" title={t('profile.title')} description={t('profile.description')} />

      <Card className="p-5">
        <div className="flex items-center gap-4">
          <div className="relative">
            <Avatar user={user} size="size-16" />
            <button
              type="button"
              onClick={() => fileRef.current?.click()}
              disabled={uploading}
              className="absolute -bottom-1 -right-1 flex size-6 items-center justify-center rounded-full border border-border bg-card text-muted-foreground shadow-sm transition-colors hover:text-foreground disabled:opacity-50"
              title={t('profile.changeAvatar')}
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
            {uploading && <div className="mt-1 text-[11px] text-muted-foreground">{t('profile.uploading')}</div>}
          </div>
        </div>

        <Separator className="my-4" />

        <form onSubmit={saveProfile} className="space-y-3">
          <div>
            <Label>
              <UserRound className="mr-1 inline size-3.5 align-[-2px]" />
              {t('profile.nickname')}
            </Label>
            <div className="flex gap-2">
              <Input
                value={nickname}
                onChange={e => setNickname(e.target.value)}
                maxLength={32}
                placeholder={t('profile.nicknamePlaceholder')}
              />
              <Button type="submit" disabled={savingProfile || nickname.trim() === (user?.nickname || '')}>
                {t('common.save')}
              </Button>
            </div>
            <p className="mt-1 text-[11px] text-muted-foreground">{t('profile.nicknameHint')}</p>
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
          {t('profile.changePassword')}
        </div>
        <form onSubmit={savePassword} className="space-y-3">
          <div>
            <Label htmlFor="old-password">{t('profile.oldPassword')}</Label>
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
              <Label htmlFor="new-password">{t('profile.newPassword')}</Label>
              <Input
                id="new-password"
                type="password"
                autoComplete="new-password"
                value={newPassword}
                onChange={e => setNewPassword(e.target.value)}
                placeholder={t('auth.passwordPlaceholderRegister')}
                required
              />
            </div>
            <div>
              <Label htmlFor="repeat-password">{t('profile.repeatPassword')}</Label>
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
            {savingPwd ? t('common.saving') : t('profile.updatePassword')}
          </Button>
        </form>
      </Card>

      {/* 语言只存在这台浏览器上：换设备/换浏览器各记一份，不写数据库 */}
      <Card className="mt-4 p-5">
        <div className="mb-3 flex items-center gap-1.5 text-[13px] font-semibold">
          <LanguagesIcon className="size-4 text-muted-foreground" />
          {t('profile.language')}
        </div>
        <div className="flex flex-wrap gap-1.5">
          {SUPPORTED_LOCALES.map(code => (
            <Button
              key={code}
              type="button"
              variant={locale === code ? 'default' : 'outline'}
              size="sm"
              onClick={() => setLocale(code)}
            >
              {LOCALE_LABEL[code]}
            </Button>
          ))}
        </div>
        <p className="mt-2 text-[11px] text-muted-foreground">{t('profile.languageHint')}</p>
      </Card>
    </div>
  )
}
