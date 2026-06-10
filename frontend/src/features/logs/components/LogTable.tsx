import { useState, useRef, useCallback } from 'react'
import {
  Laptop,
  Smartphone,
  Router,
  Monitor,
  Cloud,
  Copy,
  ChevronLeft,
  ChevronRight,
  Trash2,
  Search,
  FileX,
} from 'lucide-react'
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
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'

const PAGE_SIZE = 25

const statusConfig: Record<string, { className: string, label: string }> = {
  forwarded: { className: 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border border-emerald-500/20', label: 'Allowed' },
  blocked:   { className: 'bg-rose-500/10 text-rose-600 dark:text-rose-400 border border-rose-500/20', label: 'Blocked' },
  custom:    { className: 'bg-amber-500/10 text-amber-600 dark:text-amber-400 border border-amber-500/20', label: 'Custom' },
  cached:    { className: 'bg-primary/10 text-primary border border-primary/20', label: 'Cached' },
}

const clientIcons: Record<string, React.ElementType> = {
  '192.168.1.45':  Laptop,
  '192.168.1.102': Smartphone,
  '192.168.1.1':   Router,
  '192.168.1.12':  Monitor,
  '10.0.4.19':     Cloud,
}

function getClientIcon(ip: string): React.ElementType {
  return clientIcons[ip] || Laptop
}

interface Props {
  compact?: boolean
}

export default function LogTable({ compact }: Props) {
  const [logs, setLogs] = useState<QueryLog[]>([])
  const [loading, setLoading] = useState(true)
  const [filter, setFilter] = useState<'all' | 'blocked' | 'allowed'>('all')
  const [domainSearch, setDomainSearch] = useState('')
  const searchTimeout = useRef<ReturnType<typeof setTimeout> | null>(null)
  const [pendingSearch, setPendingSearch] = useState('')
  const [page, setPage] = useState(1)
  const [confirmClear, setConfirmClear] = useState(false)

  const actionParam = filter === 'blocked' ? 'blocked' : filter === 'allowed' ? 'forwarded' : undefined

  const fetchLogs = useCallback(async () => {
    try {
      const data = await getLogs({ action: actionParam, domain: pendingSearch || undefined, limit: 500 })
      if (data) {
        setLogs(data)
        setLoading(false)
      }
    } catch {
      setLoading(false)
    }
  }, [actionParam, pendingSearch])

  usePolling(fetchLogs, 3000, [actionParam, pendingSearch])
  useWindowFocus(fetchLogs)

  const handleDomainSearch = (val: string) => {
    setDomainSearch(val)
    setPage(1)
    if (searchTimeout.current) clearTimeout(searchTimeout.current)
    searchTimeout.current = setTimeout(() => setPendingSearch(val), 400)
  }

  const handleFilterChange = (f: 'all' | 'blocked' | 'allowed') => {
    setFilter(f)
    setPage(1)
  }

  const handleClear = async () => {
    try {
      await clearLogs()
      setLogs([])
      setPage(1)
      toast.success('Logs cleared', { description: 'All query logs have been removed.' })
    } catch {
      toast.error('Failed to clear logs', { description: 'Please try again.' })
    }
  }

  const handleCopyDomain = (domain: string) => {
    navigator.clipboard.writeText(domain).then(() => {
      toast.success('Copied to clipboard')
    })
  }

  const totalPages = Math.max(1, Math.ceil(logs.length / PAGE_SIZE))
  const pageLogs = logs.slice((page - 1) * PAGE_SIZE, page * PAGE_SIZE)
  const displayLogs = compact ? logs.slice(0, 5) : pageLogs

  const renderSkeletonRows = (count: number) =>
    Array.from({ length: count }).map((_, i) => (
      <TableRow key={i}>
        <TableCell><Skeleton className="h-3 w-28" /></TableCell>
        {!compact && <TableCell><Skeleton className="h-3 w-24" /></TableCell>}
        <TableCell><Skeleton className="h-3 w-36" /></TableCell>
        {!compact && <TableCell><Skeleton className="h-3 w-16" /></TableCell>}
        <TableCell className="text-right"><Skeleton className="h-5 w-14 rounded-full ml-auto" /></TableCell>
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

      <Card className="overflow-hidden shadow-sm">
        {!compact && (
          <CardHeader className="pb-4 border-b border-border/50 bg-muted/5">
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
                <div className="flex bg-muted/50 p-1 rounded-lg w-fit">
                  {(['all', 'blocked', 'allowed'] as const).map((f) => (
                    <Button
                      key={f}
                      variant={filter === f ? 'secondary' : 'ghost'}
                      size="sm"
                      onClick={() => handleFilterChange(f)}
                      className={`px-4 h-7 text-[10px] font-bold uppercase tracking-wider ${filter === f ? 'bg-background shadow-sm' : 'text-muted-foreground'}`}
                    >
                      {f}
                    </Button>
                  ))}
                </div>
                <Button
                  variant="outline"
                  size="sm"
                  className="gap-1.5 text-[10px] font-bold uppercase tracking-widest border-border/50 text-destructive hover:text-destructive"
                  onClick={() => setConfirmClear(true)}
                >
                  <Trash2 className="h-3.5 w-3.5" /> Clear
                </Button>
              </div>
            </div>
          </CardHeader>
        )}
        {compact && (
          <CardHeader className="pb-4 border-b border-border/50 flex flex-row items-center justify-between bg-muted/5">
            <CardTitle className="text-[10px] font-bold uppercase tracking-widest text-foreground">Recent Queries</CardTitle>
            <Button variant="link" size="sm" className="h-auto p-0 text-[10px] font-bold uppercase tracking-widest text-primary">View All</Button>
          </CardHeader>
        )}
        <div className="overflow-x-auto p-4">
          <Table>
            <TableHeader>
              <TableRow className="bg-muted/20 border-b">
                <TableHead className="w-[200px] text-[10px] font-bold uppercase tracking-widest text-muted-foreground">Timestamp</TableHead>
                {!compact && <TableHead className="text-[10px] font-bold uppercase tracking-widest text-muted-foreground">Client</TableHead>}
                <TableHead className="text-[10px] font-bold uppercase tracking-widest text-muted-foreground">Domain</TableHead>
                {!compact && <TableHead className="text-[10px] font-bold uppercase tracking-widest text-muted-foreground">Type</TableHead>}
                <TableHead className="text-right text-[10px] font-bold uppercase tracking-widest text-muted-foreground">Status</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {loading ? (
                renderSkeletonRows(compact ? 5 : 8)
              ) : displayLogs.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={compact ? 3 : 5} className="h-32 text-center">
                    <div className="flex flex-col items-center gap-3 py-4 text-muted-foreground">
                      <FileX className="h-8 w-8 opacity-40" />
                      <div>
                        <p className="text-sm font-medium">No queries found</p>
                        <p className="text-xs opacity-70 mt-1">
                          {pendingSearch || filter !== 'all'
                            ? 'Try adjusting your filters'
                            : 'Make a DNS request to see it here'}
                        </p>
                      </div>
                    </div>
                  </TableCell>
                </TableRow>
              ) : (
                displayLogs.map((l) => {
                  const config = statusConfig[l.action || 'forwarded'] || statusConfig.forwarded
                  const ClientIcon = getClientIcon(l.client_ip || '')
                  return (
                    <TableRow key={l.id} className="group transition-colors hover:bg-muted/50 animate-in fade-in slide-in-from-bottom-1 duration-200">
                      <TableCell className="font-mono text-[10px] text-muted-foreground/80 whitespace-nowrap">
                        {compact
                          ? new Date(l.timestamp || '').toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' })
                          : new Date(l.timestamp || '').toLocaleString([], { year: 'numeric', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit', second: '2-digit', hour12: false })
                        }
                      </TableCell>
                      {!compact && (
                        <TableCell>
                          <div className="flex items-center gap-2">
                            <ClientIcon className="h-3.5 w-3.5 text-primary opacity-80" />
                            <span className="text-xs font-bold text-foreground">{l.client_ip}</span>
                          </div>
                        </TableCell>
                      )}
                      <TableCell>
                        <div className="flex items-center justify-between group/cell">
                          <span className={`font-mono text-xs truncate max-w-[120px] sm:max-w-[200px] font-medium ${l.action === 'blocked' ? 'text-destructive' : 'text-primary'}`}>
                            {l.domain}
                          </span>
                          <Button
                            variant="ghost"
                            size="icon"
                            className="h-6 w-6 opacity-0 group-hover/cell:opacity-100 transition-opacity"
                            onClick={() => handleCopyDomain(l.domain || '')}
                          >
                            <Copy className="h-3 w-3 text-muted-foreground" />
                          </Button>
                        </div>
                      </TableCell>
                      {!compact && (
                        <TableCell className="text-[10px] font-bold text-muted-foreground uppercase tracking-widest">
                          {l.action === 'forwarded' ? 'A (IPv4)' : l.action === 'blocked' ? 'HTTPS' : l.action === 'cached' ? 'A (IPv4)' : 'TXT'}
                        </TableCell>
                      )}
                      <TableCell className="text-right">
                        <span className={`inline-flex items-center rounded-full uppercase text-[9px] font-bold px-2 py-0.5 tracking-wider ${config.className}`}>
                          {config.label}
                        </span>
                      </TableCell>
                    </TableRow>
                  )
                })
              )}
            </TableBody>
          </Table>
        </div>
        {!compact && (
          <div className="p-4 border-t border-border/50 bg-muted/10 flex flex-col sm:flex-row items-center justify-between gap-4">
            <div className="text-[10px] font-bold text-muted-foreground uppercase tracking-wider">
              Showing{' '}
              <span className="text-foreground">{logs.length === 0 ? 0 : (page - 1) * PAGE_SIZE + 1}–{Math.min(page * PAGE_SIZE, logs.length)}</span>
              {' '}of{' '}
              <span className="text-foreground">{logs.length}</span> queries
            </div>
            <div className="flex items-center gap-1">
              <Button
                variant="outline"
                size="icon"
                className="h-7 w-7 rounded-md"
                disabled={page <= 1}
                onClick={() => setPage(p => Math.max(1, p - 1))}
              >
                <ChevronLeft className="h-3.5 w-3.5" />
              </Button>
              {Array.from({ length: Math.min(totalPages, 5) }, (_, i) => {
                const start = Math.max(1, Math.min(page - 2, totalPages - 4))
                const p = start + i
                if (p > totalPages) return null
                return (
                  <Button
                    key={p}
                    variant={page === p ? 'default' : 'ghost'}
                    size="sm"
                    className="h-7 px-3 text-[10px] font-bold rounded-md"
                    onClick={() => setPage(p)}
                  >
                    {p}
                  </Button>
                )
              })}
              <Button
                variant="outline"
                size="icon"
                className="h-7 w-7 rounded-md"
                disabled={page >= totalPages}
                onClick={() => setPage(p => Math.min(totalPages, p + 1))}
              >
                <ChevronRight className="h-3.5 w-3.5" />
              </Button>
            </div>
          </div>
        )}
      </Card>
    </>
  )
}
