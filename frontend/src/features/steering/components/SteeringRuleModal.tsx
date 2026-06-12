import { PlusCircle } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { CONDITION_PLACEHOLDERS } from '../types'

// ── Props ────────────────────────────────────────────────────────────────

interface SteeringRuleModalProps {
  name: string
  setName: (v: string) => void
  conditionType: string
  setConditionType: (v: string) => void
  conditionValue: string
  setConditionValue: (v: string) => void
  actionType: string
  setActionType: (v: string) => void
  actionTarget: string
  setActionTarget: (v: string) => void
  priority: number
  setPriority: (v: number) => void
  saving: boolean
  onSubmit: () => void
  onClose: () => void
}

const SELECT_CLASS = "flex h-10 w-full select-premium focus:outline-none focus:ring-2 focus:ring-ring font-medium text-foreground transition-colors"

/** Modal form for creating a new steering rule. Props-driven (no internal state). */
export default function SteeringRuleModal({
  name, setName,
  conditionType, setConditionType,
  conditionValue, setConditionValue,
  actionType, setActionType,
  actionTarget, setActionTarget,
  priority, setPriority,
  saving,
  onSubmit,
  onClose,
}: SteeringRuleModalProps) {
  return (
    <div className="fixed inset-0 z-50 flex items-end sm:items-center justify-center p-0 sm:p-4">
      <div className="absolute inset-0 bg-black/60 backdrop-blur-sm" onClick={onClose} />
      <div className="relative z-10 w-full sm:max-w-2xl bg-card border border-border shadow-2xl sm:rounded-lg overflow-hidden animate-in fade-in slide-in-from-bottom-4 sm:zoom-in-95 duration-200 max-h-[90vh] overflow-y-auto">
        {/* Modal header */}
        <div className="flex items-center justify-between px-6 py-5 bg-muted/20 border-b border-border">
          <div>
            <p className="text-sm font-bold text-foreground">Create Steering Rule</p>
            <p className="text-[10px] font-bold uppercase tracking-widest text-muted-foreground mt-0.5">
              Route DNS traffic based on domain, client IP, or query type.
            </p>
          </div>
          <Button variant="ghost" size="icon" className="h-8 w-8 text-muted-foreground hover:bg-muted/40" onClick={onClose}>
            ✕
          </Button>
        </div>
        {/* Modal body */}
        <div className="px-6 py-5 space-y-4">
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
            <div className="space-y-2">
              <label className="text-sm font-bold text-foreground">Rule Name</label>
              <Input value={name} onChange={e => setName(e.target.value)} placeholder="e.g. Block Social Media" className="input-premium" autoFocus />
            </div>
            <div className="space-y-2">
              <label className="text-sm font-bold text-foreground">Priority</label>
              <select value={priority} onChange={e => setPriority(Number(e.target.value))} className={SELECT_CLASS}>
                {[1,2,3,4,5,6,7,8,9,10].map(p => (
                  <option key={p} value={p}>#{p} — {p === 1 ? 'Highest' : p === 10 ? 'Lowest' : `Priority ${p}`}</option>
                ))}
              </select>
            </div>
          </div>

          <div className="h-[1px] bg-border/40" />

          <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
            <div className="space-y-2">
              <label className="text-sm font-bold text-foreground">Condition Type</label>
              <select value={conditionType} onChange={e => { setConditionType(e.target.value); setConditionValue('') }} className={SELECT_CLASS}>
                <option>Domain</option>
                <option>Client IP</option>
                <option>Query Type</option>
              </select>
            </div>
            <div className="space-y-2">
              <label className="text-sm font-bold text-foreground">Condition Value</label>
              <Input
                value={conditionValue}
                onChange={e => setConditionValue(e.target.value)}
                placeholder={CONDITION_PLACEHOLDERS[conditionType]}
                spellCheck={false}
                className="input-premium"
              />
            </div>
          </div>

          <div className="h-[1px] bg-border/40" />

          <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
            <div className="space-y-2">
              <label className="text-sm font-bold text-foreground">Action</label>
              <select value={actionType} onChange={e => { setActionType(e.target.value); if (e.target.value === 'Block') setActionTarget('') }} className={SELECT_CLASS}>
                <option>Forward</option>
                <option>Block</option>
                <option>Redirect</option>
              </select>
              <p className="text-[10px] text-muted-foreground font-bold uppercase tracking-widest mt-1">
                {actionType === 'Forward' && 'Send queries to a specific upstream DNS'}
                {actionType === 'Block' && 'Return 0.0.0.0 — domain does not resolve'}
                {actionType === 'Redirect' && 'Return a specific IP address'}
              </p>
            </div>
            <div className="space-y-2">
              <label className="text-sm font-bold text-foreground">
                Target
                {actionType === 'Block' && <span className="text-muted-foreground font-normal ml-1">(not needed)</span>}
              </label>
              <Input
                value={actionTarget}
                onChange={e => setActionTarget(e.target.value)}
                placeholder={actionType === 'Forward' ? '1.1.1.1:853 or 10.0.0.1:53' : actionType === 'Redirect' ? '192.168.1.100' : '—'}
                disabled={actionType === 'Block'}
                spellCheck={false}
                className="input-premium"
              />
            </div>
          </div>
        </div>
        {/* Modal footer */}
        <div className="flex justify-end gap-3 px-6 py-4 bg-muted/10 border-t border-border">
          <Button variant="outline" className="text-[10px] font-bold uppercase tracking-widest btn-premium" onClick={onClose}>
            Cancel
          </Button>
          <Button className="text-[10px] font-bold uppercase tracking-widest shadow-sm gap-2 btn-premium glow-primary" onClick={onSubmit} disabled={saving}>
            <PlusCircle className="h-3.5 w-3.5" />
            {saving ? 'Adding…' : 'Add Rule'}
          </Button>
        </div>
      </div>
    </div>
  )
}
