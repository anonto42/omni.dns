import { useState, useEffect, useCallback } from 'react'
import {
  Ban,
  PlusCircle,
  Download,
  Filter,
  MoreVertical,
  ChevronLeft,
  ChevronRight,
  Zap,
  Cloud,
  Trash2,
  ShieldOff,
} from 'lucide-react'
import { toast } from 'sonner'
import { getBlocklist, addToBlocklist, removeFromBlocklist, type BlockedDomain } from '../api'
import { ConfirmDialog } from '@/components/ui/confirm-dialog'
import { Skeleton } from '@/components/ui/skeleton'
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Switch } from '@/components/ui/switch'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { Badge } from '@/components/ui/badge'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'

interface AdlistSource {
  name: string
  url: string
  domains: number
  lastSynced: string
  enabled: boolean
}

export default function BlocklistManager() {
  const [list, setList] = useState<BlockedDomain[]>([])
  const [loading, setLoading] = useState(true)
  const [blocking, setBlocking] = useState(false)
  const [blockDomain, setBlockDomain] = useState('')
  const [listName, setListName] = useState('')
  const [listUrl, setListUrl] = useState('')
  const [listDesc, setListDesc] = useState('')
  const [unblockTarget, setUnblockTarget] = useState<string | null>(null)

  const [sources] = useState<AdlistSource[]>([
    { name: 'StevenBlack Unified Ads', url: 'raw.githubusercontent.com/...', domains: 134812, lastSynced: '2 mins ago', enabled: true },
    { name: 'OISD Full (Security Only)', url: 'small.oisd.nl/dns', domains: 892104, lastSynced: '1 hour ago', enabled: true },
    { name: 'Experimental Cryptojacking', url: 'mirror.blocklist.net/...', domains: 12476, lastSynced: '3 days ago', enabled: false },
    { name: 'Privacy Protections', url: 'tracking-list.internal', domains: 209000, lastSynced: '12 mins ago', enabled: true },
  ])

  const loadBlocklist = useCallback(async () => {
    try {
      const data = await getBlocklist()
      setList(data || [])
    } catch {
      toast.error('Failed to load blocklist')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    loadBlocklist()
  }, [loadBlocklist])

  const handleBlock = async () => {
    const d = blockDomain.trim().toLowerCase()
    if (!d) {
      toast.warning('Enter a domain to block')
      return
    }
    setBlocking(true)
    try {
      await addToBlocklist(d)
      setBlockDomain('')
      await loadBlocklist()
      toast.success('Domain blocked', { description: d })
    } catch {
      toast.error('Failed to block domain')
    } finally {
      setBlocking(false)
    }
  }

  const handleUnblock = async (domain: string) => {
    try {
      await removeFromBlocklist(domain)
      await loadBlocklist()
      toast.success('Domain unblocked', { description: domain })
    } catch {
      toast.error('Failed to unblock domain')
    }
  }

  const renderSkeletonRows = () =>
    Array.from({ length: 4 }).map((_, i) => (
      <TableRow key={i}>
        <TableCell><div className="space-y-1.5"><Skeleton className="h-3 w-40" /><Skeleton className="h-2.5 w-56" /></div></TableCell>
        <TableCell><Skeleton className="h-3 w-16" /></TableCell>
        <TableCell><Skeleton className="h-3 w-20" /></TableCell>
        <TableCell><Skeleton className="h-5 w-24 rounded-full" /></TableCell>
        <TableCell />
      </TableRow>
    ))

  return (
    <div className="w-full space-y-8">
      <ConfirmDialog
        open={unblockTarget !== null}
        title="Unblock domain?"
        description={`Remove "${unblockTarget}" from the blocklist. DNS queries for this domain will be forwarded.`}
        confirmLabel="Unblock"
        onConfirm={() => unblockTarget && handleUnblock(unblockTarget)}
        onCancel={() => setUnblockTarget(null)}
      />

      <div className="flex flex-col md:flex-row md:items-center justify-between gap-4">
        <div className="space-y-1">
          <h2 className="text-2xl font-bold tracking-tight text-foreground">Blocklist Management</h2>
          <p className="text-muted-foreground text-[10px] font-bold uppercase tracking-widest">Control the security perimeter of your network by managing active blocklists.</p>
        </div>
        <Card className="flex items-center gap-6 p-6 min-w-full md:min-w-[320px] shadow-sm">
          <div className="h-12 w-12 rounded-lg bg-destructive/10 flex items-center justify-center text-destructive shrink-0 border border-destructive/20">
            <Ban className="h-6 w-6" />
          </div>
          <div>
            <p className="text-[10px] font-bold text-muted-foreground uppercase tracking-widest">Total Blocked Domains</p>
            <div className="flex items-baseline gap-2">
              {loading ? (
                <Skeleton className="h-8 w-16" />
              ) : (
                <>
                  <span className="text-3xl font-bold text-foreground">{list.length.toLocaleString()}</span>
                  <Badge variant="secondary" className="text-[10px] bg-destructive/10 text-destructive border-none">+{Math.max(1, Math.round(list.length * 0.01))} today</Badge>
                </>
              )}
            </div>
          </div>
        </Card>
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-12 gap-8">
        <div className="lg:col-span-4 space-y-8">
          <Card className="shadow-sm">
            <CardHeader className="pb-4">
              <CardTitle className="text-lg font-bold tracking-tight text-foreground">Add New Adlist</CardTitle>
              <CardDescription className="text-[10px] font-bold uppercase tracking-widest">Add a new source for domain filtering.</CardDescription>
            </CardHeader>
            <CardContent className="p-6">
              <form className="space-y-4" onSubmit={(e) => e.preventDefault()}>
                <div className="space-y-2">
                  <label className="text-sm font-bold text-foreground">List Name</label>
                  <Input value={listName} onChange={(e) => setListName(e.target.value)} placeholder="e.g. Social Media Block" />
                </div>
                <div className="space-y-2">
                  <label className="text-sm font-bold text-foreground">Source URL</label>
                  <Input value={listUrl} onChange={(e) => setListUrl(e.target.value)} placeholder="https://raw.githubusercontent.com/..." type="url" />
                </div>
                <div className="space-y-2">
                  <label className="text-sm font-bold text-foreground">Description (Optional)</label>
                  <textarea
                    value={listDesc}
                    onChange={(e) => setListDesc(e.target.value)}
                    className="flex min-h-[80px] w-full bg-muted px-3 py-2 text-sm placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring disabled:cursor-not-allowed disabled:opacity-50 transition-colors font-medium text-foreground"
                    placeholder="Brief description of this list..."
                    rows={2}
                  />
                </div>
                <Button
                  className="w-full gap-2 shadow-sm text-[10px] font-bold uppercase tracking-widest"
                  type="submit"
                  onClick={() => {
                    if (!listName || !listUrl) { toast.warning('Fill in name and URL'); return }
                    toast.success('Adlist saved', { description: listName })
                    setListName(''); setListUrl(''); setListDesc('')
                  }}
                >
                  <PlusCircle className="h-4 w-4" /> Save and Sync List
                </Button>
              </form>
            </CardContent>
          </Card>

          <Card className="shadow-sm">
            <CardHeader className="pb-4">
              <CardTitle className="text-lg font-bold tracking-tight text-foreground">Block Individual Domain</CardTitle>
              <CardDescription className="text-[10px] font-bold uppercase tracking-widest">Manually add a domain to the blocklist.</CardDescription>
            </CardHeader>
            <CardContent className="p-6 pt-0">
              <form className="space-y-4" onSubmit={(e) => { e.preventDefault(); handleBlock() }}>
                <div className="space-y-2">
                  <label className="text-sm font-bold text-foreground">Domain</label>
                  <Input
                    value={blockDomain}
                    onChange={(e) => setBlockDomain(e.target.value)}
                    placeholder="ads.example.com"
                  />
                </div>
                <Button className="w-full gap-2 shadow-sm text-[10px] font-bold uppercase tracking-widest" type="submit" disabled={blocking}>
                  <Ban className="h-4 w-4" /> {blocking ? 'Blocking…' : 'Block Domain'}
                </Button>
              </form>
            </CardContent>
          </Card>

          {list.length > 0 && (
            <Card className="shadow-sm">
              <CardHeader className="pb-2">
                <CardTitle className="text-sm font-bold tracking-tight text-foreground">Recently Blocked</CardTitle>
              </CardHeader>
              <CardContent className="p-4 pt-0 space-y-1">
                {list.slice(0, 10).map((d) => (
                  <div key={d.domain} className="flex items-center justify-between py-1.5 px-2 rounded-md hover:bg-muted/50 group transition-colors animate-in fade-in duration-200">
                    <span className="text-xs font-medium text-foreground truncate">{d.domain}</span>
                    <Button
                      variant="ghost"
                      size="icon"
                      className="h-6 w-6 opacity-0 group-hover:opacity-100 text-destructive transition-opacity"
                      onClick={() => setUnblockTarget(d.domain || '')}
                    >
                      <Trash2 className="h-3 w-3" />
                    </Button>
                  </div>
                ))}
              </CardContent>
            </Card>
          )}

        </div>

        <Card className="lg:col-span-8 overflow-hidden shadow-sm">
          <CardHeader className="flex flex-row items-center justify-between pb-4 border-b bg-muted/5">
            <div>
              <CardTitle className="text-[10px] font-bold uppercase tracking-widest text-foreground">Active Blocklists</CardTitle>
              <CardDescription className="text-[10px] font-bold uppercase tracking-widest">Currently active domain filters.</CardDescription>
            </div>
            <div className="flex gap-2">
              <Button variant="outline" size="sm" className="gap-2 text-[10px] font-bold uppercase tracking-widest shadow-sm">
                <Download className="h-4 w-4" /> Update All
              </Button>
              <Button variant="outline" size="sm" className="gap-2 text-[10px] font-bold uppercase tracking-widest shadow-sm">
                <Filter className="h-4 w-4" /> Filters
              </Button>
            </div>
          </CardHeader>
          <div className="overflow-x-auto">
            <Table>
              <TableHeader>
                <TableRow className="bg-muted/20 border-b">
                  <TableHead className="w-[300px] text-[10px] font-bold uppercase tracking-widest text-muted-foreground">Source Name</TableHead>
                  <TableHead className="text-[10px] font-bold uppercase tracking-widest text-muted-foreground">Domains</TableHead>
                  <TableHead className="text-[10px] font-bold uppercase tracking-widest text-muted-foreground">Last Synced</TableHead>
                  <TableHead className="text-[10px] font-bold uppercase tracking-widest text-muted-foreground">Status</TableHead>
                  <TableHead className="text-right text-[10px] font-bold uppercase tracking-widest text-muted-foreground"></TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {loading ? renderSkeletonRows() : sources.length === 0 ? (
                  <TableRow>
                    <TableCell colSpan={5} className="h-32 text-center">
                      <div className="flex flex-col items-center gap-3 py-4 text-muted-foreground">
                        <ShieldOff className="h-8 w-8 opacity-40" />
                        <p className="text-sm font-medium">No blocklists configured</p>
                      </div>
                    </TableCell>
                  </TableRow>
                ) : (
                  sources.map((source) => (
                    <TableRow key={source.name} className="group transition-colors hover:bg-muted/50">
                      <TableCell>
                        <div className="flex flex-col gap-0.5">
                          <span className={`font-semibold text-sm ${!source.enabled ? 'text-muted-foreground' : 'text-foreground'}`}>{source.name}</span>
                          <span className="text-[10px] text-muted-foreground truncate max-w-[240px] font-mono">{source.url}</span>
                        </div>
                      </TableCell>
                      <TableCell>
                        <span className={`font-mono text-xs font-bold ${source.enabled ? 'text-primary' : 'text-muted-foreground'}`}>
                          {source.domains.toLocaleString()}
                        </span>
                      </TableCell>
                      <TableCell className="text-[10px] font-bold text-muted-foreground uppercase tracking-widest">
                        {source.lastSynced}
                      </TableCell>
                      <TableCell>
                        <div className="flex items-center gap-3">
                          <Switch checked={source.enabled} />
                          <span className={`text-[10px] font-bold uppercase tracking-widest ${source.enabled ? 'text-primary' : 'text-muted-foreground'}`}>
                            {source.enabled ? 'Enabled' : 'Disabled'}
                          </span>
                        </div>
                      </TableCell>
                      <TableCell className="text-right">
                        <DropdownMenu>
                          <DropdownMenuTrigger asChild>
                            <Button variant="ghost" size="icon" className="h-8 w-8 opacity-0 group-hover:opacity-100 transition-opacity">
                              <MoreVertical className="h-4 w-4" />
                            </Button>
                          </DropdownMenuTrigger>
                          <DropdownMenuContent align="end" className="border-border/50">
                            <DropdownMenuItem className="gap-2 text-[10px] font-bold uppercase tracking-widest cursor-pointer">
                              <Download className="h-4 w-4" /> Sync Now
                            </DropdownMenuItem>
                            <DropdownMenuItem className="gap-2 text-[10px] font-bold uppercase tracking-widest cursor-pointer">
                              <Cloud className="h-4 w-4" /> View Details
                            </DropdownMenuItem>
                            <DropdownMenuItem className="text-destructive gap-2 text-[10px] font-bold uppercase tracking-widest cursor-pointer focus:bg-destructive/10 focus:text-destructive">
                              <Ban className="h-4 w-4" /> Disable
                            </DropdownMenuItem>
                          </DropdownMenuContent>
                        </DropdownMenu>
                      </TableCell>
                    </TableRow>
                  ))
                )}
              </TableBody>
            </Table>
          </div>
          <div className="p-4 border-t bg-muted/10 flex flex-col sm:flex-row items-center justify-between gap-4">
            <span className="text-[10px] font-bold text-muted-foreground uppercase tracking-widest">Showing {sources.length} active blocklists</span>
            <div className="flex items-center gap-1">
              <Button variant="outline" size="icon" className="h-7 w-7 rounded-md" disabled>
                <ChevronLeft className="h-3.5 w-3.5" />
              </Button>
              <Button size="sm" className="h-7 px-3 text-[10px] font-bold rounded-md">1</Button>
              <Button variant="outline" size="icon" className="h-7 w-7 rounded-md" disabled>
                <ChevronRight className="h-3.5 w-3.5" />
              </Button>
            </div>
          </div>
        </Card>
      </div>

      <div className="fixed bottom-6 right-6 z-50">
        <Button size="lg" className="rounded-full shadow-lg h-14 px-6 gap-3 group text-[10px] font-bold uppercase tracking-widest">
          <Zap className="h-5 w-5 group-hover:rotate-12 transition-transform" />
          <span className="hidden sm:inline">Optimise All Lists</span>
        </Button>
      </div>
    </div>
  )
}
