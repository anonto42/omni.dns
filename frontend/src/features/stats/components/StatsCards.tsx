import { useState, useCallback } from 'react'
import { Activity, Ban, Percent, Database } from 'lucide-react'
import { getStatus, type Status } from '../api'
import { usePolling } from '../../../hooks/usePolling'
import { useWindowFocus } from '../../../hooks/useWindowFocus'
import { Card, CardContent } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'

interface StatCardProps {
  label: string
  value: string
  sub: string
  icon: React.ElementType
  barColor: string      // explicit tailwind bg class e.g. "bg-primary"
  iconBg: string        // e.g. "bg-primary/10 text-primary"
  loading?: boolean
  bar: number           // 0-100
}

const StatCard: React.FC<StatCardProps> = ({ label, value, sub, icon: Icon, barColor, iconBg, loading, bar }) => {
  if (loading) {
    return (
      <Card className="shadow-sm overflow-hidden">
        <CardContent className="p-6 space-y-4">
          <div className="flex items-center justify-between">
            <Skeleton className="h-3 w-24" />
            <Skeleton className="h-8 w-8" />
          </div>
          <Skeleton className="h-8 w-20" />
          <Skeleton className="h-1 w-full" />
          <Skeleton className="h-3 w-36" />
        </CardContent>
      </Card>
    )
  }

  return (
    <Card className="shadow-sm overflow-hidden">
      <CardContent className="p-6">
        <div className="flex items-center justify-between mb-3">
          <p className="text-[10px] font-bold text-muted-foreground uppercase tracking-widest">{label}</p>
          <div className={`p-2 ${iconBg}`}>
            <Icon className="h-4 w-4" />
          </div>
        </div>
        <h3 className="text-3xl font-bold tracking-tight text-foreground mb-3 tabular-nums">{value}</h3>
        <div className="h-1 bg-muted mb-3 overflow-hidden">
          <div
            className={`h-full ${barColor} transition-all duration-700 ease-out`}
            style={{ width: `${Math.max(bar > 0 ? 2 : 0, bar)}%` }}
          />
        </div>
        <p className="text-[10px] font-bold text-muted-foreground uppercase tracking-widest">{sub}</p>
      </CardContent>
    </Card>
  )
}

export default function StatsCards() {
  const [stats, setStats] = useState<Status | null>(null)
  const [loading, setLoading] = useState(true)

  const fetchStats = useCallback(async () => {
    try {
      const data = await getStatus()
      if (data) { setStats(data); setLoading(false) }
    } catch {
      setLoading(false)
    }
  }, [])

  usePolling(fetchStats, 3000)
  useWindowFocus(fetchStats)

  const qf = stats?.queries_forwarded ?? 0
  const qb = stats?.queries_blocked ?? 0
  const qc = stats?.queries_cached ?? 0
  const cs = stats?.cache_size ?? 0
  const ch = stats?.cache_hits ?? 0
  const cm = stats?.cache_misses ?? 0
  const total = qf + qb + qc

  const percentBlocked  = total > 0 ? (qb / total) * 100 : 0
  const percentAllowed  = total > 0 ? (qf / total) * 100 : 0
  const cacheHitRate    = (ch + cm) > 0 ? (ch / (ch + cm)) * 100 : 0

  const fmt = (n: number) => n.toLocaleString()
  const pct = (n: number) => `${Math.round(n)}%`

  return (
    <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4 mb-8">
      <StatCard
        loading={loading}
        label="Total Queries"
        value={fmt(total)}
        sub={`${fmt(qf)} allowed · ${fmt(qc)} cached`}
        icon={Activity}
        iconBg="bg-primary/10 text-primary"
        barColor="bg-primary"
        bar={total > 0 ? 100 : 0}
      />
      <StatCard
        loading={loading}
        label="Queries Blocked"
        value={fmt(qb)}
        sub={total > 0 ? `${pct(percentBlocked)} of all queries` : 'no queries yet'}
        icon={Ban}
        iconBg="bg-rose-500/10 text-rose-500"
        barColor="bg-rose-500"
        bar={percentBlocked}
      />
      <StatCard
        loading={loading}
        label="Percent Blocked"
        value={pct(percentBlocked)}
        sub={total > 0 ? `${fmt(total)} total · ${pct(percentAllowed)} allowed` : 'no queries yet'}
        icon={Percent}
        iconBg="bg-amber-500/10 text-amber-500"
        barColor="bg-amber-500"
        bar={percentBlocked}
      />
      <StatCard
        loading={loading}
        label="Cache Size"
        value={fmt(cs)}
        sub={`${pct(cacheHitRate)} hit rate · ${fmt(ch)} hits`}
        icon={Database}
        iconBg="bg-emerald-500/10 text-emerald-500"
        barColor="bg-emerald-500"
        bar={cacheHitRate}
      />
    </div>
  )
}
