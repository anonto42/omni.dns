import { components } from '../../api-types'

export type Status = components['schemas']['models.Stats']

const BASE = '/api'

export async function getStatus(): Promise<Status> {
  const res = await fetch(`${BASE}/status`)
  return res.json()
}
