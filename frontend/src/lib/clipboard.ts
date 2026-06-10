import { toast } from 'sonner'

export function copyToClipboard(text: string): void {
  if (navigator.clipboard?.writeText) {
    navigator.clipboard.writeText(text).then(
      () => toast.success('Copied to clipboard', { description: text }),
      () => fallbackCopy(text)
    )
  } else {
    fallbackCopy(text)
  }
}

function fallbackCopy(text: string): void {
  const el = document.createElement('textarea')
  el.value = text
  el.style.cssText = 'position:fixed;top:0;left:0;opacity:0'
  document.body.appendChild(el)
  el.focus()
  el.select()
  try {
    document.execCommand('copy')
    toast.success('Copied to clipboard', { description: text })
  } catch {
    toast.error('Copy failed', { description: 'Please copy manually.' })
  }
  document.body.removeChild(el)
}
