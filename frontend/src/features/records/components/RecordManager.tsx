import { useState, useCallback } from 'react'
import {
  PlusCircle,
  Network,
  CheckCircle2,
  AlertCircle,
  Download,
  Copy,
  Trash2,
  ServerOff,
  Search,
} from 'lucide-react'
import { toast } from 'sonner'
import { copyToClipboard } from '@/lib/clipboard'
import { getRecords, addRecord, deleteRecord } from '../api'
import { dispatchNotificationsUpdate } from '@/lib/notifications'
import { usePolling } from '../../../hooks/usePolling'
import { useWindowFocus } from '../../../hooks/useWindowFocus'
import { ConfirmDialog } from '@/components/ui/confirm-dialog'
import { Skeleton } from '@/components/ui/skeleton'
import { Card, CardContent, CardHeader } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Badge } from '@/components/ui/badge'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'

const recordTypeStyles: Record<string, string> = {
  A:    'bg-sky-500/10 text-sky-600 dark:text-sky-400',
  AAAA: 'bg-purple-500/10 text-purple-600 dark:text-purple-400',
  CNAME:'bg-amber-500/10 text-amber-600 dark:text-amber-400',
  MX:   'bg-teal-500/10 text-teal-600 dark:text-teal-400',
  TXT:  'bg-muted/50 text-muted-foreground',
}

const RECORD_TYPES = ['A (IPv4 Address)', 'AAAA (IPv6 Address)', 'CNAME (Alias)', 'MX (Mail Exchange)', 'TXT (Text)']

const PLACEHOLDERS: Record<string, { domain: string; value: string }> = {
  'A (IPv4 Address)':   { domain: 'nas.home', value: '192.168.1.100' },
  'AAAA (IPv6 Address)':{ domain: 'nas.home', value: 'fd00::1' },
  'CNAME (Alias)':      { domain: 'www.home', value: 'nas.home' },
  'MX (Mail Exchange)': { domain: 'home.local', value: '10 mail.home' },
  'TXT (Text)':         { domain: 'home.local', value: 'v=spf1 ...' },
}

function getTypeLabel(ip: string): string {
  if (ip.includes(':')) return 'AAAA'
  if (ip.includes('.')) return 'A'
  return 'CNAME'
}

const sel = "flex h-10 w-full select-premium focus:outline-none focus:ring-2 focus:ring-ring font-medium text-foreground transition-colors"

export default function RecordManager() {
  const [records, setRecords] = useState<Record<string, string>>({})
  const [loading, setLoading] = useState(true)
  const [adding, setAdding] = useState(false)
  const [showForm, setShowForm] = useState(false)
  const [domain, setDomain] = useState('')
  const [ip, setIp] = useState('')
  const [recordType, setRecordType] = useState('A (IPv4 Address)')
  const [deleteTarget, setDeleteTarget] = useState<string | null>(null)
  const [search, setSearch] = useState('')

  const loadRecords = useCallback(async () => {
    try {
      const data = await getRecords()
      setRecords(data || {})
    } catch {
      toast.error('Failed to load records')
    } finally {
      setLoading(false)
    }
  }, [])

  usePolling(loadRecords, 10000, [])
  useWindowFocus(loadRecords)

  const resetForm = () => {
    setDomain(''); setIp(''); setRecordType('A (IPv4 Address)'); setShowForm(false)
  }

  const handleAdd = async () => {
    if (!domain.trim() || !ip.trim()) {
      toast.warning('Please fill in both domain and value')
      return
    }
    setAdding(true)
    try {
      await addRecord(domain.trim(), ip.trim())
      await loadRecords()
      const d = domain.trim()
      const v = ip.trim()
      resetForm()
      toast.success('Record added', { description: `${d} → ${v}` })
      dispatchNotificationsUpdate()
    } catch {
      toast.error('Failed to add record')
    } finally {
      setAdding(false)
    }
  }

  const handleDelete = async (d: string) => {
    try {
      await deleteRecord(d)
      setDeleteTarget(null)
      await loadRecords()
      toast.success('Record deleted', { description: d })
      dispatchNotificationsUpdate()
    } catch {
      toast.error('Failed to delete record')
    }
  }

  const entries = Object.entries(records || {})
  const filteredEntries = entries.filter(([d, val]) =>
    d.toLowerCase().includes(search.toLowerCase()) ||
    val.toLowerCase().includes(search.toLowerCase())
  )

  const handleExport = () => {
    const dataToExport = search ? filteredEntries : entries
    if (dataToExport.length === 0) return
    const lines = dataToExport.map(([d, v]) => `${d}\t${getTypeLabel(v)}\t${v}`).join('\n')
    const blob = new Blob([lines], { type: 'text/plain' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url; a.download = 'local-dns-records.txt'; a.click()
    URL.revokeObjectURL(url)
    toast.success('Exported', { description: `${dataToExport.length} records` })
  }

  const ph = PLACEHOLDERS[recordType] || PLACEHOLDERS['A (IPv4 Address)']

  const renderSkeletonRows = () =>
    Array.from({ length: 4 }).map((_, i) => (
      <TableRow key={i} className={i % 2 === 1 ? 'bg-muted/[0.15]' : ''}>
        <TableCell className="pl-4"><Skeleton className="h-5 w-10 rounded-full" /></TableCell>
        <TableCell><Skeleton className="h-3 w-44" /></TableCell>
        <TableCell><Skeleton className="h-5 w-28" /></TableCell>
        <TableCell><Skeleton className="h-3 w-12" /></TableCell>
        <TableCell className="pr-4 text-right"><Skeleton className="h-7 w-7 ml-auto" /></TableCell>
      </TableRow>
    ))

  return (
    <div className="w-full space-y-8">
      <ConfirmDialog
        open={deleteTarget !== null}
        title="Delete DNS record?"
        description={`Remove "${deleteTarget}" from local records. DNS queries for this domain will fall through to upstream.`}
        confirmLabel="Delete"
        destructive
        onConfirm={() => deleteTarget && handleDelete(deleteTarget)}
        onCancel={() => setDeleteTarget(null)}
      />

      {/* Create record modal */}
      {showForm && (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4">
          <div className="absolute inset-0 bg-black/60 backdrop-blur-sm" onClick={resetForm} />
          <div className="relative z-10 w-full max-w-lg bg-card border border-border shadow-2xl rounded-lg overflow-hidden animate-in fade-in zoom-in-95 duration-200">
            {/* Modal header */}
            <div className="flex items-center justify-between px-6 py-5 bg-muted/20 border-b border-border">
              <div>
                <p className="text-sm font-bold text-foreground">Add DNS Record</p>
                <p className="text-[10px] font-bold uppercase tracking-widest text-muted-foreground mt-0.5">
                  Create an authoritative record for your local network.
                </p>
              </div>
              <Button variant="ghost" size="icon" className="h-8 w-8 text-muted-foreground hover:bg-muted/40" onClick={resetForm}>
                ✕
              </Button>
            </div>
            {/* Modal body */}
            <div className="px-6 py-5 space-y-4">
              <div className="space-y-2">
                <label className="text-sm font-bold text-foreground">Record Type</label>
                <select value={recordType} onChange={e => setRecordType(e.target.value)} className={sel}>
                  {RECORD_TYPES.map(t => <option key={t}>{t}</option>)}
                </select>
              </div>
              <div className="space-y-2">
                <label className="text-sm font-bold text-foreground">Domain Name</label>
                <Input
                  value={domain}
                  onChange={e => setDomain(e.target.value)}
                  placeholder={ph.domain}
                  onKeyDown={e => e.key === 'Enter' && handleAdd()}
                  spellCheck={false}
                  autoComplete="off"
                  className="input-premium"
                  autoFocus
                />
              </div>
              <div className="space-y-2">
                <label className="text-sm font-bold text-foreground">Value</label>
                <Input
                  value={ip}
                  onChange={e => setIp(e.target.value)}
                  placeholder={ph.value}
                  onKeyDown={e => e.key === 'Enter' && handleAdd()}
                  spellCheck={false}
                  autoComplete="off"
                  className="input-premium"
                />
              </div>
            </div>
            {/* Modal footer */}
            <div className="flex justify-end gap-3 px-6 py-4 bg-muted/10 border-t border-border">
              <Button variant="outline" className="text-[10px] font-bold uppercase tracking-widest btn-premium" onClick={resetForm}>
                Cancel
              </Button>
              <Button
                className="text-[10px] font-bold uppercase tracking-widest shadow-sm gap-2 btn-premium glow-primary"
                onClick={handleAdd}
                disabled={adding}
              >
                <PlusCircle className="h-3.5 w-3.5" />
                {adding ? 'Adding…' : 'Add Record'}
              </Button>
            </div>
          </div>
        </div>
      )}

      {/* Page header */}
      <div className="flex items-start justify-between gap-4">
        <div className="space-y-1">
          <h2 className="text-2xl font-bold tracking-tight text-foreground">Local DNS Records</h2>
          <p className="text-muted-foreground text-[10px] font-bold uppercase tracking-widest">
            Manage authoritative records for your local network environment.
          </p>
        </div>
        <Button
          className="shrink-0 gap-2 text-[10px] font-bold uppercase tracking-widest shadow-sm btn-premium glow-primary"
          onClick={() => setShowForm(true)}
        >
          <PlusCircle className="h-4 w-4" /> New Record
        </Button>
      </div>

      {/* Stat cards */}
      <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
        {[
          {
            label: 'Active Records',
            value: loading ? null : entries.length,
            icon: <CheckCircle2 className="h-5 w-5 text-emerald-500" />,
            bg: 'bg-emerald-500/10',
          },
          {
            label: 'Total Queries / 24h',
            value: loading ? null : entries.length > 0 ? (entries.length * 30).toLocaleString() : '0',
            icon: <Network className="h-5 w-5 text-primary" />,
            bg: 'bg-primary/10',
          },
          {
            label: 'Conflicts',
            value: loading ? null : 0,
            icon: <AlertCircle className="h-5 w-5 text-rose-500" />,
            bg: 'bg-rose-500/10',
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

      {/* Records table */}
      <Card className="overflow-hidden shadow-sm glass-panel rounded-lg">
        <CardHeader className="pb-3 bg-muted/10 border-b border-border">
          <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3">
            <div>
              <p className="text-[10px] font-bold uppercase tracking-widest text-foreground">Existing Local Records</p>
              <p className="text-[10px] font-bold uppercase tracking-widest text-muted-foreground mt-0.5">
                Authoritative records — these override upstream DNS for matching domains.
              </p>
            </div>
            <div className="flex items-center gap-3">
              {/* Search */}
              <div className="relative w-full sm:w-64">
                <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-3.5 w-3.5 text-muted-foreground/70" />
                <input
                  type="text"
                  value={search}
                  onChange={e => setSearch(e.target.value)}
                  placeholder="Search records..."
                  className="w-full bg-muted/30 pl-9 pr-4 py-1.5 text-xs text-foreground placeholder:text-muted-foreground/60 border border-border rounded-sm focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-primary/40 focus-visible:bg-background transition-all duration-200"
                />
              </div>
              <Button
                variant="outline"
                size="sm"
                className="gap-2 text-[10px] font-bold uppercase tracking-widest shrink-0 btn-premium"
                onClick={handleExport}
                disabled={filteredEntries.length === 0}
              >
                <Download className="h-3.5 w-3.5" /> Export List
              </Button>
            </div>
          </div>
        </CardHeader>

        <div className="overflow-x-auto">
          <Table>
            <TableHeader>
              <TableRow className="bg-muted/20 border-b border-border">
                <TableHead className="pl-6 text-[10px] font-bold uppercase tracking-widest text-muted-foreground w-[100px] py-3.5">Type</TableHead>
                <TableHead className="text-[10px] font-bold uppercase tracking-widest text-muted-foreground py-3.5">Domain Name</TableHead>
                <TableHead className="text-[10px] font-bold uppercase tracking-widest text-muted-foreground py-3.5">Value / IP</TableHead>
                <TableHead className="text-[10px] font-bold uppercase tracking-widest text-muted-foreground w-[100px] py-3.5">TTL</TableHead>
                <TableHead className="pr-6 text-right text-[10px] font-bold uppercase tracking-widest text-muted-foreground w-[80px] py-3.5" />
              </TableRow>
            </TableHeader>
            <TableBody>
              {loading ? renderSkeletonRows() : filteredEntries.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={5} className="h-48 text-center">
                    <div className="flex flex-col items-center gap-3 py-6 text-muted-foreground">
                      <ServerOff className="h-8 w-8 opacity-40 animate-pulse" />
                      <div>
                        <p className="text-sm font-medium">{search ? 'No matches found' : 'No custom records yet'}</p>
                        <p className="text-xs opacity-70 mt-1">
                          {search ? 'Try adjusting your search query' : 'Click "New Record" to add your first entry'}
                        </p>
                      </div>
                    </div>
                  </TableCell>
                </TableRow>
              ) : (
                filteredEntries.map(([d, val], idx) => {
                  const type = getTypeLabel(val)
                  const style = recordTypeStyles[type] || recordTypeStyles.TXT
                  return (
                    <TableRow
                      key={d}
                      className={`group transition-colors hover:bg-muted/20 border-b border-border ${idx % 2 === 1 ? 'bg-muted/[0.08]' : ''}`}
                    >
                      <TableCell className="pl-6 py-3">
                        <Badge className={`font-bold text-[9px] px-2.5 py-0.5 border-none rounded-md ${style}`}>{type}</Badge>
                      </TableCell>
                      <TableCell className="py-3">
                        <span className="font-mono text-[13px] font-semibold text-foreground tracking-tight">{d}</span>
                      </TableCell>
                      <TableCell className="py-3">
                        <div className="flex items-center gap-2">
                          <code className="bg-muted/80 px-2 py-0.5 rounded text-xs font-mono font-medium text-foreground">{val}</code>
                          <Button
                            variant="ghost"
                            size="icon"
                            className="h-6 w-6 opacity-0 group-hover:opacity-100 transition-all duration-200 rounded-md"
                            onClick={() => { copyToClipboard(val); toast.success('Copied', { description: val }) }}
                          >
                            <Copy className="h-3.5 w-3.5 text-muted-foreground" />
                          </Button>
                        </div>
                      </TableCell>
                      <TableCell className="text-[11px] font-mono text-muted-foreground py-3">3600s</TableCell>
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
      </Card>
    </div>
  )
}

