import { ShieldAlert } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'

interface BlocklistModalProps {
  domain: string
  setDomain: (d: string) => void
  adding: boolean
  handleBlock: () => void
  onClose: () => void
}

export default function BlocklistModal({
  domain, setDomain,
  adding,
  handleBlock,
  onClose
}: BlocklistModalProps) {
  return (
    <div className="fixed inset-0 z-50 flex items-end sm:items-center justify-center p-0 sm:p-4">
      <div className="absolute inset-0 bg-black/60 backdrop-blur-sm" onClick={onClose} />
      <div className="relative z-10 w-full sm:max-w-md bg-card border border-border shadow-2xl sm:rounded-lg overflow-hidden animate-in fade-in slide-in-from-bottom-4 sm:zoom-in-95 duration-200">
        <div className="flex items-center justify-between px-6 py-5 bg-muted/20 border-b border-border">
          <div>
            <p className="text-sm font-bold text-foreground">Block Domain</p>
            <p className="text-[10px] font-bold uppercase tracking-widest text-muted-foreground mt-0.5">
              Prevent resolving a domain or wildcard matching.
            </p>
          </div>
          <Button variant="ghost" size="icon" className="h-8 w-8 text-muted-foreground hover:bg-muted/40" onClick={onClose}>
            ✕
          </Button>
        </div>
        <div className="px-6 py-5 space-y-4">
          <div className="space-y-2">
            <label className="text-sm font-bold text-foreground">Domain or Wildcard</label>
            <Input
              value={domain}
              onChange={e => setDomain(e.target.value)}
              placeholder="e.g. tracking-domain.com or *.doubleclick.net"
              onKeyDown={e => e.key === 'Enter' && handleBlock()}
              spellCheck={false}
              autoComplete="off"
              className="input-premium"
              autoFocus
            />
          </div>
        </div>
        <div className="flex justify-end gap-3 px-6 py-4 bg-muted/10 border-t border-border">
          <Button variant="outline" className="text-[10px] font-bold uppercase tracking-widest btn-premium" onClick={onClose}>
            Cancel
          </Button>
          <Button
            className="text-[10px] font-bold uppercase tracking-widest shadow-sm gap-2 btn-premium glow-destructive bg-destructive text-destructive-foreground hover:bg-destructive/95"
            onClick={handleBlock}
            disabled={adding}
          >
            <ShieldAlert className="h-3.5 w-3.5" />
            {adding ? 'Blocking…' : 'Block Domain'}
          </Button>
        </div>
      </div>
    </div>
  )
}
