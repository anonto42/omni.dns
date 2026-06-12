import { useState, useCallback } from 'react'
import { toast } from 'sonner'
import { getBlocklist, addToBlocklist, removeFromBlocklist, type BlockedDomain } from '../api'
import { dispatchNotificationsUpdate } from '@/lib/notifications'
import { usePolling } from '@/hooks/usePolling'
import { useWindowFocus } from '@/hooks/useWindowFocus'

export function useBlocklist() {
  const [list, setList] = useState<BlockedDomain[]>([])
  const [loading, setLoading] = useState(true)
  const [adding, setAdding] = useState(false)
  const [showForm, setShowForm] = useState(false)
  const [domain, setDomain] = useState('')
  const [deleteTarget, setDeleteTarget] = useState<string | null>(null)
  const [search, setSearch] = useState('')
  const [page, setPage] = useState(1)

  const loadList = useCallback(async () => {
    try {
      const data = await getBlocklist()
      setList(data || [])
    } catch {
      toast.error('Failed to load blocklist')
    } finally {
      setLoading(false)
    }
  }, [])

  usePolling(loadList, 10000, [])
  useWindowFocus(loadList)

  const handleBlock = async () => {
    if (!domain.trim()) {
      toast.warning('Please enter a domain to block')
      return
    }
    setAdding(true)
    const blocked = domain.trim()
    const wildcard = blocked.startsWith('*')
    try {
      await addToBlocklist(blocked, wildcard)
      await loadList()
      setDomain(''); setShowForm(false)
      toast.success('Domain blocked', { description: blocked })
      dispatchNotificationsUpdate()
    } catch {
      toast.error('Failed to block domain')
    } finally {
      setAdding(false)
    }
  }

  const handleUnblock = async (d: string) => {
    try {
      await removeFromBlocklist(d)
      setDeleteTarget(null)
      await loadList()
      toast.success('Domain unblocked', { description: d })
      dispatchNotificationsUpdate()
    } catch {
      toast.error('Failed to unblock domain')
    }
  }

  const filtered = list.filter(item =>
    item.domain?.toLowerCase().includes(search.toLowerCase())
  )

  const handleExport = () => {
    const dataToExport = search ? filtered : list
    if (dataToExport.length === 0) return
    const content = dataToExport.map(item => item.domain).filter(Boolean).join('\n')
    const blob = new Blob([content], { type: 'text/plain' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url; a.download = 'dns-blocklist.txt'; a.click()
    URL.revokeObjectURL(url)
    toast.success('Exported', { description: `${dataToExport.length} blocked domains` })
  }

  return {
    list,
    loading,
    adding,
    showForm,
    setShowForm,
    domain,
    setDomain,
    deleteTarget,
    setDeleteTarget,
    search,
    setSearch,
    page,
    setPage,
    handleBlock,
    handleUnblock,
    filtered,
    handleExport,
  }
}
export default useBlocklist
