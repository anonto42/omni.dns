import { PageTransition } from '@/components/shared/PageTransition'
import { LogTable } from '@/features/logs'

/** Full query log page — shows the LogTable without the compact layout. */
export default function LogsPage() {
  return (
    <PageTransition>
      <div className="space-y-6">
        <div className="space-y-1">
          <h1 className="text-2xl font-bold tracking-tight text-foreground">Query Log</h1>
          <p className="text-muted-foreground text-[10px] font-bold uppercase tracking-widest">Real-time DNS traffic and security events across your network.</p>
        </div>
        <LogTable />
      </div>
    </PageTransition>
  )
}
