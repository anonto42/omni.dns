import { apiGet } from '@/hooks/api'
import { toast } from 'sonner'

// ── Types ────────────────────────────────────────────────────────────────

export type TimeRange =
  | { mode: 'live' }
  | { mode: 'preset'; hours: number; label: string }
  | { mode: 'custom'; from: string; to: string }

// ── Helpers ──────────────────────────────────────────────────────────────

export function rangeLabel(r: TimeRange): string {
  if (r.mode === 'live') return 'Live'
  if (r.mode === 'preset') return r.label
  if (r.from && r.to) return `${r.from.slice(0, 16)} → ${r.to.slice(0, 16)}`
  return 'Custom Range'
}

export function rangeCutoff(r: TimeRange): { from?: string; to?: string } {
  if (r.mode === 'live') return {}
  if (r.mode === 'preset') return { from: new Date(Date.now() - r.hours * 3_600_000).toISOString() }
  return {
    from: r.from ? new Date(r.from).toISOString() : undefined,
    to: r.to ? new Date(r.to).toISOString() : undefined,
  }
}

// ── CSV Export ───────────────────────────────────────────────────────────

type LogEntry = {
  id: number
  timestamp: string
  domain: string
  client_ip: string
  action: string
}

export async function exportLogsCSV(range: TimeRange) {
  const { from, to } = rangeCutoff(range)
  const logs = await apiGet<LogEntry[]>('/logs?limit=10000')
  const filtered = logs.filter(l => {
    if (from && l.timestamp < from) return false
    if (to   && l.timestamp > to)   return false
    return true
  })
  if (filtered.length === 0) {
    toast.info('No logs to export', { description: `No queries found for the selected range.` })
    return
  }
  const header = 'id,timestamp,domain,client_ip,action'
  const rows = filtered.map(l => `${l.id},${l.timestamp},${l.domain},${l.client_ip},${l.action}`)
  const csv = [header, ...rows].join('\n')
  const blob = new Blob([csv], { type: 'text/csv' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = `netshield-logs-${new Date().toISOString().slice(0, 10)}.csv`
  a.click()
  URL.revokeObjectURL(url)
  toast.success('Report exported', { description: `${filtered.length} entries downloaded.` })
}
