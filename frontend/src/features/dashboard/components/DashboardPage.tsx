import { useState } from 'react'
import { Download } from 'lucide-react'
import { PageTransition } from '@/components/shared/PageTransition'
import { Button } from '@/components/ui/button'
import { StatsCards } from '@/features/stats'
import { SystemHealth } from '@/features/stats/components/SystemHealth'
import { LogTable } from '@/features/logs'
import NetworkLoadChart from './NetworkLoadChart'
import { exportLogsCSV } from '../utils/exportLogs'

/** Network Overview dashboard — the app's landing page. */
export default function DashboardPage() {
  const [exporting, setExporting] = useState(false)

  const handleExport = async () => {
    setExporting(true)
    try { await exportLogsCSV({ mode: 'live' }) } finally { setExporting(false) }
  }

  return (
    <PageTransition>
      <div className="space-y-6 md:space-y-8">
        <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3">
          <div className="space-y-1">
            <h2 className="text-xl sm:text-2xl font-bold tracking-tight text-foreground">Network Overview</h2>
            <p className="text-muted-foreground text-[10px] font-bold uppercase tracking-widest">Real-time monitoring for your DNS server.</p>
          </div>

          <div className="flex flex-wrap items-center gap-2 sm:gap-3">
            {/* Export */}
            <Button
              size="sm"
              className="gap-2 shadow-sm text-[10px] font-bold uppercase tracking-widest btn-premium glow-primary"
              onClick={handleExport}
              disabled={exporting}
            >
              <Download className="h-3.5 w-3.5" />
              {exporting ? 'Exporting…' : 'Export Report'}
            </Button>
          </div>
        </div>

        <StatsCards />
        <NetworkLoadChart />

        <div className="grid grid-cols-1 lg:grid-cols-12 gap-4 md:gap-6 lg:gap-8 items-stretch">
          <div className="lg:col-span-8 flex flex-col">
            <LogTable compact />
          </div>
          <div className="lg:col-span-4 flex flex-col" data-tour="system-health">
            <SystemHealth />
          </div>
        </div>
      </div>
    </PageTransition>
  )
}
