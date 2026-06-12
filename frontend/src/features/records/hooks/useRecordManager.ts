import { useState, useCallback } from 'react'
import { toast } from 'sonner'
import { getRecords, addRecord, deleteRecord } from '../api'
import { dispatchNotificationsUpdate } from '@/lib/notifications'
import { usePolling } from '@/hooks/usePolling'
import { useWindowFocus } from '@/hooks/useWindowFocus'

export function useRecordManager() {
  const [records, setRecords] = useState<Record<string, string>>({})
  const [loading, setLoading] = useState(true)
  const [adding, setAdding] = useState(false)
  const [showForm, setShowForm] = useState(false)
  const [domain, setDomain] = useState('')
  const [ip, setIp] = useState('')
  const [recordType, setRecordType] = useState('A (IPv4 Address)')
  const [deleteTarget, setDeleteTarget] = useState<string | null>(null)
  const [search, setSearch] = useState('')

  const loadRecords = useCallback(async () => {
    try {
      const data = await getRecords()
      setRecords(data || {})
    } catch {
      toast.error('Failed to load records')
    } finally {
      setLoading(false)
    }
  }, [])

  usePolling(loadRecords, 10000, [])
  useWindowFocus(loadRecords)

  const resetForm = () => {
    setDomain(''); setIp(''); setRecordType('A (IPv4 Address)'); setShowForm(false)
  }

  const handleAdd = async () => {
    if (!domain.trim() || !ip.trim()) {
      toast.warning('Please fill in both domain and value')
      return
    }
    setAdding(true)
    try {
      await addRecord(domain.trim(), ip.trim())
      await loadRecords()
      const d = domain.trim()
      const v = ip.trim()
      resetForm()
      toast.success('Record added', { description: `${d} → ${v}` })
      dispatchNotificationsUpdate()
    } catch {
      toast.error('Failed to add record')
    } finally {
      setAdding(false)
    }
  }

  const handleDelete = async (d: string) => {
    try {
      await deleteRecord(d)
      setDeleteTarget(null)
      await loadRecords()
      toast.success('Record deleted', { description: d })
      dispatchNotificationsUpdate()
    } catch {
      toast.error('Failed to delete record')
    }
  }

  const entries = Object.entries(records || {})
  const filteredEntries = entries.filter(([d, val]) =>
    d.toLowerCase().includes(search.toLowerCase()) ||
    val.toLowerCase().includes(search.toLowerCase())
  )

  const getTypeLabel = (val: string): string => {
    if (val.includes(':')) return 'AAAA'
    if (val.includes('.')) return 'A'
    return 'CNAME'
  }

  const handleExport = () => {
    const dataToExport = search ? filteredEntries : entries
    if (dataToExport.length === 0) return
    const lines = dataToExport.map(([d, v]) => `${d}\t${getTypeLabel(v)}\t${v}`).join('\n')
    const blob = new Blob([lines], { type: 'text/plain' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url; a.download = 'local-dns-records.txt'; a.click()
    URL.revokeObjectURL(url)
    toast.success('Exported', { description: `${dataToExport.length} records` })
  }

  return {
    records,
    loading,
    adding,
    showForm,
    setShowForm,
    domain,
    setDomain,
    ip,
    setIp,
    recordType,
    setRecordType,
    deleteTarget,
    setDeleteTarget,
    search,
    setSearch,
    resetForm,
    handleAdd,
    handleDelete,
    entries,
    filteredEntries,
    getTypeLabel,
    handleExport,
  }
}
export default useRecordManager
