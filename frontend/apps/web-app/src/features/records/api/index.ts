const BASE = '/api'

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
