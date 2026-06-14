import { useCallback, useEffect, useState } from 'react'
import { getStatus, type Status } from '../api'

interface SharedStatusState {
  status: Status | null
  loading: boolean
  lastChecked: Date | null
}

const POLL_INTERVAL_MS = 3000

let state: SharedStatusState = {
  status: null,
  loading: true,
  lastChecked: null,
}
let inflight: Promise<void> | null = null
let poller: ReturnType<typeof setInterval> | null = null
let subscribers = 0

const listeners = new Set<(next: SharedStatusState) => void>()

function setSharedState(next: SharedStatusState) {
  state = next
  listeners.forEach(listener => listener(state))
}

export async function refreshSharedStatus() {
  if (inflight) return inflight

  if (!state.status) {
    setSharedState({ ...state, loading: true })
  }

  inflight = getStatus()
    .then(status => {
      setSharedState({ status, loading: false, lastChecked: new Date() })
    })
    .catch(() => {
      setSharedState({ ...state, loading: false })
    })
    .finally(() => {
      inflight = null
    })

  return inflight
}

function startPolling() {
  subscribers += 1
  if (!poller) {
    void refreshSharedStatus()
    poller = setInterval(() => void refreshSharedStatus(), POLL_INTERVAL_MS)
  }
}

function stopPolling() {
  subscribers = Math.max(0, subscribers - 1)
  if (subscribers === 0 && poller) {
    clearInterval(poller)
    poller = null
  }
}

export function useSharedStatus() {
  const [current, setCurrent] = useState(state)

  useEffect(() => {
    listeners.add(setCurrent)
    startPolling()
    setCurrent(state)

    return () => {
      listeners.delete(setCurrent)
      stopPolling()
    }
  }, [])

  const refresh = useCallback(() => {
    void refreshSharedStatus()
  }, [])

  return { ...current, refresh }
}
