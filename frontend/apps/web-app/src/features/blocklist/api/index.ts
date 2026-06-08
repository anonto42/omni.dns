import { components } from '../../api-types'

export type BlockedDomain = components['schemas']['models.BlockedDomain']

const BASE = '/api'

export async function getBlocklist(): Promise<BlockedDomain[]> {
  const res = await fetch(`${BASE}/blocklist`)
  return res.json()
}

export async function addToBlocklist(domain: string, wildcard = false): Promise<void> {
  await fetch(`${BASE}/blocklist`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ domain, wildcard }),
  })
}

export async function removeFromBlocklist(domain: string): Promise<void> {
  await fetch(`${BASE}/blocklist`, {
    method: 'DELETE',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ domain }),
  })
}
