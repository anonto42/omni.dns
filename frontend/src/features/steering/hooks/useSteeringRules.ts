import { useState, useCallback } from 'react'
import { toast } from 'sonner'
import { apiGet, apiPost, apiPut, apiDelete } from '@/hooks/api'
import { dispatchNotificationsUpdate } from '@/lib/notifications'
import { usePolling } from '@/hooks/usePolling'
import { useWindowFocus } from '@/hooks/useWindowFocus'
import type { SteeringRule } from '../types'

// ── Hook ─────────────────────────────────────────────────────────────────

export function useSteeringRules() {
  const [rules, setRules] = useState<SteeringRule[]>([])
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [deleteTarget, setDeleteTarget] = useState<number | null>(null)
  const [showForm, setShowForm] = useState(false)
  const [search, setSearch] = useState('')

  // Form state
  const [name, setName] = useState('')
  const [conditionType, setConditionType] = useState('Domain')
  const [conditionValue, setConditionValue] = useState('')
  const [actionType, setActionType] = useState('Forward')
  const [actionTarget, setActionTarget] = useState('')
  const [priority, setPriority] = useState(1)

  // ── Data fetching ──────────────────────────────────────────────────────

  const fetchRules = useCallback(async () => {
    try {
      const data = await apiGet<SteeringRule[]>('/steering')
      setRules(data ?? [])
    } catch {
      toast.error('Failed to load steering rules')
    } finally {
      setLoading(false)
    }
  }, [])

  usePolling(fetchRules, 10000, [])
  useWindowFocus(fetchRules)

  // ── Form helpers ───────────────────────────────────────────────────────

  const resetForm = () => {
    setName(''); setConditionValue(''); setActionTarget(''); setPriority(1)
    setConditionType('Domain'); setActionType('Forward')
    setShowForm(false)
  }

  // ── CRUD operations ────────────────────────────────────────────────────

  const handleAdd = async () => {
    if (!name.trim() || !conditionValue.trim()) {
      toast.warning('Fill in rule name and condition value')
      return
    }
    if (actionType !== 'Block' && !actionTarget.trim()) {
      toast.warning('Fill in the target for Forward or Redirect actions')
      return
    }
    setSaving(true)
    try {
      const res = await apiPost('/steering', {
        name: name.trim(),
        condition_type: conditionType,
        condition_value: conditionValue.trim(),
        action_type: actionType,
        action_target: actionType === 'Block' ? '' : actionTarget.trim(),
        priority,
        enabled: true,
      }) as { id: number }
      setRules(prev => [...prev, {
        id: res.id,
        name: name.trim(),
        condition_type: conditionType,
        condition_value: conditionValue.trim(),
        action_type: actionType,
        action_target: actionType === 'Block' ? '' : actionTarget.trim(),
        priority,
        enabled: true,
      }].sort((a, b) => a.priority - b.priority || a.id - b.id))
      resetForm()
      toast.success('Rule added', { description: name.trim() })
      dispatchNotificationsUpdate()
    } catch {
      toast.error('Failed to add rule')
    } finally {
      setSaving(false)
    }
  }

  const toggleRule = async (rule: SteeringRule) => {
    const newEnabled = !rule.enabled
    // Optimistic update first
    setRules(prev => prev.map(r => r.id === rule.id ? { ...r, enabled: newEnabled } : r))
    try {
      await apiPut('/steering', { id: rule.id, enabled: newEnabled })
      toast.success(newEnabled ? 'Rule enabled' : 'Rule disabled', { description: rule.name })
      dispatchNotificationsUpdate()
    } catch {
      // Revert on failure
      setRules(prev => prev.map(r => r.id === rule.id ? { ...r, enabled: rule.enabled } : r))
      toast.error('Failed to update rule')
    }
  }

  const deleteRule = async (id: number) => {
    try {
      await apiDelete('/steering', { id })
      setRules(prev => prev.filter(r => r.id !== id))
      setDeleteTarget(null)
      toast.success('Rule deleted')
      dispatchNotificationsUpdate()
    } catch {
      toast.error('Failed to delete rule')
    }
  }

  // ── Derived state ──────────────────────────────────────────────────────

  const activeCount = rules.filter(r => r.enabled).length

  const filteredRules = rules.filter(r =>
    r.name.toLowerCase().includes(search.toLowerCase()) ||
    r.condition_value.toLowerCase().includes(search.toLowerCase()) ||
    (r.action_target && r.action_target.toLowerCase().includes(search.toLowerCase()))
  )

  return {
    // Data
    rules,
    filteredRules,
    loading,
    saving,
    activeCount,
    search,
    setSearch,
    // Delete dialog
    deleteTarget,
    setDeleteTarget,
    deleteRule,
    // Form
    showForm,
    setShowForm,
    resetForm,
    name, setName,
    conditionType, setConditionType,
    conditionValue, setConditionValue,
    actionType, setActionType,
    actionTarget, setActionTarget,
    priority, setPriority,
    // Actions
    handleAdd,
    toggleRule,
  }
}
