import React from 'react'
import { Copy, ExternalLink } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { copyToClipboard } from '@/lib/clipboard'
import type { QueryLog } from '../api'

interface LogDetailRowProps {
  log: QueryLog
  config: {
    accent: string
    panelBg: string
    className: string
    label: string
  }
  guessType: (domain: string) => string
}

export default function LogDetailRow({ log, config, guessType }: LogDetailRowProps) {
  return (
    <div className={`${config.panelBg} bg-muted/20`}>
      {/* Row 1: client info */}
      <div className="grid grid-cols-2 sm:grid-cols-4">
        {[
          { label: 'Query ID',    value: `#${log.id}` },
          { label: 'Timestamp',   value: new Date(log.timestamp || '').toLocaleString([], { year: 'numeric', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit', second: '2-digit', hour12: false }) },
          { label: 'Client IP',   value: log.client_ip || '—' },
          { label: 'MAC Address', value: log.mac_address || '—' },
        ].map(field => (
          <div key={field.label} className="px-4 py-3">
            <p className="text-[9px] font-bold uppercase tracking-widest text-muted-foreground mb-1">{field.label}</p>
            <p className="font-mono text-[11px] text-foreground">{field.value}</p>
          </div>
        ))}
      </div>

      {/* Row 2: query details */}
      <div className="grid grid-cols-2 sm:grid-cols-4 border-t border-muted/20">
        {[
          { label: 'Query Type',    value: log.query_type || '—' },
          { label: 'Protocol',      value: log.protocol || '—' },
          { label: 'Response Code', value: log.response_code || '—' },
          { label: 'Answer Count',  value: log.answer_count != null ? String(log.answer_count) : '—' },
        ].map(field => (
          <div key={field.label} className="px-4 py-3">
            <p className="text-[9px] font-bold uppercase tracking-widest text-muted-foreground mb-1">{field.label}</p>
            <p className="font-mono text-[11px] text-foreground">{field.value}</p>
          </div>
        ))}
      </div>

      {/* Row 3: resolution info */}
      <div className="grid grid-cols-2 sm:grid-cols-4 border-t border-muted/20">
        <div className="px-4 py-3">
          <p className="text-[9px] font-bold uppercase tracking-widest text-muted-foreground mb-1">Resolved IP</p>
          <p className="font-mono text-[11px] text-foreground">{log.resolved_ip || '—'}</p>
        </div>
        <div className="px-4 py-3 sm:col-span-2">
          <p className="text-[9px] font-bold uppercase tracking-widest text-muted-foreground mb-1">All Answers</p>
          <p className="font-mono text-[11px] text-foreground break-all">{log.all_answers || '—'}</p>
        </div>
        <div className="px-4 py-3">
          <p className="text-[9px] font-bold uppercase tracking-widest text-muted-foreground mb-1">TTL</p>
          <p className="font-mono text-[11px] text-foreground">{log.ttl != null && log.ttl > 0 ? `${log.ttl}s` : '—'}</p>
        </div>
      </div>

      {/* Row 4: upstream + latency */}
      <div className="grid grid-cols-2 sm:grid-cols-4 border-t border-muted/20">
        <div className="px-4 py-3 sm:col-span-2">
          <p className="text-[9px] font-bold uppercase tracking-widest text-muted-foreground mb-1">Upstream Resolver</p>
          <p className="font-mono text-[11px] text-foreground">{log.upstream_resolver || '—'}</p>
        </div>
        <div className="px-4 py-3">
          <p className="text-[9px] font-bold uppercase tracking-widest text-muted-foreground mb-1">Latency</p>
          <p className="font-mono text-[11px] text-foreground">
            {log.latency_ms != null && log.latency_ms > 0 ? `${log.latency_ms.toFixed(1)} ms` : '—'}
          </p>
        </div>
      </div>

      {/* Bottom row: domain + type + action + link */}
      <div className="bg-muted/20 px-4 py-3 flex flex-wrap items-center justify-between gap-4 border-t border-muted/20">
        <div className="flex items-center gap-4 min-w-0 flex-1">
          <span className="text-[9px] font-bold uppercase tracking-widest text-muted-foreground bg-muted/60 px-2 py-1 shrink-0">
            {log.query_type || guessType(log.domain || '')}
          </span>
          <span className={`font-mono text-[12px] font-semibold truncate ${log.action === 'blocked' ? 'text-destructive' : 'text-foreground'}`}>
            {log.domain}
          </span>
          <Button
            variant="ghost" size="icon"
            className="h-6 w-6 shrink-0 text-muted-foreground hover:text-foreground"
            onClick={e => { e.stopPropagation(); copyToClipboard(log.domain || '') }}
          >
            <Copy className="h-3 w-3" />
          </Button>
          <span className={`inline-flex items-center uppercase text-[9px] font-bold px-2 py-0.5 tracking-wider shrink-0 ${config.className}`}>
            {config.label}
          </span>
        </div>
        <a
          href={`https://www.virustotal.com/gui/domain/${log.domain}`}
          target="_blank"
          rel="noopener noreferrer"
          onClick={e => e.stopPropagation()}
          className="text-[9px] font-bold uppercase tracking-widest text-muted-foreground hover:text-foreground flex items-center gap-1.5 shrink-0 transition-colors"
        >
          <ExternalLink className="h-3 w-3" /> VirusTotal
        </a>
      </div>
    </div>
  )
}
