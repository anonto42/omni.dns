import React from 'react'
import { AlertCircle, Eye, EyeOff } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { OmniDNSLogo } from '@/components/shared/OmniDNSLogo'
import type { LoginViewModel } from '../hooks/useLogin'

interface LoginPresenterProps extends LoginViewModel {}

export const LoginPresenter: React.FC<LoginPresenterProps> = ({
  email,
  setEmail,
  password,
  setPassword,
  showPw,
  setShowPw,
  error,
  loading,
  handleSubmit,
}) => {
  return (
    <div className="min-h-screen flex items-center justify-center bg-gradient-to-br from-background via-muted/30 to-background p-4">
      <Card className="w-full max-w-sm shadow-2xl glass-panel rounded-none border-border/40 bg-card">
        <CardHeader className="text-center pb-2">
          <div className="mx-auto mb-4 w-12 h-12 flex items-center justify-center">
            <OmniDNSLogo size={44} />
          </div>
          <CardTitle className="text-xl font-bold tracking-tight text-foreground">OmniDNS</CardTitle>
        </CardHeader>
        <CardContent>
          <form onSubmit={handleSubmit} className="space-y-4">
            {error && (
              <div className="flex items-start gap-2 p-3 bg-red-500/10 border border-red-500/30 text-red-400 text-xs font-semibold rounded-none">
                <AlertCircle className="h-4 w-4 shrink-0 mt-0.5" />
                <div className="flex-1">{error}</div>
              </div>
            )}
            <div className="space-y-2">
              <label htmlFor="email" className="text-[10px] font-bold text-foreground uppercase tracking-widest">
                Email Address
              </label>
              <input
                id="email"
                name="email"
                type="email"
                value={email}
                onChange={e => setEmail(e.target.value)}
                className="w-full bg-background dark:bg-[#14191f] px-3 py-2 text-sm text-foreground placeholder:text-muted-foreground/60 border border-border rounded-none focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-primary/40 focus-visible:bg-background dark:focus-visible:bg-[#000204] transition-all duration-200 font-medium h-10"
                placeholder="admin@omnidns.local"
                autoFocus
              />
            </div>
            <div className="space-y-2">
              <label htmlFor="password" className="text-[10px] font-bold text-foreground uppercase tracking-widest">
                Password
              </label>
              <div className="relative">
                <input
                  id="password"
                  name="password"
                  type={showPw ? 'text' : 'password'}
                  value={password}
                  onChange={e => setPassword(e.target.value)}
                  className="w-full bg-background dark:bg-[#14191f] px-3 py-2 pr-10 text-sm text-foreground placeholder:text-muted-foreground/60 border border-border rounded-none focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-primary/40 focus-visible:bg-background dark:focus-visible:bg-[#000204] transition-all duration-200 font-medium h-10"
                  placeholder="Enter your password"
                />
                <button
                  type="button"
                  onClick={() => setShowPw(!showPw)}
                  className="absolute right-3 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground transition-colors"
                >
                  {showPw ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
                </button>
              </div>
            </div>
            <Button
              type="submit"
              className="w-full shadow-sm text-[10px] font-bold uppercase tracking-widest btn-premium glow-primary h-10 rounded-none"
              disabled={loading}
            >
              {loading ? 'Signing in...' : 'Sign In'}
            </Button>
          </form>
        </CardContent>
      </Card>
    </div>
  )
}
