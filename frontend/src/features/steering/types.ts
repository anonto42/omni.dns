// ── Steering Rule Types ──────────────────────────────────────────────────

export type SteeringRule = {
  id: number
  name: string
  condition_type: string
  condition_value: string
  action_type: string
  action_target: string
  priority: number
  enabled: boolean
}

// ── Constants ────────────────────────────────────────────────────────────

export const CONDITION_PLACEHOLDERS: Record<string, string> = {
  'Domain':     '*.corp.internal',
  'Client IP':  '192.168.1.0/24',
  'Query Type': 'A, AAAA',
  'Time Range': '09:00-18:00',
}

export const ACTION_COLORS: Record<string, string> = {
  'Forward':  'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400',
  'Block':    'bg-rose-500/10 text-rose-600 dark:text-rose-400',
  'Redirect': 'bg-amber-500/10 text-amber-600 dark:text-amber-400',
}
