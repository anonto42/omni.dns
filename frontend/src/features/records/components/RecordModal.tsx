import { PlusCircle } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'

interface RecordModalProps {
  recordType: string
  setRecordType: (t: string) => void
  domain: string
  setDomain: (d: string) => void
  ip: string
  setIp: (i: string) => void
  adding: boolean
  handleAdd: () => void
  resetForm: () => void
}

const RECORD_TYPES = ['A (IPv4 Address)', 'AAAA (IPv6 Address)', 'CNAME (Alias)', 'MX (Mail Exchange)', 'TXT (Text)']

const PLACEHOLDERS: Record<string, { domain: string; value: string }> = {
  'A (IPv4 Address)':   { domain: 'nas.home', value: '192.168.1.100' },
  'AAAA (IPv6 Address)':{ domain: 'nas.home', value: 'fd00::1' },
  'CNAME (Alias)':      { domain: 'www.home', value: 'nas.home' },
  'MX (Mail Exchange)': { domain: 'home.local', value: '10 mail.home' },
  'TXT (Text)':         { domain: 'home.local', value: 'v=spf1 ...' },
}

const sel = "flex h-10 w-full select-premium focus:outline-none focus:ring-2 focus:ring-ring font-medium text-foreground transition-colors"

export default function RecordModal({
  recordType, setRecordType,
  domain, setDomain,
  ip, setIp,
  adding,
  handleAdd,
  resetForm
}: RecordModalProps) {
  const ph = PLACEHOLDERS[recordType] || PLACEHOLDERS['A (IPv4 Address)']

  return (
    <div className="fixed inset-0 z-50 flex items-end sm:items-center justify-center p-0 sm:p-4">
      <div className="absolute inset-0 bg-black/60 backdrop-blur-sm" onClick={resetForm} />
      <div className="relative z-10 w-full sm:max-w-lg bg-card border border-border shadow-2xl sm:rounded-lg overflow-hidden animate-in fade-in slide-in-from-bottom-4 sm:zoom-in-95 duration-200 max-h-[90vh] overflow-y-auto">
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
  )
}
