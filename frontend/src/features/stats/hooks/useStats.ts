import { type Status } from '../api'
import { useWindowFocus } from '../../../hooks/useWindowFocus'
import { useSharedStatus } from './useSharedStatus'

export interface StatsViewModel {
  raw: Status | null
  loading: boolean
  // derived totals
  total: number
  qf: number
  qb: number
  qc: number
  cs: number
  ch: number
  cm: number
  percentBlocked: number
  percentAllowed: number
  cacheHitRate: number
  blockRate: number
  uptime: number
}

export function useStats(): StatsViewModel {
  const { status: raw, loading, refresh } = useSharedStatus()

  useWindowFocus(refresh)

  const qf = raw?.queries_forwarded ?? 0
  const qb = raw?.queries_blocked ?? 0
  const qc = raw?.queries_cached ?? 0
  const cs = raw?.cache_size ?? 0
  const ch = raw?.cache_hits ?? 0
  const cm = raw?.cache_misses ?? 0
  const uptime = raw?.uptime_seconds ?? 0
  const total = qf + qb + qc

  const percentBlocked = total > 0 ? (qb / total) * 100 : 0
  const percentAllowed = total > 0 ? (qf / total) * 100 : 0
  const cacheHitRate   = (ch + cm) > 0 ? (ch / (ch + cm)) * 100 : 0
  const blockRate      = total > 0 ? Math.round((qb / total) * 100) : 0

  return { raw, loading, total, qf, qb, qc, cs, ch, cm, percentBlocked, percentAllowed, cacheHitRate, blockRate, uptime }
}
