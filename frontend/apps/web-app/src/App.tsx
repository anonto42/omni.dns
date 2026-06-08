import { useState } from 'react'
import { StatsCards } from './features/stats'
import { LogTable } from './features/logs'
import { RecordManager } from './features/records'
import { BlocklistManager } from './features/blocklist'

type Tab = 'logs' | 'records' | 'blocklist'

export default function App() {
  const [tab, setTab] = useState<Tab>('logs')

  const tabs: { key: Tab; label: string }[] = [
    { key: 'logs', label: 'Live Logs' },
    { key: 'records', label: 'Custom Records' },
    { key: 'blocklist', label: 'Blocklist' },
  ]

  return (
    <div className="min-h-screen bg-gray-950 text-gray-100 p-6">
      <div className="max-w-6xl mx-auto">
        <h1 className="text-2xl font-bold text-cyan-400 mb-6">
          DNS Server Dashboard
        </h1>

        <StatsCards />

        <div className="flex gap-1 mb-6">
          {tabs.map((t) => (
            <button
              key={t.key}
              onClick={() => setTab(t.key)}
              className={`px-4 py-2 rounded-lg text-sm font-medium transition ${
                tab === t.key
                  ? 'bg-cyan-500 text-gray-950'
                  : 'bg-gray-800 text-gray-400 hover:bg-gray-700'
              }`}
            >
              {t.label}
            </button>
          ))}
        </div>

        {tab === 'logs' && <LogTable />}
        {tab === 'records' && <RecordManager />}
        {tab === 'blocklist' && <BlocklistManager />}
      </div>
    </div>
  )
}
