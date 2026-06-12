import type { ReactNode } from 'react'

interface PageTransitionProps {
  children: ReactNode
}

/** Page wrapper that applies a subtle enter animation on mount. */
export function PageTransition({ children }: PageTransitionProps) {
  return (
    <div className="animate-in fade-in slide-in-from-bottom-3 duration-300">
      {children}
    </div>
  )
}
