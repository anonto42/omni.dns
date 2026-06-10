import React from 'react';
import { Timer, Database, CheckCircle2, Cpu } from 'lucide-react';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Skeleton } from '@/components/ui/skeleton';
import { getStatus, type Status } from '../api';
import { usePolling } from '../../../hooks/usePolling';
import { useCallback, useState } from 'react';

function formatUptime(seconds: number): string {
  if (seconds < 60) return `${Math.floor(seconds)}s`
  const d = Math.floor(seconds / 86400)
  const h = Math.floor((seconds % 86400) / 3600)
  const m = Math.floor((seconds % 3600) / 60)
  if (d > 0) return `${d}d ${h.toString().padStart(2, '0')}h ${m.toString().padStart(2, '0')}m`
  if (h > 0) return `${h}h ${m.toString().padStart(2, '0')}m`
  return `${m}m`
}

export const SystemHealth: React.FC = () => {
  const [status, setStatus] = useState<Status | null>(null)
  const [loading, setLoading] = useState(true)
  const [lastChecked, setLastChecked] = useState<Date | null>(null)

  const fetch = useCallback(async () => {
    try {
      const data = await getStatus()
      setStatus(data)
      setLastChecked(new Date())
    } catch {
      // keep showing last known values
    } finally {
      setLoading(false)
    }
  }, [])

  usePolling(fetch, 10000)

  const uptime = status?.uptime_seconds ?? 0
  const cacheHitRate = (status?.cache_hits ?? 0) + (status?.cache_misses ?? 0) > 0
    ? Math.round(((status?.cache_hits ?? 0) / ((status?.cache_hits ?? 0) + (status?.cache_misses ?? 0))) * 100)
    : 0
  const totalQueries = (status?.queries_forwarded ?? 0) + (status?.queries_blocked ?? 0) + (status?.queries_cached ?? 0)
  const blockRate = totalQueries > 0
    ? Math.round(((status?.queries_blocked ?? 0) / totalQueries) * 100)
    : 0

  const items = [
    {
      label: 'Uptime',
      value: loading ? null : formatUptime(uptime),
      color: 'bg-emerald-500',
      percent: Math.min(100, Math.round((uptime / 86400) * 10)),
      icon: Timer,
    },
    {
      label: 'Cache Hit Rate',
      value: loading ? null : `${cacheHitRate}%`,
      color: 'bg-primary',
      percent: cacheHitRate,
      icon: Database,
    },
    {
      label: 'Block Rate',
      value: loading ? null : `${blockRate}%`,
      color: 'bg-rose-500',
      percent: blockRate,
      icon: Cpu,
    },
  ]

  const lastCheckedLabel = lastChecked
    ? `Last checked: ${lastChecked.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' })}`
    : 'Checking…'

  return (
    <Card className="shadow-sm h-full flex flex-col">
      <CardHeader className="pb-2">
        <CardTitle className="text-lg font-bold tracking-tight text-foreground mt-2 ml-2">System Health</CardTitle>
      </CardHeader>
      <CardContent className="space-y-6 p-6">
        {items.map((item) => (
          <div key={item.label}>
            <div className="flex justify-between items-center mb-2">
              <span className="text-[10px] font-bold text-muted-foreground uppercase tracking-widest flex items-center gap-2">
                <item.icon className="h-3.5 w-3.5 text-muted-foreground/85" />
                {item.label}
              </span>
              {item.value === null
                ? <Skeleton className="h-4 w-16" />
                : <span className="text-sm font-bold text-foreground">{item.value}</span>
              }
            </div>
            <div className="h-2 bg-muted/60 overflow-hidden shadow-inner">
              {item.value === null
                ? <Skeleton className="h-full w-full" />
                : <div
                    className={`h-full ${item.color} transition-all duration-500 ease-out`}
                    style={{ width: `${Math.max(2, item.percent)}%` }}
                  />
              }
            </div>
          </div>
        ))}

        <div className="mt-6 p-4 bg-muted/20 flex items-center gap-4">
          <div className="w-10 h-10 bg-emerald-500/10 flex items-center justify-center text-emerald-500 shadow-sm">
            <CheckCircle2 className="h-5 w-5" />
          </div>
          <div>
            <p className="text-sm font-bold text-foreground">All Systems Nominal</p>
            <p className="text-[10px] text-muted-foreground uppercase font-bold tracking-wider">{lastCheckedLabel}</p>
          </div>
        </div>
      </CardContent>
    </Card>
  );
};
