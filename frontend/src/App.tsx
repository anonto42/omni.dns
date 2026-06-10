import { useState, useEffect, useCallback, useMemo } from 'react'
import {
  AreaChart, Area, XAxis, YAxis, CartesianGrid, Tooltip as RechartsTooltip,
  ResponsiveContainer, BarChart, Bar, PieChart, Pie, Cell, Legend,
} from 'recharts'
import { Routes, Route, Navigate, useLocation } from 'react-router-dom'
import { useAuth } from './hooks/useAuth'
import { useTheme, type Theme } from './hooks/useTheme'
import LoginPage from './pages/LoginPage'
import { apiGet, apiPost, apiPut, apiDelete } from './hooks/api'
import {
  Calendar,
  Download,
  BarChart3,
  CheckCircle2,
  PlusCircle,
  Trash2,
  Gauge,
  Globe,
  Power,
  Sun,
  Moon,
  Monitor,
} from 'lucide-react'

import { toast } from 'sonner'
import { Toaster } from 'sonner'
import { DashboardLayout } from './components/layout/DashboardLayout'
import { StatsCards } from './features/stats'
import { SystemHealth } from './features/stats/components/SystemHealth'
import { LogTable } from './features/logs'
import { RecordManager } from './features/records'
import { BlocklistManager } from './features/blocklist'
import { getSettings, saveSettings } from './features/settings/api'
import { getStatus } from './features/stats/api'
import { usePolling } from './hooks/usePolling'
import { ConfirmDialog } from './components/ui/confirm-dialog'
import { Skeleton } from './components/ui/skeleton'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Switch } from '@/components/ui/switch'
import { Input } from '@/components/ui/input'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'

async function exportLogsCSV(last24h: boolean) {
  const cutoff = last24h ? new Date(Date.now() - 86400_000).toISOString() : undefined
  const logs = await apiGet<Array<{ id: number; timestamp: string; domain: string; client_ip: string; action: string }>>('/logs?limit=5000')
  const filtered = cutoff ? logs.filter(l => l.timestamp >= cutoff) : logs
  if (filtered.length === 0) {
    toast.info('No logs to export', { description: last24h ? 'No queries in the last 24 hours.' : 'No query logs found.' })
    return
  }
  const header = 'id,timestamp,domain,client_ip,action'
  const rows = filtered.map(l => `${l.id},${l.timestamp},${l.domain},${l.client_ip},${l.action}`)
  const csv = [header, ...rows].join('\n')
  const blob = new Blob([csv], { type: 'text/csv' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = `netshield-logs-${new Date().toISOString().slice(0, 10)}${last24h ? '-24h' : ''}.csv`
  a.click()
  URL.revokeObjectURL(url)
  toast.success('Report exported', { description: `${filtered.length} entries downloaded.` })
}

// Page wrapper with enter animation
function PageTransition({ children }: { children: React.ReactNode }) {
  return (
    <div className="animate-in fade-in slide-in-from-bottom-3 duration-300">
      {children}
    </div>
  )
}

const CHART_POINTS = 30
const TOOLTIP_STYLE = {
  background: 'hsl(var(--card))',
  border: 'none',
  borderRadius: 0,
  fontSize: 11,
  fontFamily: 'monospace',
  boxShadow: '0 4px 24px rgba(0,0,0,0.3)',
}

// Each sample stores the DELTA (new queries since last poll), not cumulative totals.
type ChartSample = { time: string; total: number; blocked: number; cached: number; allowed: number }
// Snapshot of raw cumulative counts from the API — used to compute deltas.
type Snapshot = { forwarded: number; blocked: number; cached: number }

function NetworkLoadChart() {
  const [data, setData] = useState<ChartSample[]>(() =>
    Array(CHART_POINTS).fill(null).map(() => ({ time: '', total: 0, blocked: 0, cached: 0, allowed: 0 }))
  )
  // Holds the last raw cumulative counts so we can diff
  const prevRef = useMemo<{ current: Snapshot | null }>(() => ({ current: null }), [])
  // Cumulative totals from API (for the analytics pills)
  const [cumulative, setCumulative] = useState<Snapshot>({ forwarded: 0, blocked: 0, cached: 0 })
  const [loading, setLoading] = useState(true)

  const fetchFn = useCallback(async () => {
    try {
      const s = await getStatus()
      const fwd  = s.queries_forwarded ?? 0
      const blk  = s.queries_blocked   ?? 0
      const cach = s.queries_cached    ?? 0

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
    } catch { setLoading(false) }
  }, [prevRef])

  usePolling(fetchFn, 3000)

  const cumTotal = cumulative.forwarded + cumulative.blocked + cumulative.cached
  const cumPctBlocked  = cumTotal > 0 ? Math.round((cumulative.blocked  / cumTotal) * 100) : 0
  const cumPctAllowed  = cumTotal > 0 ? Math.round((cumulative.forwarded / cumTotal) * 100) : 0
  const cumPctCached   = cumTotal > 0 ? Math.round((cumulative.cached   / cumTotal) * 100) : 0

  const pieData = useMemo(() => [
    { name: 'Allowed', value: cumulative.forwarded, pct: cumPctAllowed,  color: '#22c55e' },
    { name: 'Blocked', value: cumulative.blocked,   pct: cumPctBlocked,  color: '#f43f5e' },
    { name: 'Cached',  value: cumulative.cached,    pct: cumPctCached,   color: 'hsl(var(--primary))' },
  ].filter(d => d.value > 0), [cumulative, cumPctAllowed, cumPctBlocked, cumPctCached])

  const hasData = cumTotal > 0

  return (
    <Card className="overflow-hidden shadow-sm">
      <CardHeader className="pb-0 pt-4 px-4">
        <div className="flex flex-wrap items-start justify-between gap-4">
          {/* Title */}
          <div>
            <CardTitle className="text-lg flex items-center gap-2 font-bold tracking-tight text-foreground">
              <BarChart3 className="h-5 w-5 text-primary" />
              Network Load
            </CardTitle>
            <CardDescription className="text-[10px] font-bold uppercase tracking-widest mt-0.5">
              Live query rate · updates every 3s
            </CardDescription>
          </div>

          {/* Analytics pills */}
          <div className="flex flex-wrap gap-2">
            {[
              { label: 'Total Queries',    value: cumTotal.toLocaleString(),          color: 'text-foreground',                   bg: 'bg-muted/50' },
              { label: 'Blocked',          value: cumulative.blocked.toLocaleString(), color: 'text-rose-500',                     bg: 'bg-rose-500/10' },
              { label: '% Blocked',        value: `${cumPctBlocked}%`,                color: 'text-rose-500',                     bg: 'bg-rose-500/10' },
              { label: '% Allowed',        value: `${cumPctAllowed}%`,                color: 'text-emerald-600 dark:text-emerald-400', bg: 'bg-emerald-500/10' },
            ].map(pill => (
              <div key={pill.label} className={`flex flex-col items-center px-3 py-1.5 ${pill.bg}`}>
                <span className={`text-sm font-bold tabular-nums ${pill.color}`}>{loading ? '—' : pill.value}</span>
                <span className="text-[9px] font-bold uppercase tracking-widest text-muted-foreground mt-0.5">{pill.label}</span>
              </div>
            ))}
          </div>
        </div>
      </CardHeader>

      <CardContent className="pb-4 px-2 pt-2">
        {loading ? (
          <Skeleton className="w-full h-[200px]" />
        ) : (
          <div className="grid grid-cols-1 lg:grid-cols-3 gap-0">
            {/* Area chart — 2/3 width */}
            <div className="lg:col-span-2">
              <ResponsiveContainer width="100%" height={200}>
                <AreaChart data={data} margin={{ top: 8, right: 8, left: -20, bottom: 0 }}>
                  <defs>
                    <linearGradient id="gradAllowed" x1="0" y1="0" x2="0" y2="1">
                      <stop offset="5%"  stopColor="#22c55e" stopOpacity={0.2} />
                      <stop offset="95%" stopColor="#22c55e" stopOpacity={0} />
                    </linearGradient>
                    <linearGradient id="gradBlocked" x1="0" y1="0" x2="0" y2="1">
                      <stop offset="5%"  stopColor="#f43f5e" stopOpacity={0.2} />
                      <stop offset="95%" stopColor="#f43f5e" stopOpacity={0} />
                    </linearGradient>
                    <linearGradient id="gradCached" x1="0" y1="0" x2="0" y2="1">
                      <stop offset="5%"  stopColor="hsl(var(--primary))" stopOpacity={0.25} />
                      <stop offset="95%" stopColor="hsl(var(--primary))" stopOpacity={0} />
                    </linearGradient>
                  </defs>
                  <CartesianGrid strokeDasharray="3 3" stroke="currentColor" strokeOpacity={0.06} vertical={false} />
                  <XAxis
                    dataKey="time"
                    tick={{ fontSize: 9, fontFamily: 'monospace', fill: 'currentColor', opacity: 0.4 }}
                    tickLine={false}
                    axisLine={false}
                    interval={Math.floor(CHART_POINTS / 5)}
                  />
                  <YAxis
                    tick={{ fontSize: 9, fontFamily: 'monospace', fill: 'currentColor', opacity: 0.4 }}
                    tickLine={false}
                    axisLine={false}
                    allowDecimals={false}
                    width={30}
                    label={{ value: 'req/3s', angle: -90, position: 'insideLeft', offset: 16, style: { fontSize: 8, fill: 'currentColor', opacity: 0.3, fontFamily: 'monospace' } }}
                  />
                  <RechartsTooltip
                    contentStyle={TOOLTIP_STYLE}
                    labelStyle={{ color: 'hsl(var(--muted-foreground))', fontWeight: 700, textTransform: 'uppercase', letterSpacing: '0.05em', fontSize: 9 }}
                    itemStyle={{ color: 'hsl(var(--foreground))' }}
                    cursor={{ stroke: 'hsl(var(--primary))', strokeWidth: 1, strokeDasharray: '4 2' }}
                    formatter={(value: number, name: string) => [`${value} req`, name]}
                  />
                  <Area type="monotone" dataKey="allowed" name="Allowed" stroke="#22c55e" strokeWidth={1.5} fill="url(#gradAllowed)" dot={false} activeDot={{ r: 3, strokeWidth: 0 }} />
                  <Area type="monotone" dataKey="blocked" name="Blocked" stroke="#f43f5e" strokeWidth={1.5} fill="url(#gradBlocked)" dot={false} activeDot={{ r: 3, strokeWidth: 0 }} />
                  <Area type="monotone" dataKey="cached"  name="Cached"  stroke="hsl(var(--primary))" strokeWidth={1.5} fill="url(#gradCached)" dot={false} activeDot={{ r: 3, strokeWidth: 0 }} />
                </AreaChart>
              </ResponsiveContainer>
            </div>

            {/* Donut + legend — 1/3 width */}
            <div className="flex flex-col items-center justify-center px-2 py-2">
              {hasData ? (
                <>
                  <ResponsiveContainer width="100%" height={140}>
                    <PieChart>
                      <Pie
                        data={pieData}
                        cx="50%"
                        cy="50%"
                        innerRadius={42}
                        outerRadius={60}
                        paddingAngle={2}
                        dataKey="value"
                        strokeWidth={0}
                      >
                        {pieData.map((entry, i) => (
                          <Cell key={i} fill={entry.color} />
                        ))}
                      </Pie>
                      <RechartsTooltip
                        contentStyle={TOOLTIP_STYLE}
                        itemStyle={{ color: 'hsl(var(--foreground))' }}
                        formatter={(value: number, name: string) => [value.toLocaleString(), name]}
                      />
                    </PieChart>
                  </ResponsiveContainer>
                  <div className="w-full space-y-1.5 px-1">
                    {pieData.map(d => (
                      <div key={d.name} className="flex items-center justify-between gap-2">
                        <div className="flex items-center gap-1.5 min-w-0">
                          <div className="w-2 h-2 shrink-0" style={{ background: d.color }} />
                          <span className="text-[10px] font-bold uppercase tracking-widest text-muted-foreground truncate">{d.name}</span>
                        </div>
                        <div className="flex items-center gap-1.5 shrink-0">
                          <span className="text-[10px] font-bold text-foreground tabular-nums">{d.value.toLocaleString()}</span>
                          <span className="text-[9px] text-muted-foreground tabular-nums">({d.pct}%)</span>
                        </div>
                      </div>
                    ))}
                  </div>
                </>
              ) : (
                <div className="flex flex-col items-center gap-2 py-10">
                  <div className="w-16 h-16 bg-muted/30 flex items-center justify-center">
                    <BarChart3 className="h-6 w-6 text-muted-foreground/30" />
                  </div>
                  <p className="text-[10px] font-bold uppercase tracking-widest text-muted-foreground text-center">No traffic yet</p>
                  <p className="text-[9px] text-muted-foreground/60 text-center">Make a DNS query to see data</p>
                </div>
              )}
            </div>
          </div>
        )}
      </CardContent>
    </Card>
  )
}

const Dashboard = () => {
  const [last24h, setLast24h] = useState(false)
  const [exporting, setExporting] = useState(false)

  const handleExport = async () => {
    setExporting(true)
    try { await exportLogsCSV(last24h) } finally { setExporting(false) }
  }

  return (
    <PageTransition>
      <div className="space-y-8">
        <div className="flex flex-col md:flex-row md:items-center justify-between gap-4">
          <div className="space-y-1">
            <h2 className="text-2xl font-bold tracking-tight text-foreground">Network Overview</h2>
            <p className="text-muted-foreground text-[10px] font-bold uppercase tracking-widest">Real-time monitoring for your DNS server.</p>
          </div>
          <div className="flex items-center gap-2">
            <Button
              variant={last24h ? 'default' : 'outline'}
              size="sm"
              className="gap-2 text-[10px] font-bold uppercase tracking-widest shadow-sm"
              onClick={() => setLast24h(v => !v)}
            >
              <Calendar className="h-3.5 w-3.5" />
              {last24h ? 'Last 24 Hours ✓' : 'Last 24 Hours'}
            </Button>
            <Button
              size="sm"
              className="gap-2 shadow-sm text-[10px] font-bold uppercase tracking-widest"
              onClick={handleExport}
              disabled={exporting}
            >
              <Download className="h-3.5 w-3.5" />
              {exporting ? 'Exporting…' : 'Export Report'}
            </Button>
          </div>
        </div>

        <StatsCards />

        <NetworkLoadChart />

        <div className="grid grid-cols-1 lg:grid-cols-12 gap-8 items-stretch">
          <div className="lg:col-span-8 flex flex-col">
            <LogTable compact />
          </div>
          <div className="lg:col-span-4 flex flex-col">
            <SystemHealth />
          </div>
        </div>
      </div>
    </PageTransition>
  )
}

const LogsPage = () => (
  <PageTransition>
    <div className="space-y-6">
      <div className="space-y-1">
        <h1 className="text-2xl font-bold tracking-tight text-foreground">Query Log</h1>
        <p className="text-muted-foreground text-[10px] font-bold uppercase tracking-widest">Real-time DNS traffic and security events across your organization.</p>
      </div>
      <LogTable />
    </div>
  </PageTransition>
)

type SteeringRule = {
  id: number
  name: string
  condition_type: string
  condition_value: string
  action_type: string
  action_target: string
  priority: number
  enabled: boolean
}

const SteeringPage = () => {
  const [rules, setRules] = useState<SteeringRule[]>([])
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [deleteTarget, setDeleteTarget] = useState<number | null>(null)
  const [name, setName] = useState('')
  const [conditionType, setConditionType] = useState('Domain')
  const [conditionValue, setConditionValue] = useState('')
  const [actionType, setActionType] = useState('Forward')
  const [actionValue, setActionValue] = useState('')
  const [priority, setPriority] = useState(1)

  useEffect(() => {
    apiGet<SteeringRule[]>('/steering')
      .then(data => { setRules(data ?? []); setLoading(false) })
      .catch(() => { toast.error('Failed to load steering rules'); setLoading(false) })
  }, [])

  const handleAdd = async () => {
    if (!name.trim() || !conditionValue.trim()) {
      toast.warning('Fill in rule name and condition value')
      return
    }
    setSaving(true)
    try {
      const res = await apiPost('/steering', {
        name: name.trim(),
        condition_type: conditionType,
        condition_value: conditionValue.trim(),
        action_type: actionType,
        action_target: actionType === 'Block' ? '' : actionValue,
        priority,
        enabled: true,
      }) as { id: number }
      const newRule: SteeringRule = {
        id: res.id,
        name: name.trim(),
        condition_type: conditionType,
        condition_value: conditionValue.trim(),
        action_type: actionType,
        action_target: actionType === 'Block' ? '' : actionValue,
        priority,
        enabled: true,
      }
      setRules(prev => [...prev, newRule].sort((a, b) => a.priority - b.priority))
      setName(''); setConditionValue(''); setActionValue('')
      toast.success('Rule added', { description: name.trim() })
    } catch {
      toast.error('Failed to add rule')
    } finally {
      setSaving(false)
    }
  }

  const toggleRule = async (rule: SteeringRule) => {
    try {
      await apiPut('/steering', { id: rule.id, enabled: !rule.enabled })
      setRules(prev => prev.map(r => r.id === rule.id ? { ...r, enabled: !r.enabled } : r))
      toast.success(rule.enabled ? 'Rule disabled' : 'Rule enabled', { description: rule.name })
    } catch {
      toast.error('Failed to update rule')
    }
  }

  const deleteRule = async (id: number) => {
    try {
      await apiDelete('/steering', { id })
      setRules(prev => prev.filter(r => r.id !== id))
      toast.success('Rule deleted')
    } catch {
      toast.error('Failed to delete rule')
    }
  }

  const activeCount = rules.filter(r => r.enabled).length

  const renderSkeletonRows = () =>
    Array.from({ length: 3 }).map((_, i) => (
      <TableRow key={i}>
        <TableCell><Skeleton className="h-3 w-6" /></TableCell>
        <TableCell><Skeleton className="h-3 w-32" /></TableCell>
        <TableCell><Skeleton className="h-5 w-40 rounded" /></TableCell>
        <TableCell className="hidden md:table-cell"><Skeleton className="h-5 w-20 rounded-full" /></TableCell>
        <TableCell><Skeleton className="h-5 w-10 rounded-full" /></TableCell>
        <TableCell />
      </TableRow>
    ))

  return (
    <PageTransition>
      <ConfirmDialog
        open={deleteTarget !== null}
        title="Delete steering rule?"
        description="This rule will be permanently removed and no longer applied to DNS traffic."
        confirmLabel="Delete"
        destructive
        onConfirm={() => deleteTarget !== null && deleteRule(deleteTarget)}
        onCancel={() => setDeleteTarget(null)}
      />

      <div className="space-y-8">
        <div className="space-y-1">
          <h1 className="text-2xl font-bold tracking-tight text-foreground">Traffic Steering</h1>
          <p className="text-muted-foreground text-[10px] font-bold uppercase tracking-widest">Define routing rules to control how DNS traffic is resolved across your network.</p>
        </div>

        <div className="grid grid-cols-1 lg:grid-cols-12 gap-8">
          <Card className="lg:col-span-8 shadow-sm">
            <CardHeader className="pb-4">
              <div className="flex items-center gap-2">
                <PlusCircle className="h-5 w-5 text-primary" />
                <CardTitle className="text-[10px] font-bold uppercase tracking-widest text-foreground">Create Steering Rule</CardTitle>
              </div>
              <CardDescription className="text-[10px] font-bold uppercase tracking-widest">Route traffic based on domain, client IP, or query type.</CardDescription>
            </CardHeader>
            <CardContent className="p-6 pt-0">
              <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
                <div className="space-y-2">
                  <label className="text-sm font-bold text-foreground">Rule Name</label>
                  <Input value={name} onChange={(e) => setName(e.target.value)} placeholder="e.g. Corporate VPN Traffic" />
                </div>
                <div className="space-y-2">
                  <label className="text-sm font-bold text-foreground">Priority</label>
                  <select value={priority} onChange={e => setPriority(Number(e.target.value))} className="flex h-10 w-full items-center justify-between bg-muted px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-ring disabled:cursor-not-allowed disabled:opacity-50 transition-colors font-medium text-foreground">
                    {[1,2,3,4,5,6,7,8,9,10].map(p => <option key={p} value={p}>#{p}</option>)}
                  </select>
                </div>
                <div className="space-y-2">
                  <label className="text-sm font-bold text-foreground">Condition Type</label>
                  <select value={conditionType} onChange={(e) => setConditionType(e.target.value)} className="flex h-10 w-full items-center justify-between bg-muted px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-ring disabled:cursor-not-allowed disabled:opacity-50 transition-colors font-medium text-foreground">
                    <option>Domain</option>
                    <option>Client IP</option>
                    <option>Query Type</option>
                    <option>Time Range</option>
                  </select>
                </div>
                <div className="space-y-2">
                  <label className="text-sm font-bold text-foreground">Condition Value</label>
                  <Input value={conditionValue} onChange={(e) => setConditionValue(e.target.value)} placeholder={conditionType === 'Domain' ? '*.corp.internal' : conditionType === 'Client IP' ? '192.168.1.0/24' : conditionType === 'Query Type' ? 'A, AAAA' : '09:00-18:00'} />
                </div>
                <div className="space-y-2">
                  <label className="text-sm font-bold text-foreground">Action</label>
                  <select value={actionType} onChange={(e) => { setActionType(e.target.value); if (e.target.value === 'Block') setActionValue('') }} className="flex h-10 w-full items-center justify-between bg-muted px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-ring disabled:cursor-not-allowed disabled:opacity-50 transition-colors font-medium text-foreground">
                    <option>Forward</option>
                    <option>Block</option>
                    <option>Redirect</option>
                  </select>
                </div>
                <div className="space-y-2">
                  <label className="text-sm font-bold text-foreground">Target{actionType === 'Block' ? ' (N/A)' : ''}</label>
                  <Input value={actionValue} onChange={(e) => setActionValue(e.target.value)} placeholder={actionType === 'Forward' ? '10.0.0.1' : actionType === 'Redirect' ? '192.168.1.100' : '—'} disabled={actionType === 'Block'} />
                </div>
              </div>
              <div className="flex justify-end gap-3 mt-6">
                <Button variant="outline" className="text-[10px] font-bold uppercase tracking-widest" onClick={() => { setName(''); setConditionValue(''); setActionValue('') }}>Cancel</Button>
                <Button className="text-[10px] font-bold uppercase tracking-widest shadow-sm" onClick={handleAdd} disabled={saving}>{saving ? 'Adding…' : 'Add Rule'}</Button>
              </div>
            </CardContent>
          </Card>

          <div className="lg:col-span-4 grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-1 gap-6">
            <Card className="bg-primary/5 border-primary/20 shadow-sm transition-all duration-300 hover:shadow-md hover:scale-[1.01]">
              <CardContent className="p-6 flex flex-col justify-between h-full min-h-[140px]">
                <div className="p-2 w-fit bg-primary/10 rounded-lg border border-primary/20 shadow-sm">
                  <Gauge className="h-6 w-6 text-primary" />
                </div>
                <div>
                  <p className="text-3xl font-bold tracking-tight text-foreground">{activeCount}</p>
                  <p className="text-[10px] uppercase tracking-widest text-muted-foreground font-bold">Active Rules</p>
                </div>
              </CardContent>
            </Card>
            <Card className="shadow-sm">
              <CardContent className="p-6 flex items-center justify-between">
                <div className="space-y-1">
                  <p className="text-2xl font-bold text-foreground">{rules.length}</p>
                  <p className="text-[10px] uppercase font-bold text-muted-foreground tracking-widest">Total Rules</p>
                </div>
                <Globe className="h-6 w-6 text-muted-foreground" />
              </CardContent>
            </Card>
          </div>
        </div>

        <Card className="overflow-hidden shadow-sm">
          <CardHeader className="flex flex-row items-center justify-between pb-4 bg-muted/5">
            <div>
              <CardTitle className="text-[10px] font-bold uppercase tracking-widest text-foreground">Steering Rules</CardTitle>
              <CardDescription className="text-[10px] font-bold uppercase tracking-widest">Ordered by priority — higher rules are evaluated first.</CardDescription>
            </div>
            <Button variant="outline" size="sm" className="gap-2 text-[10px] font-bold uppercase tracking-widest shadow-sm">
              <Power className="h-4 w-4" /> Toggle All
            </Button>
          </CardHeader>
          <div className="overflow-x-auto p-4 pt-0">
            <Table>
              <TableHeader>
                <TableRow className="bg-muted/20">
                  <TableHead className="text-[10px] font-bold uppercase tracking-widest text-muted-foreground w-[60px]">#</TableHead>
                  <TableHead className="text-[10px] font-bold uppercase tracking-widest text-muted-foreground">Rule</TableHead>
                  <TableHead className="text-[10px] font-bold uppercase tracking-widest text-muted-foreground">Condition</TableHead>
                  <TableHead className="text-[10px] font-bold uppercase tracking-widest text-muted-foreground hidden md:table-cell">Action</TableHead>
                  <TableHead className="text-[10px] font-bold uppercase tracking-widest text-muted-foreground w-[80px]">Status</TableHead>
                  <TableHead className="text-right text-[10px] font-bold uppercase tracking-widest text-muted-foreground w-[60px]"></TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {loading ? renderSkeletonRows() : rules.length === 0 ? (
                  <TableRow>
                    <TableCell colSpan={6} className="h-24 text-center text-muted-foreground text-sm font-medium">
                      No steering rules configured yet. Create one above.
                    </TableCell>
                  </TableRow>
                ) : (
                  rules.map((rule) => (
                    <TableRow key={rule.id} className="group transition-colors hover:bg-muted/50 hover:shadow-sm animate-in fade-in duration-200">
                      <TableCell><span className="text-xs font-bold text-muted-foreground">#{rule.priority}</span></TableCell>
                      <TableCell><span className="font-semibold text-foreground">{rule.name}</span></TableCell>
                      <TableCell>
                        <code className="bg-muted px-2 py-0.5 rounded text-xs font-medium text-muted-foreground">{rule.condition_type}: {rule.condition_value}</code>
                      </TableCell>
                      <TableCell className="hidden md:table-cell">
                        <Badge variant="outline" className={`font-bold text-[9px] px-2 py-0 border-none ${rule.action_type === 'Block' ? 'bg-destructive/10 text-destructive' : 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400'}`}>
                          {rule.action_type}{rule.action_target ? ` → ${rule.action_target}` : ''}
                        </Badge>
                      </TableCell>
                      <TableCell>
                        <Switch checked={rule.enabled} onCheckedChange={() => toggleRule(rule)} size="sm" />
                      </TableCell>
                      <TableCell className="text-right">
                        <Button variant="ghost" size="icon" className="h-8 w-8 text-muted-foreground hover:text-destructive" onClick={() => setDeleteTarget(rule.id)}>
                          <Trash2 className="h-4 w-4" />
                        </Button>
                      </TableCell>
                    </TableRow>
                  ))
                )}
              </TableBody>
            </Table>
          </div>
        </Card>

      </div>
    </PageTransition>
  )
}

const UPSTREAM_OPTIONS = [
  { label: 'Cloudflare (1.1.1.1)', value: '1.1.1.1:53' },
  { label: 'Google (8.8.8.8)', value: '8.8.8.8:53' },
  { label: 'Quad9 (9.9.9.9)', value: '9.9.9.9:53' },
]

const THEME_OPTIONS: { label: string; value: Theme; icon: React.ElementType }[] = [
  { label: 'Light', value: 'light', icon: Sun },
  { label: 'Dark', value: 'dark', icon: Moon },
  { label: 'System', value: 'system', icon: Monitor },
]

const SettingsPage = () => {
  const { theme, setTheme } = useTheme()
  const [serverName, setServerName] = useState('north-america-east-1')
  const [autoUpdate, setAutoUpdate] = useState(true)
  const [upstream, setUpstream] = useState('1.1.1.1:53')
  const [customUpstream, setCustomUpstream] = useState('')
  const [saving, setSaving] = useState(false)
  const [loaded, setLoaded] = useState(false)

  useEffect(() => {
    getSettings().then((s) => {
      if (s.server_name) setServerName(s.server_name)
      if (s.auto_update) setAutoUpdate(s.auto_update === 'true')
      if (s.upstream_dns) setUpstream(s.upstream_dns)
      setLoaded(true)
    }).catch(() => setLoaded(true))
  }, [])

  const handleSave = async () => {
    setSaving(true)
    const resolvedUpstream = upstream === 'custom' ? customUpstream : upstream
    try {
      await saveSettings({ server_name: serverName, auto_update: String(autoUpdate), upstream_dns: resolvedUpstream })
      toast.success('Settings saved', { description: `Upstream: ${resolvedUpstream}` })
    } catch {
      toast.error('Failed to save settings')
    } finally {
      setSaving(false)
    }
  }

  const isCustom = !UPSTREAM_OPTIONS.find(o => o.value === upstream)

  return (
    <PageTransition>
      <div className="space-y-8">
        <div className="space-y-1">
          <h1 className="text-2xl font-bold tracking-tight text-foreground">Settings</h1>
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
          <div className="grid gap-6">
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
                <CardTitle className="font-bold tracking-tight text-foreground">General Configuration</CardTitle>
                <CardDescription className="text-[10px] font-bold uppercase tracking-widest">Basic node settings and updates.</CardDescription>
              </CardHeader>
              <CardContent className="space-y-6">
                <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
                  <div className="space-y-0.5">
                    <p className="text-sm font-bold text-foreground">Server Name</p>
                    <p className="text-[10px] font-bold text-muted-foreground uppercase tracking-widest">Identify this node in your cluster</p>
                  </div>
                  <input className="flex h-9 w-full sm:w-64 bg-muted px-3 py-1 text-sm placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring font-medium text-foreground" value={serverName} onChange={e => setServerName(e.target.value)} />
                </div>
                <div className="h-[1px] bg-muted" />
                <div className="flex items-center justify-between gap-4">
                  <div className="space-y-0.5">
                    <p className="text-sm font-bold text-foreground">Automatic Updates</p>
                    <p className="text-[10px] font-bold text-muted-foreground uppercase tracking-widest">Keep blocklists and firmware up to date</p>
                  </div>
                  <Switch checked={autoUpdate} onCheckedChange={setAutoUpdate} />
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
                  <input className="flex h-9 w-full bg-muted px-3 py-1 text-sm font-medium text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring" placeholder="e.g. 192.168.1.1:53" value={customUpstream} onChange={e => setCustomUpstream(e.target.value)} />
                )}
              </CardContent>
            </Card>

            <div className="flex justify-end gap-3">
              <Button variant="outline" className="text-[10px] font-bold uppercase tracking-widest" onClick={() => { setServerName('north-america-east-1'); setAutoUpdate(true); setUpstream('1.1.1.1:53') }}>Discard Changes</Button>
              <Button className="shadow-sm text-[10px] font-bold uppercase tracking-widest" onClick={handleSave} disabled={saving}>{saving ? 'Saving…' : 'Save Configuration'}</Button>
            </div>
          </div>
        )}
      </div>
    </PageTransition>
  )
}

const ProfilePage = () => {
  const [currentPw, setCurrentPw] = useState('')
  const [newPw, setNewPw] = useState('')
  const [confirmPw, setConfirmPw] = useState('')
  const [saving, setSaving] = useState(false)

  const handleSave = async () => {
    if (newPw !== confirmPw) { toast.error('Passwords do not match'); return }
    if (newPw.length < 8) { toast.error('Password must be at least 8 characters'); return }
    setSaving(true)
    try {
      const res = await apiPut('/password', { current_password: currentPw, new_password: newPw }) as { ok?: boolean; error?: string }
      if (res.ok) {
        toast.success('Password changed successfully')
        setCurrentPw(''); setNewPw(''); setConfirmPw('')
      } else {
        toast.error(res.error ?? 'Failed to change password')
      }
    } catch {
      toast.error('Network error — please try again')
    } finally {
      setSaving(false)
    }
  }

  return (
    <PageTransition>
      <div className="space-y-8">
        <div className="space-y-1">
          <h1 className="text-2xl font-bold tracking-tight text-foreground">Profile</h1>
          <p className="text-muted-foreground text-[10px] font-bold uppercase tracking-widest">Manage your account details and preferences.</p>
        </div>
        <div className="grid gap-6">
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
                    <input type="password" value={val} onChange={e => setter(e.target.value)} className="flex h-9 w-full sm:w-64 bg-muted px-3 py-1 text-sm font-medium text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring" />
                  </div>
                )
              })}
            </CardContent>
          </Card>
          <div className="flex justify-end gap-3">
            <Button variant="outline" className="text-[10px] font-bold uppercase tracking-widest" onClick={() => { setCurrentPw(''); setNewPw(''); setConfirmPw('') }}>Discard</Button>
            <Button className="shadow-sm text-[10px] font-bold uppercase tracking-widest" onClick={handleSave} disabled={saving}>{saving ? 'Saving…' : 'Save Changes'}</Button>
          </div>
        </div>
      </div>
    </PageTransition>
  )
}

const CloudSyncPage = () => {
  const [autoSync, setAutoSync] = useState(true)
  return (
    <PageTransition>
      <div className="space-y-8">
        <div className="space-y-1">
          <h1 className="text-2xl font-bold tracking-tight text-foreground">Cloud Sync</h1>
          <p className="text-muted-foreground text-[10px] font-bold uppercase tracking-widest">Synchronize your configuration across nodes and clusters.</p>
        </div>
        <div className="grid gap-6">
          <Card className="shadow-sm">
            <CardHeader>
              <CardTitle className="font-bold tracking-tight text-foreground">Sync Status</CardTitle>
              <CardDescription className="text-[10px] font-bold uppercase tracking-widest">Current synchronization state of your cluster.</CardDescription>
            </CardHeader>
            <CardContent>
              <div className="flex items-center gap-3 p-4 bg-emerald-500/5 border border-emerald-500/20 rounded-lg">
                <CheckCircle2 className="h-5 w-5 text-emerald-500" />
                <div className="space-y-0.5">
                  <p className="text-sm font-bold text-foreground">All nodes are in sync</p>
                  <p className="text-[10px] font-bold text-muted-foreground uppercase tracking-widest">Last synchronized 2 hours ago</p>
                </div>
              </div>
            </CardContent>
          </Card>
          <Card className="shadow-sm">
            <CardHeader>
              <CardTitle className="font-bold tracking-tight text-foreground">Sync Configuration</CardTitle>
              <CardDescription className="text-[10px] font-bold uppercase tracking-widest">Configure automatic synchronization settings.</CardDescription>
            </CardHeader>
            <CardContent className="space-y-6">
              <div className="flex items-center justify-between gap-4">
                <div className="space-y-0.5">
                  <p className="text-sm font-bold text-foreground">Auto-Sync</p>
                  <p className="text-[10px] font-bold text-muted-foreground uppercase tracking-widest">Automatically sync configuration changes</p>
                </div>
                <Switch checked={autoSync} onCheckedChange={setAutoSync} />
              </div>
              <div className="h-[1px] bg-muted" />
              <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
                <div className="space-y-0.5">
                  <p className="text-sm font-bold text-foreground">Sync Interval</p>
                  <p className="text-[10px] font-bold text-muted-foreground uppercase tracking-widest">How often to sync with remote nodes</p>
                </div>
                <select className="flex h-9 w-full sm:w-48 bg-muted px-3 py-1 text-sm font-medium text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring">
                  <option>Every 5 minutes</option>
                  <option>Every 15 minutes</option>
                  <option>Every hour</option>
                  <option>Every 6 hours</option>
                </select>
              </div>
            </CardContent>
          </Card>
          <div className="flex justify-end gap-3">
            <Button variant="outline" className="text-[10px] font-bold uppercase tracking-widest" onClick={() => toast.info('Sync initiated')}>Sync Now</Button>
            <Button className="shadow-sm text-[10px] font-bold uppercase tracking-widest" onClick={() => toast.success('Cloud sync settings saved')}>Save Configuration</Button>
          </div>
        </div>
      </div>
    </PageTransition>
  )
}

function ProtectedRoute({ children }: { children: React.ReactNode }) {
  const { isAuthenticated } = useAuth()
  if (!isAuthenticated) return <Navigate to="/login" replace />
  return <>{children}</>
}

// Animated route container — re-triggers on path change
function AnimatedRoutes() {
  const location = useLocation()
  return (
    <Routes location={location}>
      <Route path="/" element={<Dashboard />} />
      <Route path="/logs" element={<LogsPage />} />
      <Route path="/records" element={<RecordManager />} />
      <Route path="/blocklist" element={<BlocklistManager />} />
      <Route path="/steering" element={<SteeringPage />} />
      <Route path="/settings" element={<SettingsPage />} />
      <Route path="/profile" element={<ProfilePage />} />
      <Route path="/cloud-sync" element={<CloudSyncPage />} />
      <Route path="*" element={<Dashboard />} />
    </Routes>
  )
}

export default function App() {
  return (
    <>
      <Toaster
        position="bottom-right"
        toastOptions={{
          classNames: {
            toast: 'bg-card text-foreground shadow-lg',
            title: 'text-sm font-bold',
            description: 'text-xs text-muted-foreground',
            success: 'border-emerald-500/30',
            error: 'border-destructive/30',
            warning: 'border-amber-500/30',
          },
        }}
      />
      <Routes>
        <Route path="/login" element={<LoginPage />} />
        <Route path="/*" element={
          <ProtectedRoute>
            <DashboardLayout>
              <AnimatedRoutes />
            </DashboardLayout>
          </ProtectedRoute>
        } />
      </Routes>
    </>
  )
}
