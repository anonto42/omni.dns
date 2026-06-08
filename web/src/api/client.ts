const BASE = '/api'

export interface Status {
  queries_forwarded: number
  queries_blocked: number
  queries_custom: number
  queries_cached: number
  cache_size: number
  cache_hits: number
  cache_misses: number
  uptime_seconds: number
}

export interface QueryLog {
  id: number
  timestamp: string
  domain: string
  client_ip: string
  action: 'forwarded' | 'blocked' | 'custom' | 'cached'
}

export async function getStatus(): Promise<Status> {
  const res = await fetch(`${BASE}/status`)
  return res.json()
}

export async function getLogs(): Promise<QueryLog[]> {
  const res = await fetch(`${BASE}/logs`)
  return res.json()
}

export async function clearLogs(): Promise<void> {
  await fetch(`${BASE}/logs`, { method: 'DELETE' })
}

export async function getRecords(): Promise<Record<string, string>> {
  const res = await fetch(`${BASE}/records`)
  return res.json()
}

export async function addRecord(domain: string, ip: string): Promise<void> {
  await fetch(`${BASE}/records`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ domain, ip }),
  })
}

export async function deleteRecord(domain: string): Promise<void> {
  await fetch(`${BASE}/records`, {
    method: 'DELETE',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ domain }),
  })
}

export interface BlockedDomain {
  domain: string
  added_at: string
  wildcard: boolean
}

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
