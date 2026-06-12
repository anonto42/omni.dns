import type React from 'react'
import type { TourStep } from './tourSteps'

export function getPopoverStyle(
  targetRect: DOMRect | null,
  currentStep: TourStep | null
): React.CSSProperties {
  const POPOVER_W = Math.min(380, window.innerWidth - 32)
  const POPOVER_H = 300
  const GAP = 14
  const EDGE = 16

  if (window.innerWidth < 600) {
    return {
      position: 'fixed',
      bottom: `${EDGE}px`,
      left: `${EDGE}px`,
      right: `${EDGE}px`,
      zIndex: 9999,
    }
  }

  const style: React.CSSProperties = {
    position: 'fixed',
    zIndex: 9999,
    width: `${POPOVER_W}px`,
  }

  if (!targetRect) {
    style.top = '50%'
    style.left = '50%'
    style.transform = 'translate(-50%, -50%)'
    return style
  }

  let placement = currentStep?.placement ?? 'bottom'
  const elementCenterX = targetRect.left + targetRect.width / 2
  const rawLeft = elementCenterX - POPOVER_W / 2
  const clampedLeft = Math.max(EDGE, Math.min(window.innerWidth - POPOVER_W - EDGE, rawLeft))

  const spaceBelow = window.innerHeight - targetRect.bottom
  const spaceAbove = targetRect.top
  const spaceRight = window.innerWidth - targetRect.right
  const spaceLeft = targetRect.left

  if (placement === 'bottom' && spaceBelow < POPOVER_H + GAP && spaceAbove > spaceBelow) {
    placement = 'top'
  } else if (placement === 'top' && spaceAbove < POPOVER_H + GAP && spaceBelow > spaceAbove) {
    placement = 'bottom'
  } else if (placement === 'right' && spaceRight < POPOVER_W + GAP && spaceLeft > spaceRight) {
    placement = 'left'
  } else if (placement === 'left' && spaceLeft < POPOVER_W + GAP && spaceRight > spaceLeft) {
    placement = 'right'
  }

  if (placement === 'bottom') {
    const rawTop = targetRect.bottom + GAP
    style.top = `${Math.min(rawTop, window.innerHeight - POPOVER_H - EDGE)}px`
    style.left = `${clampedLeft}px`
  } else if (placement === 'top') {
    const clampedTop = Math.max(EDGE, targetRect.top - GAP - POPOVER_H)
    style.top = `${clampedTop}px`
    style.left = `${clampedLeft}px`
  } else if (placement === 'right') {
    const rawLeft2 = targetRect.right + GAP
    style.left = `${Math.min(rawLeft2, window.innerWidth - POPOVER_W - EDGE)}px`
    style.top = `${Math.max(EDGE, Math.min(window.innerHeight - POPOVER_H - EDGE, targetRect.top + targetRect.height / 2 - POPOVER_H / 2))}px`
  } else if (placement === 'left') {
    const rawRight = window.innerWidth - targetRect.left + GAP
    style.right = `${Math.min(rawRight, window.innerWidth - POPOVER_W - EDGE)}px`
    style.top = `${Math.max(EDGE, Math.min(window.innerHeight - POPOVER_H - EDGE, targetRect.top + targetRect.height / 2 - POPOVER_H / 2))}px`
  } else {
    style.top = '50%'
    style.left = '50%'
    style.transform = 'translate(-50%, -50%)'
  }

  return style
}

export function getSpotlightStyle(targetRect: DOMRect | null): React.CSSProperties | null {
  if (!targetRect) return null
  const pad = 6
  return {
    position: 'fixed',
    top: targetRect.top - pad,
    left: targetRect.left - pad,
    width: targetRect.width + pad * 2,
    height: targetRect.height + pad * 2,
    borderRadius: '4px',
    zIndex: 9991,
    boxShadow: '0 0 0 9999px rgba(0,0,0,0.65)',
    border: '2px solid oklch(0.68 0.16 200)',
    outline: '2px solid oklch(0.68 0.16 200 / 0.3)',
    outlineOffset: '3px',
  }
}
