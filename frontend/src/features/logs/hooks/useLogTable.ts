import { useState, useRef, useCallback } from 'react'
import { toast } from 'sonner'
import { getLogs, clearLogs, type QueryLog } from '../api'
import { usePolling } from '@/hooks/usePolling'
import { useWindowFocus } from '@/hooks/useWindowFocus'

export function useLogTable(compact?: boolean) {
  const [logs, setLogs] = useState<QueryLog[]>([])
  const [loading, setLoading] = useState(true)
  const [filter, setFilter] = useState<'all' | 'blocked' | 'allowed' | 'cached'>('all')
  const [domainSearch, setDomainSearch] = useState('')
  const searchTimeout = useRef<ReturnType<typeof setTimeout> | null>(null)
  const [pendingSearch, setPendingSearch] = useState('')
  const [page, setPage] = useState(1)
  const [confirmClear, setConfirmClear] = useState(false)
  const [expanded, setExpanded] = useState<number | null>(null)

  const actionParam = filter === 'blocked' ? 'blocked' : filter === 'allowed' ? 'forwarded' : filter === 'cached' ? 'cached' : undefined

  const fetchLogs = useCallback(async () => {
    try {
      const data = await getLogs({ action: actionParam, domain: pendingSearch || undefined, limit: 500 })
      if (data) {
        setLogs(data)
        setLoading(false)
      }
    } catch {
      setLoading(false)
    }
  }, [actionParam, pendingSearch])

  usePolling(fetchLogs, 3000, [actionParam, pendingSearch])
  useWindowFocus(fetchLogs)

  const handleDomainSearch = (val: string) => {
    setDomainSearch(val)
    setPage(1)
    if (searchTimeout.current) clearTimeout(searchTimeout.current)
    searchTimeout.current = setTimeout(() => setPendingSearch(val), 400)
  }

  const handleFilterChange = (f: typeof filter) => {
    setFilter(f)
    setPage(1)
  }

  const handleClear = async () => {
    try {
      await clearLogs()
      setLogs([])
      setPage(1)
      toast.success('Logs cleared', { description: 'All query logs have been removed.' })
    } catch {
      toast.error('Failed to clear logs', { description: 'Please try again.' })
    }
  }

  return {
    logs,
    loading,
    filter,
    domainSearch,
    pendingSearch,
    page,
    setPage,
    confirmClear,
    setConfirmClear,
    expanded,
    setExpanded,
    handleDomainSearch,
    handleFilterChange,
    handleClear,
  }
}
export default useLogTable
