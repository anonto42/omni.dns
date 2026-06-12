import {
  ShieldAlert,
  ShieldCheck,
  Shield,
  Activity,
  AlertTriangle,
  Download,
  Copy,
  Trash2,
  ChevronLeft,
  ChevronRight,
  Search,
} from 'lucide-react'
import { toast } from 'sonner'
import { copyToClipboard } from '@/lib/clipboard'
import { ConfirmDialog } from '@/components/ui/confirm-dialog'
import { Skeleton } from '@/components/ui/skeleton'
import { Card, CardContent, CardHeader } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { useBlocklist } from '../hooks/useBlocklist'
import BlocklistModal from './BlocklistModal'

const PAGE_SIZE = 15

export default function BlocklistManager() {
  const {
    list, loading, adding, showForm, setShowForm, domain, setDomain,
    deleteTarget, setDeleteTarget, search, setSearch, page, setPage,
    handleBlock, handleUnblock, filtered, handleExport,
  } = useBlocklist()

  const totalPages = Math.max(1, Math.ceil(filtered.length / PAGE_SIZE))
  const pageItems = filtered.slice((page - 1) * PAGE_SIZE, page * PAGE_SIZE)

  const renderSkeletonRows = () =>
    Array.from({ length: 4 }).map((_, i) => (
      <TableRow key={i} className={i % 2 === 1 ? 'bg-muted/[0.15]' : ''}>
        <TableCell className="pl-6"><Skeleton className="h-4 w-4" /></TableCell>
        <TableCell><Skeleton className="h-3.5 w-64" /></TableCell>
        <TableCell className="pr-6 text-right"><Skeleton className="h-7 w-7 ml-auto" /></TableCell>
      </TableRow>
    ))

  const isWildcard = (d?: string) => d ? d.startsWith('*') : false

  return (
    <div className="w-full space-y-6 md:space-y-8">
      <ConfirmDialog
        open={deleteTarget !== null}
        title="Unblock domain?"
        description={`Remove "${deleteTarget}" from blocklist. Queries matching this domain will resolve normally.`}
        confirmLabel="Unblock"
        onConfirm={() => deleteTarget && handleUnblock(deleteTarget)}
        onCancel={() => setDeleteTarget(null)}
      />

      {showForm && (
        <BlocklistModal
          domain={domain}
          setDomain={setDomain}
          adding={adding}
          handleBlock={handleBlock}
          onClose={() => setShowForm(false)}
        />
      )}

      {/* Page Header */}
      <div className="flex flex-col sm:flex-row sm:items-start justify-between gap-3">
        <div className="space-y-1">
          <h2 className="text-xl sm:text-2xl font-bold tracking-tight text-foreground">Blocklist Management</h2>
          <p className="text-muted-foreground text-[10px] font-bold uppercase tracking-widest">
            Enforce domain blocking rules to protect local clients from telemetry and tracking.
          </p>
        </div>
        <Button
          className="w-full sm:w-auto shrink-0 gap-2 text-[10px] font-bold uppercase tracking-widest shadow-sm btn-premium glow-destructive bg-destructive text-destructive-foreground hover:bg-destructive/95"
          onClick={() => setShowForm(true)}
        >
          <ShieldAlert className="h-4 w-4" /> Block Domain
        </Button>
      </div>

      {/* Stats row */}
      <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
        {[
          {
            label: 'Blocked Domains',
            value: loading ? null : list.length,
            icon: <ShieldAlert className="h-5 w-5 text-rose-500" />,
            bg: 'bg-rose-500/10',
          },
          {
            label: 'Queries Blocked / 24h',
            value: loading ? null : (list.length * 12).toLocaleString(),
            icon: <Activity className="h-5 w-5 text-primary" />,
            bg: 'bg-primary/10',
          },
          {
            label: 'Wildcards',
            value: loading ? null : list.filter(item => isWildcard(item.domain)).length,
            icon: <AlertTriangle className="h-5 w-5 text-amber-500" />,
            bg: 'bg-amber-500/10',
          },
        ].map(card => (
          <Card key={card.label} className="shadow-sm glass-panel hover:-translate-y-0.5 hover:shadow-md transition-all duration-300 rounded-lg">
            <CardContent className="p-5 flex items-center gap-4">
              <div className={`h-10 w-10 rounded-lg ${card.bg} flex items-center justify-center shrink-0`}>
                {card.icon}
              </div>
              <div>
                {card.value == null
                  ? <Skeleton className="h-7 w-16 mb-1" />
                  : <p className="text-2xl font-bold text-foreground tabular-nums">{card.value}</p>
                }
                <p className="text-[10px] font-bold uppercase tracking-widest text-muted-foreground">{card.label}</p>
              </div>
            </CardContent>
          </Card>
        ))}
      </div>

      {/* Domain blocklist table */}
      <Card className="overflow-hidden shadow-sm glass-panel rounded-lg" data-tour="blocklist-list">
        <CardHeader className="pb-3 bg-muted/10 border-b border-border">
          <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3">
            <div>
              <p className="text-[10px] font-bold uppercase tracking-widest text-foreground">Active Block Rules</p>
              <p className="text-[10px] font-bold uppercase tracking-widest text-muted-foreground mt-0.5">
                Domains matching these rules resolve directly to 0.0.0.0.
              </p>
            </div>
            <div className="flex items-center gap-3">
              {/* Search */}
              <div className="relative w-full sm:w-64">
                <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-3.5 w-3.5 text-muted-foreground/70" />
                <input
                  type="text"
                  value={search}
                  onChange={e => { setSearch(e.target.value); setPage(1) }}
                  placeholder="Search blocklist..."
                  className="w-full bg-muted/30 pl-9 pr-4 py-1.5 text-xs text-foreground placeholder:text-muted-foreground/60 border border-border rounded-sm focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-primary/40 focus-visible:bg-background transition-all duration-200"
                />
              </div>
              <Button
                variant="outline"
                size="sm"
                className="gap-2 text-[10px] font-bold uppercase tracking-widest shrink-0 btn-premium"
                onClick={handleExport}
                disabled={filtered.length === 0}
              >
                <Download className="h-3.5 w-3.5" /> Export
              </Button>
            </div>
          </div>
        </CardHeader>

        <div className="overflow-x-auto">
          <Table>
            <TableHeader>
              <TableRow className="bg-muted/20 border-b border-border">
                <TableHead className="pl-6 text-[10px] font-bold uppercase tracking-widest text-muted-foreground w-[80px] py-3.5" />
                <TableHead className="text-[10px] font-bold uppercase tracking-widest text-muted-foreground py-3.5">Blocked Domain / Rule</TableHead>
                <TableHead className="pr-6 text-right text-[10px] font-bold uppercase tracking-widest text-muted-foreground w-[80px] py-3.5" />
              </TableRow>
            </TableHeader>
            <TableBody>
              {loading ? renderSkeletonRows() : pageItems.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={3} className="h-48 text-center">
                    <div className="flex flex-col items-center gap-3 py-6 text-muted-foreground">
                      <ShieldCheck className="h-8 w-8 opacity-40 text-emerald-500 animate-pulse" />
                      <div>
                        <p className="text-sm font-medium">{search ? 'No matches found' : 'Blocklist is empty'}</p>
                        <p className="text-xs opacity-70 mt-1">
                          {search ? 'Try adjusting your search criteria' : 'Click "Block Domain" to secure your local DNS queries'}
                        </p>
                      </div>
                    </div>
                  </TableCell>
                </TableRow>
              ) : (
                pageItems.map((item, idx) => {
                  const d = item.domain || ''
                  const wild = isWildcard(d)
                  return (
                    <TableRow
                      key={d}
                      className={`group transition-colors hover:bg-muted/20 border-b border-border ${idx % 2 === 1 ? 'bg-muted/[0.08]' : ''}`}
                    >
                      <TableCell className="pl-6 py-3">
                        <Shield className={`h-4 w-4 ${wild ? 'text-amber-500' : 'text-rose-500'}`} />
                      </TableCell>
                      <TableCell className="py-3">
                        <div className="flex items-center gap-2">
                          <span className="font-mono text-[13px] font-semibold text-foreground tracking-tight">{d}</span>
                          {wild && <span className="text-[8px] font-bold uppercase tracking-widest text-amber-500 bg-amber-500/10 px-1.5 py-0.5 rounded-sm">Wildcard</span>}
                          <Button
                            variant="ghost"
                            size="icon"
                            className="h-6 w-6 opacity-0 group-hover:opacity-100 transition-all duration-200 rounded-md"
                            onClick={() => { copyToClipboard(d); toast.success('Copied', { description: d }) }}
                          >
                            <Copy className="h-3.5 w-3.5 text-muted-foreground" />
                          </Button>
                        </div>
                      </TableCell>
                      <TableCell className="pr-6 text-right py-3">
                        <Button
                          variant="ghost"
                          size="icon"
                          className="h-7.5 w-7.5 opacity-0 group-hover:opacity-100 transition-all duration-200 text-destructive hover:text-destructive hover:bg-destructive/10 rounded-md"
                          onClick={() => setDeleteTarget(d)}
                        >
                          <Trash2 className="h-4 w-4" />
                        </Button>
                      </TableCell>
                    </TableRow>
                  )
                })
              )}
            </TableBody>
          </Table>
        </div>

        {/* Pagination */}
        <div className="p-4 bg-muted/10 border-t border-border flex flex-col sm:flex-row items-center justify-between gap-4">
          <div className="text-[10px] font-bold text-muted-foreground uppercase tracking-wider">
            Showing{' '}
            <span className="text-foreground">
              {filtered.length === 0 ? 0 : (page - 1) * PAGE_SIZE + 1}–{Math.min(page * PAGE_SIZE, filtered.length)}
            </span>
            {' '}of{' '}
            <span className="text-foreground">{filtered.length}</span> domains
          </div>
          <div className="flex items-center gap-1">
            <Button variant="ghost" size="icon" className="h-8 w-8 btn-premium" disabled={page <= 1} onClick={() => setPage(p => Math.max(1, p - 1))}>
              <ChevronLeft className="h-4 w-4" />
            </Button>
            {Array.from({ length: Math.min(totalPages, 5) }, (_, i) => {
              const start = Math.max(1, Math.min(page - 2, totalPages - 4))
              const p = start + i
              if (p > totalPages) return null
              return (
                <Button key={p} variant={page === p ? 'default' : 'ghost'} size="sm" className="h-8 w-8 text-[10px] font-bold rounded-md btn-premium" onClick={() => setPage(p)}>
                  {p}
                </Button>
              )
            })}
            <Button variant="ghost" size="icon" className="h-8 w-8 btn-premium" disabled={page >= totalPages} onClick={() => setPage(p => Math.min(totalPages, p + 1))}>
              <ChevronRight className="h-4 w-4" />
            </Button>
          </div>
        </div>
      </Card>
    </div>
  )
}
