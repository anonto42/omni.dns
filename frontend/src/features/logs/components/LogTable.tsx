import React from 'react'
import { useNavigate } from 'react-router-dom'
import { Copy, ChevronLeft, ChevronRight, ChevronDown, Trash2, Search, FileX } from 'lucide-react'
import { copyToClipboard } from '@/lib/clipboard'
import { ConfirmDialog } from '@/components/ui/confirm-dialog'
import { Skeleton } from '@/components/ui/skeleton'
import { Card, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import {
  Table, TableBody, TableCell, TableHead, TableHeader, TableRow,
} from '@/components/ui/table'
import { useLogTable } from '../hooks/useLogTable'
import LogDetailRow from './LogDetailRow'

const PAGE_SIZE = 25

const statusConfig: Record<string, { className: string; label: string; accent: string; panelBg: string }> = {
  forwarded: { className: 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400', label: 'Allowed', accent: 'bg-emerald-500', panelBg: 'bg-emerald-500/[0.04]' },
  blocked:   { className: 'bg-rose-500/10 text-rose-600 dark:text-rose-400',          label: 'Blocked', accent: 'bg-rose-500',    panelBg: 'bg-rose-500/[0.04]'    },
  custom:    { className: 'bg-amber-500/10 text-amber-600 dark:text-amber-400',        label: 'Custom',  accent: 'bg-amber-500',   panelBg: 'bg-amber-500/[0.04]'   },
  cached:    { className: 'bg-primary/10 text-primary',                                label: 'Cached',  accent: 'bg-primary',     panelBg: 'bg-primary/[0.04]'     },
}

function formatTs(ts: string, compact: boolean) {
  const d = new Date(ts)
  if (compact) return d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' })
  return d.toLocaleString([], { year: 'numeric', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit', second: '2-digit', hour12: false })
}

function guessType(domain: string): string {
  if (domain.endsWith('.arpa')) return 'PTR'
  if (domain.startsWith('_')) return 'SRV'
  return 'A'
}

interface Props { compact?: boolean }

export default function LogTable({ compact }: Props) {
  const navigate = useNavigate()
  const {
    logs, loading, filter, domainSearch, pendingSearch, page, setPage,
    confirmClear, setConfirmClear, expanded, setExpanded,
    handleDomainSearch, handleFilterChange, handleClear,
  } = useLogTable(compact)

  const totalPages = Math.max(1, Math.ceil(logs.length / PAGE_SIZE))
  const pageLogs = logs.slice((page - 1) * PAGE_SIZE, page * PAGE_SIZE)
  const displayLogs = compact ? logs.slice(0, 5) : pageLogs

  const renderSkeletonRows = (count: number) =>
    Array.from({ length: count }).map((_, i) => (
      <TableRow key={i} className={i % 2 === 1 ? 'bg-muted/20' : ''}>
        <TableCell><Skeleton className="h-3 w-28" /></TableCell>
        {!compact && <TableCell><Skeleton className="h-3 w-24" /></TableCell>}
        {!compact && <TableCell><Skeleton className="h-3 w-32" /></TableCell>}
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

      <Card data-tour="query-logs-list" className={`overflow-hidden shadow-sm${compact ? ' h-full flex flex-col' : ''}`}>
        {!compact && (
          <CardHeader className="pb-3 bg-muted/5">
            <div className="flex flex-col gap-3">
              <div className="relative w-full">
                <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-3.5 w-3.5 text-muted-foreground/70" />
                <input
                  type="text"
                  value={domainSearch}
                  onChange={e => handleDomainSearch(e.target.value)}
                  placeholder="Filter by domain..."
                  className="w-full bg-muted/30 pl-9 pr-4 py-1.5 text-xs text-foreground placeholder:text-muted-foreground/60 border border-border rounded-sm focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-primary/40 focus-visible:bg-background transition-all duration-200"
                />
              </div>
              <div className="flex items-center justify-between gap-2">
                <div className="flex flex-wrap bg-muted/50 p-1 gap-0.5">
                  {(['all', 'allowed', 'blocked', 'cached'] as const).map((f) => (
                    <Button
                      key={f}
                      variant={filter === f ? 'secondary' : 'ghost'}
                      size="sm"
                      onClick={() => handleFilterChange(f)}
                      className={`px-2.5 sm:px-3 h-7 text-[10px] font-bold uppercase tracking-wider ${filter === f ? 'bg-background shadow-sm' : 'text-muted-foreground'}`}
                    >
                      {f}
                    </Button>
                  ))}
                </div>
                <Button
                  variant="ghost"
                  size="sm"
                  className="gap-1.5 text-[10px] font-bold uppercase tracking-widest text-destructive hover:text-destructive hover:bg-destructive/10 h-7 px-3 shrink-0"
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
            <Button variant="link" size="sm" className="h-auto p-0 text-[10px] font-bold uppercase tracking-widest text-primary" onClick={() => navigate('/logs')}>View All</Button>
          </CardHeader>
        )}

        <div className="overflow-x-auto">
          <Table>
            <TableHeader>
              <TableRow className="bg-muted/30">
                <TableHead className="w-[190px] text-[10px] font-bold uppercase tracking-widest text-muted-foreground pl-4">Timestamp</TableHead>
                {!compact && <TableHead className="text-[10px] font-bold uppercase tracking-widest text-muted-foreground">Client IP</TableHead>}
                {!compact && <TableHead className="text-[10px] font-bold uppercase tracking-widest text-muted-foreground w-[150px]">MAC Address</TableHead>}
                <TableHead className="text-[10px] font-bold uppercase tracking-widest text-muted-foreground">Domain</TableHead>
                {!compact && <TableHead className="text-[10px] font-bold uppercase tracking-widest text-muted-foreground w-[60px]">Type</TableHead>}
                <TableHead className="text-right text-[10px] font-bold uppercase tracking-widest text-muted-foreground pr-4">Status</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {loading ? renderSkeletonRows(compact ? 5 : 8) : displayLogs.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={compact ? 3 : 6} className="h-32 text-center">
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
                  <React.Fragment key={l.id ?? idx}>
                    <TableRow
                      className={`group cursor-pointer transition-colors ${isOdd && !isExpanded ? 'bg-muted/[0.15]' : ''} hover:bg-muted/30 ${isExpanded ? config.panelBg : ''}`}
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
                      {!compact && (
                        <TableCell>
                          <span className={`font-mono text-[11px] ${l.mac_address ? 'text-foreground' : 'text-muted-foreground/40'}`}>
                            {l.mac_address || '—'}
                          </span>
                        </TableCell>
                      )}
                      <TableCell>
                        <div className="flex items-center gap-2 group/cell">
                          <span className={`font-mono text-[12px] font-medium truncate max-w-[140px] sm:max-w-[260px] ${l.action === 'blocked' ? 'text-destructive' : 'text-foreground'}`}>
                            {l.domain}
                          </span>
                          <Button
                            variant="ghost" size="icon"
                            className="h-5 w-5 opacity-0 group-hover/cell:opacity-100 transition-opacity shrink-0"
                            onClick={e => { e.stopPropagation(); copyToClipboard(l.domain || '') }}
                          >
                            <Copy className="h-3.5 w-3.5 text-muted-foreground" />
                          </Button>
                        </div>
                      </TableCell>
                      {!compact && (
                        <TableCell>
                          <span className="text-[10px] font-bold text-muted-foreground uppercase tracking-widest">{l.query_type || guessType(l.domain || '')}</span>
                        </TableCell>
                      )}
                      <TableCell className="text-right pr-3">
                        <div className="flex items-center justify-end gap-2">
                          <span className={`inline-flex items-center uppercase text-[9px] font-bold px-2 py-0.5 tracking-wider ${config.className}`}>
                            {config.label}
                          </span>
                          {!compact && (
                            <ChevronDown className={`h-3.5 w-3.5 text-muted-foreground/50 transition-transform duration-200 shrink-0 ${isExpanded ? 'rotate-180' : ''}`} />
                          )}
                        </div>
                      </TableCell>
                    </TableRow>

                    {isExpanded && !compact && (
                      <TableRow className={`hover:bg-transparent ${config.panelBg}`}>
                        <TableCell colSpan={6} className="px-4 pt-0 pb-4">
                          <div className={`h-0.5 w-full ${config.accent} opacity-40 mb-3`} />
                          <LogDetailRow log={l} config={config} guessType={guessType} />
                        </TableCell>
                      </TableRow>
                    )}
                  </React.Fragment>
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
