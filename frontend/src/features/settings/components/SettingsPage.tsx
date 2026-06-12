import { useState, useEffect } from 'react'
import { toast } from 'sonner'
import { useTheme } from '@/hooks/useTheme'
import { dispatchNotificationsUpdate } from '@/lib/notifications'
import { PageTransition } from '@/components/shared/PageTransition'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '@/components/ui/card'
import { Switch } from '@/components/ui/switch'
import { Skeleton } from '@/components/ui/skeleton'
import { getSettings, saveSettings } from '../api'
import { UPSTREAM_OPTIONS, THEME_OPTIONS } from '../constants'

/** Settings page — appearance, DNS behaviour, and upstream DNS provider. */
export default function SettingsPage() {
  const { theme, setTheme } = useTheme()
  const [upstream, setUpstream] = useState('1.1.1.1:853')
  const [customUpstream, setCustomUpstream] = useState('')
  const [blockNXDomain, setBlockNXDomain] = useState(false)
  const [saving, setSaving] = useState(false)
  const [loaded, setLoaded] = useState(false)

  useEffect(() => {
    getSettings().then((s) => {
      if (s.upstream_dns) {
        const isKnown = UPSTREAM_OPTIONS.find(o => o.value === s.upstream_dns)
        if (isKnown) {
          setUpstream(s.upstream_dns)
        } else {
          // Custom address saved previously — restore it into the custom input
          setUpstream('custom')
          setCustomUpstream(s.upstream_dns)
        }
      }
      if (s.block_nxdomain) setBlockNXDomain(s.block_nxdomain === 'true')
      setLoaded(true)
    }).catch(() => setLoaded(true))
  }, [])

  const handleSave = async () => {
    const resolvedUpstream = upstream === 'custom' ? customUpstream.trim() : upstream
    if (!resolvedUpstream) {
      toast.error('Enter a custom DNS address')
      return
    }
    setSaving(true)
    try {
      await saveSettings({ upstream_dns: resolvedUpstream, block_nxdomain: String(blockNXDomain) })
      toast.success('Settings saved', { description: `Upstream: ${resolvedUpstream}` })
      dispatchNotificationsUpdate()
    } catch {
      toast.error('Failed to save settings')
    } finally {
      setSaving(false)
    }
  }

  const isCustom = upstream === 'custom'

  return (
    <PageTransition>
      <div className="space-y-6 md:space-y-8">
        <div className="space-y-1">
          <h1 className="text-xl sm:text-2xl font-bold tracking-tight text-foreground">Settings</h1>
          <p className="text-muted-foreground text-[10px] font-bold uppercase tracking-widest">Configure your DNS server and security preferences.</p>
        </div>

        {!loaded ? (
          <div className="grid gap-6">
            <Card className="shadow-sm">
              <CardHeader><Skeleton className="h-5 w-40" /></CardHeader>
              <CardContent className="space-y-4">
                <Skeleton className="h-10 w-full" />
                <Skeleton className="h-10 w-full" />
              </CardContent>
            </Card>
          </div>
        ) : (
          <div className="grid gap-6" data-tour="settings-card">
            <Card className="shadow-sm">
              <CardHeader>
                <CardTitle className="font-bold tracking-tight text-foreground">Appearance</CardTitle>
                <CardDescription className="text-[10px] font-bold uppercase tracking-widest">Choose between light, dark, or follow your device setting.</CardDescription>
              </CardHeader>
              <CardContent>
                <div className="grid grid-cols-3 gap-3">
                  {THEME_OPTIONS.map(({ label, value, icon: Icon }) => (
                    <button
                      key={value}
                      onClick={() => setTheme(value)}
                      className={`flex flex-col items-center gap-2 p-4 transition-colors cursor-pointer ${theme === value ? 'bg-primary/10 text-primary' : 'bg-muted/40 text-muted-foreground hover:bg-muted/70 hover:text-foreground'}`}
                    >
                      <Icon className="h-5 w-5" />
                      <span className="text-xs font-bold uppercase tracking-widest">{label}</span>
                    </button>
                  ))}
                </div>
              </CardContent>
            </Card>

            <Card className="shadow-sm">
              <CardHeader>
                <CardTitle className="font-bold tracking-tight text-foreground">DNS Behaviour</CardTitle>
                <CardDescription className="text-[10px] font-bold uppercase tracking-widest">Control how blocked domains are answered.</CardDescription>
              </CardHeader>
              <CardContent className="space-y-6">
                <div className="flex items-start justify-between gap-6">
                  <div className="space-y-1">
                    <p className="text-sm font-bold text-foreground">Return NXDOMAIN for blocked domains</p>
                    <p className="text-[10px] font-bold text-muted-foreground uppercase tracking-widest">
                      Off — returns <span className="font-mono text-foreground">0.0.0.0</span> (sink-hole, faster for clients)
                    </p>
                    <p className="text-[10px] font-bold text-muted-foreground uppercase tracking-widest">
                      On — returns <span className="font-mono text-foreground">NXDOMAIN</span> (domain does not exist)
                    </p>
                  </div>
                  <Switch checked={blockNXDomain} onCheckedChange={setBlockNXDomain} />
                </div>
              </CardContent>
            </Card>

            <Card className="shadow-sm">
              <CardHeader>
                <CardTitle className="font-bold tracking-tight text-foreground">Upstream DNS</CardTitle>
                <CardDescription className="text-[10px] font-bold uppercase tracking-widest">Select your preferred upstream DNS provider for resolution.</CardDescription>
              </CardHeader>
              <CardContent className="space-y-4">
                <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
                  {UPSTREAM_OPTIONS.map((opt) => (
                    <div key={opt.value} onClick={() => setUpstream(opt.value)} className={`flex items-center space-x-3 p-3 transition-colors cursor-pointer ${upstream === opt.value ? 'bg-primary/10 text-primary' : 'bg-muted/40 hover:bg-muted/70'}`}>
                      <div className={`h-4 w-4 flex items-center justify-center transition-colors ${upstream === opt.value ? 'bg-primary' : 'bg-muted'}`}>
                        {upstream === opt.value && <div className="h-1.5 w-1.5 bg-primary-foreground" />}
                      </div>
                      <span className="text-sm font-bold text-foreground">{opt.label}</span>
                    </div>
                  ))}
                  <div onClick={() => setUpstream('custom')} className={`flex items-center space-x-3 p-3 transition-colors cursor-pointer ${isCustom ? 'bg-primary/10 text-primary' : 'bg-muted/40 hover:bg-muted/70'}`}>
                    <div className={`h-4 w-4 flex items-center justify-center transition-colors ${isCustom ? 'bg-primary' : 'bg-muted'}`}>
                      {isCustom && <div className="h-1.5 w-1.5 bg-primary-foreground" />}
                    </div>
                    <span className="text-sm font-bold text-foreground">Custom Provider</span>
                  </div>
                </div>
                {isCustom && (
                  <div className="space-y-2">
                    <input
                      className="flex h-10 w-full font-mono text-sm input-premium"
                      placeholder="e.g. 192.168.1.1:53 or 192.168.1.1:853"
                      value={customUpstream}
                      onChange={e => setCustomUpstream(e.target.value)}
                      spellCheck={false}
                      autoComplete="off"
                    />
                    <p className="text-[10px] font-bold text-muted-foreground uppercase tracking-widest">
                      Use <span className="font-mono text-foreground">:853</span> for DNS-over-TLS (encrypted) · <span className="font-mono text-foreground">:53</span> for plain UDP (ISP can see queries)
                    </p>
                  </div>
                )}
              </CardContent>
            </Card>

            <div className="flex flex-col sm:flex-row justify-end gap-2 sm:gap-3">
              <Button variant="outline" className="w-full sm:w-auto text-[10px] font-bold uppercase tracking-widest btn-premium" onClick={() => { setUpstream('1.1.1.1:853'); setBlockNXDomain(false) }}>Discard Changes</Button>
              <Button className="w-full sm:w-auto shadow-sm text-[10px] font-bold uppercase tracking-widest btn-premium glow-primary" onClick={handleSave} disabled={saving}>{saving ? 'Saving…' : 'Save Configuration'}</Button>
            </div>
          </div>
        )}
      </div>
    </PageTransition>
  )
}
