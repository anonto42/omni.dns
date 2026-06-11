import React, { createContext, useContext, useState, useCallback, useEffect } from 'react'

interface AuthContextValue {
  token: string | null
  user: { email: string; name: string } | null
  login: (email: string, password: string) => Promise<void>
  logout: () => void
  isAuthenticated: boolean
  updateProfile: (name: string, email: string) => Promise<void>
}

const AuthContext = createContext<AuthContextValue | null>(null)

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const [token, setToken] = useState<string | null>(() => localStorage.getItem('auth_token'))
  const [user, setUser] = useState<{ email: string; name: string } | null>(null)

  const fetchProfile = useCallback(async (authToken: string) => {
    try {
      const res = await fetch('/api/profile', {
        headers: { 'Authorization': `Bearer ${authToken}` }
      })
      if (res.ok) {
        const data = await res.json()
        setUser(data)
      } else {
        localStorage.removeItem('auth_token')
        setToken(null)
        setUser(null)
      }
    } catch {
      // Fail silent on network error
    }
  }, [])

  const login = useCallback(async (email: string, password: string) => {
    const res = await fetch('/api/login', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ email, password }),
    })
    if (!res.ok) {
      const err = await res.json().catch(() => ({ error: 'login failed' }))
      throw new Error(err.error || 'login failed')
    }
    const data = await res.json()
    localStorage.setItem('auth_token', data.token)
    setToken(data.token)
  }, [])

  const logout = useCallback(() => {
    localStorage.removeItem('auth_token')
    setToken(null)
    setUser(null)
  }, [])

  const updateProfile = useCallback(async (name: string, email: string) => {
    if (!token) return
    const res = await fetch('/api/profile', {
      method: 'PUT',
      headers: {
        'Content-Type': 'application/json',
        'Authorization': `Bearer ${token}`
      },
      body: JSON.stringify({ name, email }),
    })
    if (!res.ok) {
      const err = await res.json().catch(() => ({ error: 'update profile failed' }))
      throw new Error(err.error || 'update profile failed')
    }
    // Reload profile details
    await fetchProfile(token)
  }, [token, fetchProfile])

  useEffect(() => {
    const stored = localStorage.getItem('auth_token')
    if (stored) {
      setToken(stored)
      fetchProfile(stored)
    }
  }, [fetchProfile])

  return (
    <AuthContext.Provider value={{ token, user, login, logout, isAuthenticated: !!token, updateProfile }}>
      {children}
    </AuthContext.Provider>
  )
}

export function useAuth() {
  const ctx = useContext(AuthContext)
  if (!ctx) throw new Error('useAuth must be used within AuthProvider')
  return ctx
}
