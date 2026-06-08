import { useState } from 'react'
import { getLogs, clearLogs, type QueryLog } from '../api'
import { usePolling } from '../../../hooks/usePolling'

const actionColors: Record<string, string> = {
  forwarded: 'bg-blue-900 text-blue-300',
  blocked: 'bg-red-900 text-red-300',
  custom: 'bg-yellow-900 text-yellow-300',
  cached: 'bg-green-900 text-green-300',
}

export default function LogTable() {
  const [logs, setLogs] = useState<QueryLog[]>([])

  usePolling(async () => {
    try {
      setLogs(await getLogs())
    } catch {}
  }, 3000)

  return (
    <div className="bg-gray-900 rounded-xl border border-gray-800 overflow-hidden">
      <div className="flex justify-between items-center p-4 border-b border-gray-800">
        <span className="text-xs text-gray-500">Auto-refresh every 3s</span>
        <button
          onClick={async () => {
            await clearLogs()
            setLogs([])
          }}
          className="text-xs text-gray-500 hover:text-gray-300 border border-gray-700 px-3 py-1 rounded"
        >
          Clear Logs
        </button>
      </div>
      <div className="overflow-x-auto">
        <table className="w-full text-sm">
          <thead>
            <tr className="text-left text-gray-500 text-xs uppercase border-b border-gray-800">
              <th className="p-3">Time</th>
              <th className="p-3">Client</th>
              <th className="p-3">Domain</th>
              <th className="p-3">Action</th>
            </tr>
          </thead>
          <tbody>
            {logs.length === 0 ? (
              <tr>
                <td colSpan={4} className="p-6 text-center text-gray-600">
                  No queries yet. Make a DNS request to see it here.
                </td>
              </tr>
            ) : (
              [...logs].reverse().map((l) => (
                <tr key={l.id} className="border-b border-gray-800/50 hover:bg-gray-800/30">
                  <td className="p-3 text-gray-400 font-mono text-xs">
                    {new Date(l.timestamp).toLocaleTimeString()}
                  </td>
                  <td className="p-3 text-gray-300 font-mono text-xs">{l.client_ip}</td>
                  <td className="p-3 text-gray-100 font-mono text-xs">{l.domain}</td>
                  <td className="p-3">
                    <span
                      className={`text-xs px-2 py-0.5 rounded font-medium ${
                        actionColors[l.action] || 'bg-gray-800 text-gray-400'
                      }`}
                    >
                      {l.action}
                    </span>
                  </td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>
    </div>
  )
}
