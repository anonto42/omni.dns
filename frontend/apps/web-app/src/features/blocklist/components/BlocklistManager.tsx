import { useState, useEffect } from 'react'
import { getBlocklist, addToBlocklist, removeFromBlocklist, type BlockedDomain } from '../api'

export default function BlocklistManager() {
  const [list, setList] = useState<BlockedDomain[]>([])
  const [domain, setDomain] = useState('')

  useEffect(() => {
    getBlocklist().then(setList)
  }, [])

  const handleAdd = async () => {
    if (!domain) return
    await addToBlocklist(domain)
    setList(await getBlocklist())
    setDomain('')
  }

  const handleDelete = async (d: string) => {
    await removeFromBlocklist(d)
    setList(await getBlocklist())
  }

  return (
    <div className="bg-gray-900 rounded-xl border border-gray-800 p-4">
      <h2 className="text-sm font-medium text-gray-400 mb-4">Blocked Domains</h2>

      <div className="flex gap-2 mb-4">
        <input
          value={domain}
          onChange={(e) => setDomain(e.target.value)}
          placeholder="ads.example.com"
          className="flex-1 bg-gray-950 border border-gray-700 rounded-lg px-3 py-2 text-sm text-gray-100 placeholder-gray-600 focus:outline-none focus:border-cyan-500"
        />
        <button
          onClick={handleAdd}
          className="bg-red-500 text-white px-4 py-2 rounded-lg text-sm font-medium hover:bg-red-400"
        >
          Block
        </button>
      </div>

      {list.length === 0 ? (
        <p className="text-gray-600 text-sm text-center py-6">No blocked domains.</p>
      ) : (
        <div className="space-y-1">
          {list.map((b) => (
            <div
              key={b.domain}
              className="flex justify-between items-center bg-gray-950 rounded-lg px-3 py-2"
            >
              <span className="font-mono text-sm text-gray-300">
                {b.domain}
                {b.wildcard && (
                  <span className="text-xs text-gray-600 ml-2">(wildcard)</span>
                )}
              </span>
              <button
                onClick={() => handleDelete(b.domain)}
                className="text-xs text-gray-500 hover:text-gray-300"
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
