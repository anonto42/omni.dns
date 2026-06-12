import { useState, useEffect } from 'react'
import { toast } from 'sonner'
import { useAuth } from '@/hooks/useAuth'
import { apiPut } from '@/hooks/api'
import { dispatchNotificationsUpdate } from '@/lib/notifications'
import { PageTransition } from '@/components/shared/PageTransition'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '@/components/ui/card'

/** Profile page — account details and password management. */
export default function ProfilePage() {
  const { user, updateProfile } = useAuth()
  const [displayName, setDisplayName] = useState('')
  const [email, setEmail] = useState('')
  const [currentPw, setCurrentPw] = useState('')
  const [newPw, setNewPw] = useState('')
  const [confirmPw, setConfirmPw] = useState('')
  const [saving, setSaving] = useState(false)

  useEffect(() => {
    if (user) {
      setDisplayName(user.name || '')
      setEmail(user.email || '')
    }
  }, [user])

  const handleDiscard = () => {
    if (user) {
      setDisplayName(user.name || '')
      setEmail(user.email || '')
    }
    setCurrentPw('')
    setNewPw('')
    setConfirmPw('')
  }

  const handleSave = async () => {
    const isChangingPassword = currentPw || newPw || confirmPw
    if (isChangingPassword) {
      if (newPw !== confirmPw) { toast.error('Passwords do not match'); return }
      if (newPw.length < 8) { toast.error('Password must be at least 8 characters'); return }
    }
    if (!displayName.trim() || !email.trim()) {
      toast.error('Display Name and Email cannot be empty')
      return
    }

    setSaving(true)
    try {
      const profileChanged = displayName.trim() !== (user?.name || '') || email.trim().toLowerCase() !== (user?.email || '').toLowerCase()
      if (profileChanged) {
        await updateProfile(displayName.trim(), email.trim().toLowerCase())
        toast.success('Profile updated successfully')
        dispatchNotificationsUpdate()
      }

      if (isChangingPassword) {
        const res = await apiPut('/password', { current_password: currentPw, new_password: newPw }) as { ok?: boolean; error?: string }
        if (res.ok) {
          toast.success('Password changed successfully')
          dispatchNotificationsUpdate()
          setCurrentPw(''); setNewPw(''); setConfirmPw('')
        } else {
          toast.error(res.error ?? 'Failed to change password')
        }
      }
    } catch (err: any) {
      toast.error(err.message || 'Failed to save changes')
    } finally {
      setSaving(false)
    }
  }

  return (
    <PageTransition>
      <div className="space-y-6 md:space-y-8">
        <div className="space-y-1">
          <h1 className="text-xl sm:text-2xl font-bold tracking-tight text-foreground">Profile</h1>
          <p className="text-muted-foreground text-[10px] font-bold uppercase tracking-widest">Manage your account details and preferences.</p>
        </div>
        <div className="grid gap-6">
          <Card className="shadow-sm" data-tour="profile-card">
            <CardHeader>
              <CardTitle className="font-bold tracking-tight text-foreground">Account Details</CardTitle>
              <CardDescription className="text-[10px] font-bold uppercase tracking-widest">Your dynamic NetShield account profile info.</CardDescription>
            </CardHeader>
            <CardContent className="space-y-4">
              <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4 py-2 border-b border-border/40">
                <p className="text-sm font-bold text-muted-foreground uppercase tracking-wider text-[11px]">Account Type</p>
                <p className="text-sm font-bold text-foreground">System Administrator</p>
              </div>
              <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4 py-2 border-b border-border/40">
                <p className="text-sm font-bold text-muted-foreground uppercase tracking-wider text-[11px]">Display Name</p>
                <input
                  type="text"
                  value={displayName}
                  onChange={e => setDisplayName(e.target.value)}
                  className="flex h-10 w-full sm:w-64 input-premium"
                />
              </div>
              <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4 py-2">
                <p className="text-sm font-bold text-muted-foreground uppercase tracking-wider text-[11px]">Email Address</p>
                <input
                  type="email"
                  value={email}
                  onChange={e => setEmail(e.target.value)}
                  className="flex h-10 w-full sm:w-64 input-premium"
                />
              </div>
            </CardContent>
          </Card>

          <Card className="shadow-sm">
            <CardHeader>
              <CardTitle className="font-bold tracking-tight text-foreground">Change Password</CardTitle>
              <CardDescription className="text-[10px] font-bold uppercase tracking-widest">Update your admin account password.</CardDescription>
            </CardHeader>
            <CardContent className="space-y-6">
              {(['Current Password', 'New Password', 'Confirm New Password'] as const).map((label, i) => {
                const val = [currentPw, newPw, confirmPw][i]
                const setter = [setCurrentPw, setNewPw, setConfirmPw][i]
                return (
                  <div key={label} className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
                    <p className="text-sm font-bold text-foreground">{label}</p>
                    <input type="password" value={val} onChange={e => setter(e.target.value)} className="flex h-10 w-full sm:w-64 input-premium" />
                  </div>
                )
              })}
            </CardContent>
          </Card>
          <div className="flex flex-col sm:flex-row justify-end gap-2 sm:gap-3">
            <Button variant="outline" className="w-full sm:w-auto text-[10px] font-bold uppercase tracking-widest btn-premium" onClick={handleDiscard}>Discard</Button>
            <Button className="w-full sm:w-auto shadow-sm text-[10px] font-bold uppercase tracking-widest btn-premium glow-primary" onClick={handleSave} disabled={saving}>{saving ? 'Saving…' : 'Save Changes'}</Button>
          </div>
        </div>
      </div>
    </PageTransition>
  )
}
