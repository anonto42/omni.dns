import { useState, useRef, useCallback } from 'react'
import { Copy, ChevronLeft, ChevronRight, Trash2, Search, FileX, ExternalLink } from 'lucide-react'
import { toast } from 'sonner'
import { getLogs, clearLogs, type QueryLog } from '../api'
import { usePolling } from '../../../hooks/usePolling'
import { useWindowFocus } from '../../../hooks/useWindowFocus'
import { ConfirmDialog } from '@/components/ui/confirm-dialog'
import { Skeleton } from '@/components/ui/skeleton'
import { Input } from '@/components/ui/input'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import {
  Table, TableBody, TableCell, TableHead, TableHeader, TableRow,
} from '@/components/ui/table'

const PAGE_SIZE = 25

const statusConfig: Record<string, { className: string; label: string }> = {
  forwarded: { className: 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400', label: 'Allowed'  },
  blocked:   { className: 'bg-rose-500/10 text-rose-600 dark:text-rose-400',          label: 'Blocked'  },
  custom:    { className: 'bg-amber-500/10 text-amber-600 dark:text-amber-400',        label: 'Custom'   },
  cached:    { className: 'bg-primary/10 text-primary',                                label: 'Cached'   },
}

function formatTs(ts: string, compact: boolean) {
  const d = new Date(ts)
  if (compact) return d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' })
  return d.toLocaleString([], { year: 'numeric', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit', second: '2-digit', hour12: false })
}

// Derive a rough record type from the domain (heuristic — backend doesn't return it)
function guessType(domain: string): string {
  if (domain.endsWith('.arpa')) return 'PTR'
  if (domain.startsWith('_')) return 'SRV'
  return 'A'
}

interface Props { compact?: boolean }

export default function LogTable({ compact }: Props) {
  const [logs, setLogs] = useState<QueryLog[]>([])
  const [loading, setLoading] = useState(true)
  const [filter, setFilter] = useState<'all' | 'blocked' | 'allowed' | 'cached'>('all')
  const [domainSearch, setDomainSearch] = useState('')
  const searchTimeout = useRef<ReturnType<typeof setTimeout> | null>(null)
  const [pendingSearch, setPendingSearch] = useState('')
  const [page, setPage] = useState(1)
  const [confirmClear, setConfirmClear] = useState(false)
  const [expanded, setExpanded] = useState<number | null>(null)

  const actionParam = filter === 'blocked' ? 'blocked' : filter === 'allowed' ? 'forwarded' : filter === 'cached' ? 'cached' : undefined

  const fetchLogs = useCallback(async () => {
    try {
      const data = await getLogs({ action: actionParam, domain: pendingSearch || undefined, limit: 500 })
      if (data) { setLogs(data); setLoading(false) }
    } catch { setLoading(false) }
  }, [actionParam, pendingSearch])

  usePolling(fetchLogs, 3000, [actionParam, pendingSearch])
  useWindowFocus(fetchLogs)

  const handleDomainSearch = (val: string) => {
    setDomainSearch(val)
    setPage(1)
    if (searchTimeout.current) clearTimeout(searchTimeout.current)
    searchTimeout.current = setTimeout(() => setPendingSearch(val), 400)
  }

  const handleFilterChange = (f: typeof filter) => { setFilter(f); setPage(1) }

  const handleClear = async () => {
    try {
      await clearLogs()
      setLogs([]); setPage(1)
      toast.success('Logs cleared', { description: 'All query logs have been removed.' })
    } catch {
      toast.error('Failed to clear logs', { description: 'Please try again.' })
    }
  }

  const totalPages = Math.max(1, Math.ceil(logs.length / PAGE_SIZE))
  const pageLogs = logs.slice((page - 1) * PAGE_SIZE, page * PAGE_SIZE)
  const displayLogs = compact ? logs.slice(0, 5) : pageLogs

  const renderSkeletonRows = (count: number) =>
    Array.from({ length: count }).map((_, i) => (
      <TableRow key={i} className={i % 2 === 1 ? 'bg-muted/20' : ''}>
        <TableCell><Skeleton className="h-3 w-28" /></TableCell>
        {!compact && <TableCell><Skeleton className="h-3 w-24" /></TableCell>}
        <TableCell><Skeleton className="h-3 w-36" /></TableCell>
        {!compact && <TableCell><Skeleton className="h-3 w-10" /></TableCell>}
        <TableCell className="text-right"><Skeleton className="h-5 w-14 ml-auto" /></TableCell>
      </TableRow>
    ))

  return (
    <>
      <ConfirmDialog
        open={confirmClear}
        title="Clear all query logs?"
        description="This will permanently delete all query history. This action cannot be undone."
        confirmLabel="Clear Logs"
        destructive
        onConfirm={handleClear}
        onCancel={() => setConfirmClear(false)}
      />

      <Card className={`overflow-hidden shadow-sm${compact ? ' h-full flex flex-col' : ''}`}>
        {!compact && (
          <CardHeader className="pb-4 bg-muted/5">
            <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
              <CardTitle className="text-lg font-bold tracking-tight text-foreground">Query Logs</CardTitle>
              <div className="flex flex-wrap items-center gap-3">
                <div className="relative">
                  <Search className="absolute left-2.5 top-1/2 -translate-y-1/2 h-3.5 w-3.5 text-muted-foreground" />
                  <Input
                    value={domainSearch}
                    onChange={e => handleDomainSearch(e.target.value)}
                    placeholder="Filter by domain…"
                    className="pl-8 h-8 w-44 text-xs"
                  />
                </div>
                <div className="flex bg-muted/50 p-1 w-fit">
                  {(['all', 'allowed', 'blocked', 'cached'] as const).map((f) => (
                    <Button
                      key={f}
                      variant={filter === f ? 'secondary' : 'ghost'}
                      size="sm"
                      onClick={() => handleFilterChange(f)}
                      className={`px-3 h-7 text-[10px] font-bold uppercase tracking-wider ${filter === f ? 'bg-background shadow-sm' : 'text-muted-foreground'}`}
                    >
                      {f}
                    </Button>
                  ))}
                </div>
                <Button
                  variant="ghost"
                  size="sm"
                  className="gap-1.5 text-[10px] font-bold uppercase tracking-widest text-destructive hover:text-destructive hover:bg-destructive/10"
                  onClick={() => setConfirmClear(true)}
                >
                  <Trash2 className="h-3.5 w-3.5" /> Clear
                </Button>
              </div>
            </div>
          </CardHeader>
        )}
        {compact && (
          <CardHeader className="pb-4 flex flex-row items-center justify-between bg-muted/5">
            <CardTitle className="text-[10px] font-bold uppercase tracking-widest text-foreground">Recent Queries</CardTitle>
            <Button variant="link" size="sm" className="h-auto p-0 text-[10px] font-bold uppercase tracking-widest text-primary">View All</Button>
          </CardHeader>
        )}

        <div className="overflow-x-auto">
          <Table>
            <TableHeader>
              <TableRow className="bg-muted/30">
                <TableHead className="w-[190px] text-[10px] font-bold uppercase tracking-widest text-muted-foreground pl-4">Timestamp</TableHead>
                {!compact && <TableHead className="text-[10px] font-bold uppercase tracking-widest text-muted-foreground">Client IP</TableHead>}
                <TableHead className="text-[10px] font-bold uppercase tracking-widest text-muted-foreground">Domain</TableHead>
                {!compact && <TableHead className="text-[10px] font-bold uppercase tracking-widest text-muted-foreground w-[60px]">Type</TableHead>}
                <TableHead className="text-right text-[10px] font-bold uppercase tracking-widest text-muted-foreground pr-4">Status</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {loading ? renderSkeletonRows(compact ? 5 : 8) : displayLogs.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={compact ? 3 : 5} className="h-32 text-center">
                    <div className="flex flex-col items-center gap-3 py-4 text-muted-foreground">
                      <FileX className="h-8 w-8 opacity-40" />
                      <div>
                        <p className="text-sm font-medium">No queries found</p>
                        <p className="text-xs opacity-70 mt-1">
                          {pendingSearch || filter !== 'all' ? 'Try adjusting your filters' : 'Make a DNS request to see it here'}
                        </p>
                      </div>
                    </div>
                  </TableCell>
                </TableRow>
              ) : displayLogs.map((l, idx) => {
                const config = statusConfig[l.action || 'forwarded'] || statusConfig.forwarded
                const isExpanded = l.id != null && expanded === l.id
                const isOdd = idx % 2 === 1
                return (
                  <>
                    <TableRow
                      key={l.id}
                      className={`group cursor-pointer ${isOdd ? 'bg-muted/[0.15]' : ''} hover:bg-primary/5 ${isExpanded ? 'bg-primary/5' : ''}`}
                      onClick={() => !compact && setExpanded(isExpanded ? null : (l.id ?? null))}
                    >
                      <TableCell className="font-mono text-[11px] text-muted-foreground whitespace-nowrap pl-4">
                        {formatTs(l.timestamp || '', !!compact)}
                      </TableCell>
                      {!compact && (
                        <TableCell>
                          <span className="font-mono text-[11px] text-muted-foreground">{l.client_ip || '—'}</span>
                        </TableCell>
                      )}
                      <TableCell>
                        <div className="flex items-center gap-2 group/cell">
                          <span className={`font-mono text-[12px] font-medium truncate max-w-[140px] sm:max-w-[280px] ${l.action === 'blocked' ? 'text-destructive' : 'text-foreground'}`}>
                            {l.domain}
                          </span>
                          <Button
                            variant="ghost" size="icon"
                            className="h-5 w-5 opacity-0 group-hover/cell:opacity-100 transition-opacity shrink-0"
                            onClick={e => { e.stopPropagation(); navigator.clipboard.writeText(l.domain || '').then(() => toast.success('Copied')) }}
                          >
                            <Copy className="h-3 w-3 text-muted-foreground" />
                          </Button>
                        </div>
                      </TableCell>
                      {!compact && (
                        <TableCell>
                          <span className="text-[10px] font-bold text-muted-foreground uppercase tracking-widest">{guessType(l.domain || '')}</span>
                        </TableCell>
                      )}
                      <TableCell className="text-right pr-4">
                        <span className={`inline-flex items-center uppercase text-[9px] font-bold px-2 py-0.5 tracking-wider ${config.className}`}>
                          {config.label}
                        </span>
                      </TableCell>
                    </TableRow>

                    {/* Expanded detail row */}
                    {isExpanded && !compact && (
                      <TableRow key={`${l.id}-detail`} className="bg-muted/20 hover:bg-muted/20">
                        <TableCell colSpan={5} className="py-3 px-4">
                          <div className="grid grid-cols-2 sm:grid-cols-4 gap-4">
                            <div>
                              <p className="text-[9px] font-bold uppercase tracking-widest text-muted-foreground mb-1">Query ID</p>
                              <p className="font-mono text-xs text-foreground">#{l.id}</p>
                            </div>
                            <div>
                              <p className="text-[9px] font-bold uppercase tracking-widest text-muted-foreground mb-1">Full Timestamp</p>
                              <p className="font-mono text-xs text-foreground">{new Date(l.timestamp || '').toISOString()}</p>
                            </div>
                            <div>
                              <p className="text-[9px] font-bold uppercase tracking-widest text-muted-foreground mb-1">Client IP</p>
                              <p className="font-mono text-xs text-foreground">{l.client_ip || '—'}</p>
                            </div>
                            <div>
                              <p className="text-[9px] font-bold uppercase tracking-widest text-muted-foreground mb-1">Action</p>
                              <span className={`inline-flex items-center uppercase text-[9px] font-bold px-2 py-0.5 tracking-wider ${config.className}`}>
                                {config.label}
                              </span>
                            </div>
                            <div className="col-span-2 sm:col-span-4">
                              <p className="text-[9px] font-bold uppercase tracking-widest text-muted-foreground mb-1">Full Domain</p>
                              <div className="flex items-center gap-2">
                                <code className="font-mono text-xs text-foreground bg-muted/50 px-2 py-1 block">{l.domain}</code>
                                <a
                                  href={`https://www.virustotal.com/gui/domain/${l.domain}`}
                                  target="_blank"
                                  rel="noopener noreferrer"
                                  onClick={e => e.stopPropagation()}
                                  className="text-[9px] font-bold uppercase tracking-widest text-primary hover:underline flex items-center gap-1 shrink-0"
                                >
                                  <ExternalLink className="h-3 w-3" /> VirusTotal
                                </a>
                              </div>
                            </div>
                          </div>
                        </TableCell>
                      </TableRow>
                    )}
                  </>
                )
              })}
            </TableBody>
          </Table>
        </div>

        {!compact && (
          <div className="p-4 bg-muted/10 flex flex-col sm:flex-row items-center justify-between gap-4">
            <div className="text-[10px] font-bold text-muted-foreground uppercase tracking-wider">
              Showing{' '}
              <span className="text-foreground">{logs.length === 0 ? 0 : (page - 1) * PAGE_SIZE + 1}–{Math.min(page * PAGE_SIZE, logs.length)}</span>
              {' '}of{' '}
              <span className="text-foreground">{logs.length}</span> queries
              {pendingSearch && <span className="ml-2 text-primary">· filtered</span>}
            </div>
            <div className="flex items-center gap-1">
              <Button variant="ghost" size="icon" className="h-7 w-7" disabled={page <= 1} onClick={() => setPage(p => Math.max(1, p - 1))}>
                <ChevronLeft className="h-3.5 w-3.5" />
              </Button>
              {Array.from({ length: Math.min(totalPages, 5) }, (_, i) => {
                const start = Math.max(1, Math.min(page - 2, totalPages - 4))
                const p = start + i
                if (p > totalPages) return null
                return (
                  <Button key={p} variant={page === p ? 'default' : 'ghost'} size="sm" className="h-7 px-3 text-[10px] font-bold" onClick={() => setPage(p)}>
                    {p}
                  </Button>
                )
              })}
              <Button variant="ghost" size="icon" className="h-7 w-7" disabled={page >= totalPages} onClick={() => setPage(p => Math.min(totalPages, p + 1))}>
                <ChevronRight className="h-3.5 w-3.5" />
              </Button>
            </div>
          </div>
        )}
      </Card>
    </>
  )
}
