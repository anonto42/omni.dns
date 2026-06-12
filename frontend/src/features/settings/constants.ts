import { Sun, Moon, Monitor } from 'lucide-react'
import type { Theme } from '@/hooks/useTheme'

// ── Upstream DNS Options ─────────────────────────────────────────────────

export const UPSTREAM_OPTIONS = [
  { label: 'Cloudflare (1.1.1.1) — DoT', value: '1.1.1.1:853' },
  { label: 'Google (8.8.8.8) — DoT', value: '8.8.8.8:853' },
  { label: 'Quad9 (9.9.9.9) — DoT', value: '9.9.9.9:853' },
]

// ── Theme Options ────────────────────────────────────────────────────────

export const THEME_OPTIONS: { label: string; value: Theme; icon: React.ElementType }[] = [
  { label: 'Light', value: 'light', icon: Sun },
  { label: 'Dark', value: 'dark', icon: Moon },
  { label: 'System', value: 'system', icon: Monitor },
]
