import { useState, useEffect } from 'react'
import { getRecords, addRecord, deleteRecord } from '../api'

export default function RecordManager() {
  const [records, setRecords] = useState<Record<string, string>>({})
  const [domain, setDomain] = useState('')
  const [ip, setIp] = useState('')

  useEffect(() => {
    getRecords().then(setRecords)
  }, [])

  const handleAdd = async () => {
    if (!domain || !ip) return
    await addRecord(domain, ip)
    setRecords(await getRecords())
    setDomain('')
    setIp('')
  }

  const handleDelete = async (d: string) => {
    await deleteRecord(d)
    setRecords(await getRecords())
  }

  const entries = Object.entries(records)

  return (
    <div className="bg-gray-900 rounded-xl border border-gray-800 p-4">
      <h2 className="text-sm font-medium text-gray-400 mb-4">Custom DNS Records</h2>

      <div className="flex gap-2 mb-4">
        <input
          value={domain}
          onChange={(e) => setDomain(e.target.value)}
          placeholder="mydevice.local"
          className="flex-1 bg-gray-950 border border-gray-700 rounded-lg px-3 py-2 text-sm text-gray-100 placeholder-gray-600 focus:outline-none focus:border-cyan-500"
        />
        <input
          value={ip}
          onChange={(e) => setIp(e.target.value)}
          placeholder="192.168.1.100"
          className="flex-1 bg-gray-950 border border-gray-700 rounded-lg px-3 py-2 text-sm text-gray-100 placeholder-gray-600 focus:outline-none focus:border-cyan-500"
        />
        <button
          onClick={handleAdd}
          className="bg-cyan-500 text-gray-950 px-4 py-2 rounded-lg text-sm font-medium hover:bg-cyan-400"
        >
          Add
        </button>
      </div>

      {entries.length === 0 ? (
        <p className="text-gray-600 text-sm text-center py-6">No custom records yet.</p>
      ) : (
        <div className="space-y-1">
          {entries.map(([d, ip]) => (
            <div
              key={d}
              className="flex justify-between items-center bg-gray-950 rounded-lg px-3 py-2"
            >
              <span className="font-mono text-sm text-gray-300">
                {d} <span className="text-gray-600">→</span> {ip}
              </span>
              <button
                onClick={() => handleDelete(d)}
                className="text-xs text-red-400 hover:text-red-300"
              >
                Remove
              </button>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}
