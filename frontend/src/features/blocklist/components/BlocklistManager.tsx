import { useState, useCallback, useRef } from 'react'
import {
  Ban,
  Search,
  Trash2,
  ShieldOff,
  Download,
  ChevronLeft,
  ChevronRight,
  Shield,
} from 'lucide-react'
import { toast } from 'sonner'
import { getBlocklist, addToBlocklist, removeFromBlocklist, type BlockedDomain } from '../api'
import { dispatchNotificationsUpdate } from '@/lib/notifications'
import { usePolling } from '../../../hooks/usePolling'
import { useWindowFocus } from '../../../hooks/useWindowFocus'
import { ConfirmDialog } from '@/components/ui/confirm-dialog'
import { Skeleton } from '@/components/ui/skeleton'
import { Card, CardContent, CardHeader } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Switch } from '@/components/ui/switch'
import { Badge } from '@/components/ui/badge'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'

const PAGE_SIZE = 25

function formatDate(ts: string) {
  if (!ts) return '—'
  return new Date(ts).toLocaleString([], {
    year: 'numeric', month: '2-digit', day: '2-digit',
    hour: '2-digit', minute: '2-digit', hour12: false,
  })
}

export default function BlocklistManager() {
  const [list, setList] = useState<BlockedDomain[]>([])
  const [loading, setLoading] = useState(true)
  const [blocking, setBlocking] = useState(false)
  const [showForm, setShowForm] = useState(false)
  const [blockDomain, setBlockDomain] = useState('')
  const [wildcard, setWildcard] = useState(false)
  const [unblockTarget, setUnblockTarget] = useState<string | null>(null)
  const [search, setSearch] = useState('')
  const [pendingSearch, setPendingSearch] = useState('')
  const searchTimeout = useRef<ReturnType<typeof setTimeout> | null>(null)
  const [page, setPage] = useState(1)

  const fetchList = useCallback(async () => {
    try {
      const data = await getBlocklist()
      setList(data || [])
    } catch {
      toast.error('Failed to load blocklist')
    } finally {
      setLoading(false)
    }
  }, [])

  usePolling(fetchList, 10000, [])
  useWindowFocus(fetchList)

  const resetForm = () => {
    setBlockDomain('')
    setWildcard(false)
    setShowForm(false)
  }

  const handleBlock = async () => {
    const d = blockDomain.trim().toLowerCase()
    if (!d) { toast.warning('Enter a domain to block'); return }
    setBlocking(true)
    try {
      await addToBlocklist(d, wildcard)
      resetForm()
      await fetchList()
      toast.success('Domain blocked', { description: wildcard ? `*.${d}` : d })
      dispatchNotificationsUpdate()
    } catch {
      toast.error('Failed to block domain')
    } finally {
      setBlocking(false)
    }
  }

  const handleUnblock = async (domain: string) => {
    try {
      await removeFromBlocklist(domain)
      setUnblockTarget(null)
      await fetchList()
      toast.success('Domain unblocked', { description: domain })
      dispatchNotificationsUpdate()
    } catch {
      toast.error('Failed to unblock domain')
    }
  }

  const handleExport = () => {
    const content = filtered.map(d => (d.wildcard ? `*.${d.domain}` : d.domain)).join('\n')
    const blob = new Blob([content], { type: 'text/plain' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = 'blocklist.txt'
    a.click()
    URL.revokeObjectURL(url)
    toast.success('Exported', { description: `${filtered.length} domains` })
  }

  const handleSearch = (val: string) => {
    setSearch(val)
    setPage(1)
    if (searchTimeout.current) clearTimeout(searchTimeout.current)
    searchTimeout.current = setTimeout(() => setPendingSearch(val), 300)
  }

  const filtered = pendingSearch
    ? list.filter(d => d.domain?.toLowerCase().includes(pendingSearch.toLowerCase()))
    : list

  const totalPages = Math.max(1, Math.ceil(filtered.length / PAGE_SIZE))
  const paginated = filtered.slice((page - 1) * PAGE_SIZE, page * PAGE_SIZE)

  const wildcardCount = list.filter(d => d.wildcard).length
  const exactCount = list.length - wildcardCount

  const renderSkeletonRows = () =>
    Array.from({ length: 6 }).map((_, i) => (
      <TableRow key={i} className={i % 2 === 1 ? 'bg-muted/[0.15]' : ''}>
        <TableCell><Skeleton className="h-3 w-48" /></TableCell>
        <TableCell><Skeleton className="h-5 w-16 rounded-full" /></TableCell>
        <TableCell><Skeleton className="h-3 w-32" /></TableCell>
        <TableCell><Skeleton className="h-7 w-7 ml-auto" /></TableCell>
      </TableRow>
    ))

  return (
    <div className="w-full space-y-8">
      <ConfirmDialog
        open={unblockTarget !== null}
        title="Unblock domain?"
        description={`Remove "${unblockTarget}" from the blocklist. DNS queries for this domain will be forwarded to upstream.`}
        confirmLabel="Unblock"
        onConfirm={() => unblockTarget && handleUnblock(unblockTarget)}
        onCancel={() => setUnblockTarget(null)}
      />

      {/* Block Domain Modal Overlay */}
      {showForm && (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4">
          <div className="absolute inset-0 bg-black/60 backdrop-blur-sm" onClick={resetForm} />
          <div className="relative z-10 w-full max-w-lg bg-card border border-border shadow-2xl rounded-lg overflow-hidden animate-in fade-in zoom-in-95 duration-200">
            {/* Modal header */}
            <div className="flex items-center justify-between px-6 py-5 bg-muted/20 border-b border-border">
              <div>
                <p className="text-sm font-bold text-foreground">Block a Domain</p>
                <p className="text-[10px] font-bold uppercase tracking-widest text-muted-foreground mt-0.5">
                  Manually add a domain to the blocklist.
                </p>
              </div>
              <Button variant="ghost" size="icon" className="h-8 w-8 text-muted-foreground hover:bg-muted/40" onClick={resetForm}>
                ✕
              </Button>
            </div>
            {/* Modal body */}
            <form onSubmit={e => { e.preventDefault(); handleBlock() }}>
              <div className="px-6 py-5 space-y-4">
                <div className="space-y-2">
                  <label className="text-sm font-bold text-foreground">Domain Name</label>
                  <Input
                    value={blockDomain}
                    onChange={e => setBlockDomain(e.target.value)}
                    placeholder="ads.example.com"
                    autoComplete="off"
                    spellCheck={false}
                    className="input-premium"
                    autoFocus
                  />
                  <p className="text-[10px] text-muted-foreground mt-1">
                    {wildcard
                      ? <span className="font-mono text-amber-500">*.{blockDomain || 'example.com'} — all subdomains blocked</span>
                      : <span className="font-mono">Exact match only</span>
                    }
                  </p>
                </div>

                <div className="flex items-center justify-between py-3 px-4 bg-muted/20 border border-border rounded-md">
                  <div>
                    <p className="text-sm font-bold text-foreground">Wildcard block</p>
                    <p className="text-[10px] text-muted-foreground font-medium uppercase tracking-widest">Also block all subdomains</p>
                  </div>
                  <Switch checked={wildcard} onCheckedChange={setWildcard} />
                </div>
              </div>
              {/* Modal footer */}
              <div className="flex justify-end gap-3 px-6 py-4 bg-muted/10 border-t border-border">
                <Button variant="outline" className="text-[10px] font-bold uppercase tracking-widest btn-premium" type="button" onClick={resetForm}>
                  Cancel
                </Button>
                <Button
                  className="text-[10px] font-bold uppercase tracking-widest shadow-sm gap-2 btn-premium glow-primary"
                  type="submit"
                  disabled={blocking}
                >
                  <Ban className="h-3.5 w-3.5" />
                  {blocking ? 'Blocking…' : 'Block Domain'}
                </Button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* Page header */}
      <div className="flex items-start justify-between gap-4">
        <div className="space-y-1">
          <h2 className="text-2xl font-bold tracking-tight text-foreground">Blocklist</h2>
          <p className="text-muted-foreground text-[10px] font-bold uppercase tracking-widest">
            Block domains from resolving across your network.
          </p>
        </div>
        <Button
          className="shrink-0 gap-2 text-[10px] font-bold uppercase tracking-widest shadow-sm btn-premium glow-primary"
          onClick={() => setShowForm(true)}
        >
          <Ban className="h-4 w-4" /> Block Domain
        </Button>
      </div>

      {/* Stat cards */}
      <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
        {[
          {
            label: 'Total Blocked',
            value: loading ? null : list.length.toLocaleString(),
            icon: <Ban className="h-5 w-5 text-rose-500 animate-pulse" />,
            bg: 'bg-rose-500/10',
          },
          {
            label: 'Exact Domains',
            value: loading ? null : exactCount.toLocaleString(),
            icon: <Shield className="h-5 w-5 text-primary" />,
            bg: 'bg-primary/10',
          },
          {
            label: 'Wildcard Rules',
            value: loading ? null : wildcardCount.toLocaleString(),
            icon: <ShieldOff className="h-5 w-5 text-amber-500" />,
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

      {/* Main blocked domains table */}
      <Card className="w-full overflow-hidden shadow-sm glass-panel rounded-lg">
        {/* Toolbar */}
        <CardHeader className="pb-3 bg-muted/10 border-b border-border">
          <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3">
            <div className="relative w-full sm:w-64">
              <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-3.5 w-3.5 text-muted-foreground/70" />
              <input
                type="text"
                value={search}
                onChange={e => handleSearch(e.target.value)}
                placeholder="Search domains..."
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
              <Download className="h-3.5 w-3.5" /> Export List
            </Button>
          </div>
        </CardHeader>

        <div className="overflow-x-auto">
          <Table>
            <TableHeader>
              <TableRow className="bg-muted/20 border-b border-border">
                <TableHead className="pl-6 text-[10px] font-bold uppercase tracking-widest text-muted-foreground py-3.5">Domain</TableHead>
                <TableHead className="text-[10px] font-bold uppercase tracking-widest text-muted-foreground w-[120px] py-3.5">Type</TableHead>
                <TableHead className="text-[10px] font-bold uppercase tracking-widest text-muted-foreground w-[180px] py-3.5">Added</TableHead>
                <TableHead className="pr-6 text-right text-[10px] font-bold uppercase tracking-widest text-muted-foreground w-[80px] py-3.5"></TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {loading ? renderSkeletonRows() : paginated.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={4} className="h-40 text-center">
                    <div className="flex flex-col items-center gap-3 py-6 text-muted-foreground">
                      <ShieldOff className="h-8 w-8 opacity-40" />
                      <div>
                        <p className="text-sm font-medium">
                          {pendingSearch ? 'No matches found' : 'No domains blocked yet'}
                        </p>
                        <p className="text-xs opacity-70 mt-1">
                          {pendingSearch ? 'Try a different search term' : 'Click "Block Domain" to add a domain'}
                        </p>
                      </div>
                    </div>
                  </TableCell>
                </TableRow>
              ) : paginated.map((d, idx) => (
                <TableRow
                  key={d.domain}
                  className={`group transition-colors hover:bg-muted/20 border-b border-border ${idx % 2 === 1 ? 'bg-muted/[0.08]' : ''}`}
                >
                  <TableCell className="pl-6 py-3">
                    <span className={`font-mono text-[13px] font-semibold tracking-tight ${d.wildcard ? 'text-amber-500' : 'text-foreground'}`}>
                      {d.wildcard ? `*.${d.domain}` : d.domain}
                    </span>
                  </TableCell>
                  <TableCell className="py-3">
                    {d.wildcard
                      ? <Badge className="text-[9px] font-bold px-2.5 py-0.5 border-none bg-amber-500/10 text-amber-600 dark:text-amber-400">Wildcard</Badge>
                      : <Badge className="text-[9px] font-bold px-2.5 py-0.5 border-none bg-muted/60 text-muted-foreground">Exact</Badge>
                    }
                  </TableCell>
                  <TableCell className="text-[10px] font-mono text-muted-foreground py-3">
                    {formatDate(d.added_at || '')}
                  </TableCell>
                  <TableCell className="pr-6 text-right py-3">
                    <Button
                      variant="ghost"
                      size="icon"
                      className="h-7.5 w-7.5 opacity-0 group-hover:opacity-100 transition-all duration-200 text-destructive hover:text-destructive hover:bg-destructive/10 rounded-md"
                      onClick={() => setUnblockTarget(d.domain || '')}
                    >
                      <Trash2 className="h-4 w-4" />
                    </Button>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>

        {/* Pagination footer */}
        <div className="p-4 bg-muted/10 border-t border-border flex flex-col sm:flex-row items-center justify-between gap-4">
          <div className="text-[10px] font-bold text-muted-foreground uppercase tracking-wider">
            Showing{' '}
            <span className="text-foreground">
              {filtered.length === 0 ? 0 : (page - 1) * PAGE_SIZE + 1}–{Math.min(page * PAGE_SIZE, filtered.length)}
            </span>
            {' '}of{' '}
            <span className="text-foreground">{filtered.length}</span> domains
            {pendingSearch && <span className="ml-2 text-primary font-bold">· Filtered</span>}
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

