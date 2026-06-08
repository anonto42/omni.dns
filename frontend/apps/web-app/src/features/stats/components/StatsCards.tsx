import { useState } from 'react'
import { getStatus, type Status } from '../api'
import { usePolling } from '../../../hooks/usePolling'

function fmtUptime(seconds: number): string {
  const d = Math.floor(seconds / 86400)
  const h = Math.floor((seconds % 86400) / 3600)
  const m = Math.floor((seconds % 3600) / 60)
  if (d > 0) return `${d}d ${h}h`
  if (h > 0) return `${h}h ${m}m`
  return `${m}m`
}

export default function StatsCards() {
  const [stats, setStats] = useState<Status>({
    queries_forwarded: 0,
    queries_blocked: 0,
    queries_custom: 0,
    queries_cached: 0,
    cache_size: 0,
    cache_hits: 0,
    cache_misses: 0,
    uptime_seconds: 0,
  })

  usePolling(async () => {
    try {
      setStats(await getStatus())
    } catch {}
  }, 3000)

  const total = stats.cache_hits + stats.cache_misses
  const hitRate = total > 0 ? Math.round((stats.cache_hits / total) * 100) : 0

  const cards = [
    { label: 'Forwarded', value: stats.queries_forwarded, color: 'text-blue-400' },
    { label: 'Blocked', value: stats.queries_blocked, color: 'text-red-400' },
    { label: 'Custom', value: stats.queries_custom, color: 'text-yellow-400' },
    { label: 'Cached', value: stats.queries_cached, color: 'text-green-400' },
    { label: 'Cache Size', value: stats.cache_size, color: 'text-purple-400' },
    { label: 'Hit Rate', value: `${hitRate}%`, color: 'text-cyan-400' },
    { label: 'Uptime', value: fmtUptime(stats.uptime_seconds), color: 'text-gray-400' },
  ]

  return (
    <div className="grid grid-cols-2 md:grid-cols-4 lg:grid-cols-7 gap-4 mb-6">
      {cards.map((c) => (
        <div key={c.label} className="bg-gray-900 rounded-xl p-4 border border-gray-800">
          <div className="text-xs text-gray-500 uppercase tracking-wide">{c.label}</div>
          <div className={`text-2xl font-bold mt-1 ${c.color}`}>{c.value}</div>
        </div>
      ))}
    </div>
  )
}
