import { components } from '../../api-types'

export type QueryLog = components['schemas']['models.QueryLog']

const BASE = '/api'

export async function getLogs(): Promise<QueryLog[]> {
  const res = await fetch(`${BASE}/logs`)
  return res.json()
}

export async function clearLogs(): Promise<void> {
  await fetch(`${BASE}/logs`, { method: 'DELETE' })
}
