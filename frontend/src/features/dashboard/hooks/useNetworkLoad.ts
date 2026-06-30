import { useEffect, useState, useMemo } from 'react'
import { useSharedStatus } from '@/features/stats/hooks/useSharedStatus'

// ── Types ────────────────────────────────────────────────────────────────

export type ChartSample = {
  time: string
  total: number
  blocked: number
  cached: number
  allowed: number
}

type Snapshot = {
  forwarded: number
  blocked: number
  cached: number
}

type PieEntry = {
  name: string
  value: number
  pct: number
  color: string
}

// ── Constants ────────────────────────────────────────────────────────────

export const CHART_POINTS = 30

export const TOOLTIP_STYLE = {
  background: 'var(--card)',
  border: 'none',
  borderRadius: 0,
  fontSize: 11,
  fontFamily: 'monospace',
  boxShadow: '0 4px 24px rgba(0,0,0,0.3)',
} as const

// ── Hook ─────────────────────────────────────────────────────────────────

export function useNetworkLoad() {
  const { status, loading: statusLoading } = useSharedStatus()
  const [data, setData] = useState<ChartSample[]>(() =>
    Array(CHART_POINTS).fill(null).map(() => ({ time: '', total: 0, blocked: 0, cached: 0, allowed: 0 }))
  )
  // Holds the last raw cumulative counts so we can diff
  const prevRef = useMemo<{ current: Snapshot | null }>(() => ({ current: null }), [])
  // Cumulative totals from API (for the analytics pills)
  const [cumulative, setCumulative] = useState<Snapshot>({ forwarded: 0, blocked: 0, cached: 0 })
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    if (!status) {
      if (!statusLoading) setLoading(false)
      return
    }

    const fwd  = status.queries_forwarded ?? 0
    const blk  = status.queries_blocked   ?? 0
    const cach = status.queries_cached    ?? 0

    setCumulative({ forwarded: fwd, blocked: blk, cached: cach })

    // Compute delta vs last snapshot
    const prev = prevRef.current
    const dFwd  = prev ? Math.max(0, fwd  - prev.forwarded) : 0
    const dBlk  = prev ? Math.max(0, blk  - prev.blocked)   : 0
    const dCach = prev ? Math.max(0, cach - prev.cached)    : 0
    prevRef.current = { forwarded: fwd, blocked: blk, cached: cach }

    const time = new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit', hour12: false })
    setData(prev => [...prev.slice(1), {
      time,
      total:   dFwd + dBlk + dCach,
      allowed: dFwd,
      blocked: dBlk,
      cached:  dCach,
    }])
    setLoading(false)
  }, [prevRef, status, statusLoading])

  const cumTotal = cumulative.forwarded + cumulative.blocked + cumulative.cached
  const cumPctBlocked  = cumTotal > 0 ? Math.round((cumulative.blocked  / cumTotal) * 100) : 0
  const cumPctAllowed  = cumTotal > 0 ? Math.round((cumulative.forwarded / cumTotal) * 100) : 0
  const cumPctCached   = cumTotal > 0 ? Math.round((cumulative.cached   / cumTotal) * 100) : 0

  const pieData = useMemo<PieEntry[]>(() => [
    { name: 'Allowed', value: cumulative.forwarded, pct: cumPctAllowed,  color: '#22c55e' },
    { name: 'Blocked', value: cumulative.blocked,   pct: cumPctBlocked,  color: '#f43f5e' },
    { name: 'Cached',  value: cumulative.cached,    pct: cumPctCached,   color: 'var(--primary)' },
  ].filter(d => d.value > 0), [cumulative, cumPctAllowed, cumPctBlocked, cumPctCached])

  const hasData = cumTotal > 0

  return {
    data,
    loading,
    cumulative,
    cumTotal,
    cumPctBlocked,
    cumPctAllowed,
    pieData,
    hasData,
  }
}
