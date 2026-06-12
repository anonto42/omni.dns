import React, { useState, useEffect, useRef, useCallback } from 'react'
import { useNavigate } from 'react-router-dom'
import { Search, ArrowRight, LayoutDashboard, Server, Route as RouteIcon, Shield, ListTodo, Settings, User } from 'lucide-react'
import { cn } from '@/lib/utils'

const searchItems = [
  { path: '/', label: 'Dashboard', icon: LayoutDashboard, keywords: 'overview home stats' },
  { path: '/records', label: 'DNS Records', icon: Server, keywords: 'zone dns record domain custom' },
  { path: '/steering', label: 'Traffic Steering', icon: RouteIcon, keywords: 'route forward redirect rule policy' },
  { path: '/blocklist', label: 'Blocklist Management', icon: Shield, keywords: 'block security threat domain filter' },
  { path: '/logs', label: 'Query Log', icon: ListTodo, keywords: 'query log activity traffic history' },
  { path: '/settings', label: 'Upstream DNS', icon: Settings, keywords: 'upstream provider resolver cloudflare google' },
  { path: '/settings', label: 'Settings', icon: Settings, keywords: 'configuration preferences general' },
  { path: '/profile', label: 'Profile', icon: User, keywords: 'account profile email name' },
]

export const SearchBox: React.FC = () => {
  const navigate = useNavigate()
  const [query, setQuery] = useState('')
  const [isOpen, setIsOpen] = useState(false)
  const [selectedIndex, setSelectedIndex] = useState(0)
  const inputRef = useRef<HTMLInputElement>(null)
  const dropdownRef = useRef<HTMLDivElement>(null)

  const filtered = query.trim()
    ? searchItems.filter(item =>
        item.label.toLowerCase().includes(query.toLowerCase()) ||
        item.keywords.toLowerCase().includes(query.toLowerCase())
      )
    : searchItems

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'ArrowDown') {
      e.preventDefault()
      setSelectedIndex(i => Math.min(i + 1, filtered.length - 1))
    } else if (e.key === 'ArrowUp') {
      e.preventDefault()
      setSelectedIndex(i => Math.max(i - 1, 0))
    } else if (e.key === 'Enter' && filtered[selectedIndex]) {
      navigate(filtered[selectedIndex].path)
      setQuery('')
      setIsOpen(false)
      inputRef.current?.blur()
    } else if (e.key === 'Escape') {
      setIsOpen(false)
      inputRef.current?.blur()
    }
  }

  const selectItem = useCallback((item: typeof searchItems[number]) => {
    navigate(item.path)
    setQuery('')
    setIsOpen(false)
    inputRef.current?.blur()
  }, [navigate])

  useEffect(() => {
    setSelectedIndex(0)
  }, [query])

  useEffect(() => {
    const handleClickOutside = (e: MouseEvent) => {
      if (dropdownRef.current && !dropdownRef.current.contains(e.target as Node) &&
          inputRef.current && !inputRef.current.contains(e.target as Node)) {
        setIsOpen(false)
      }
    }
    document.addEventListener('mousedown', handleClickOutside)
    return () => document.removeEventListener('mousedown', handleClickOutside)
  }, [])

  useEffect(() => {
    const handleKeyDownGlobal = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key === 'k') {
        e.preventDefault()
        inputRef.current?.focus()
        setIsOpen(true)
      }
    }
    document.addEventListener('keydown', handleKeyDownGlobal)
    return () => document.removeEventListener('keydown', handleKeyDownGlobal)
  }, [])

  return (
    <div className="relative max-w-md w-full hidden md:block group">
      <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-5 w-5 text-muted-foreground/70 group-focus-within:text-primary transition-colors" />
      <input
        ref={inputRef}
        value={query}
        onChange={e => { setQuery(e.target.value); setIsOpen(true) }}
        onFocus={() => setIsOpen(true)}
        onKeyDown={handleKeyDown}
        className="w-full bg-muted pl-9 pr-10 py-2 text-sm text-foreground placeholder:italic placeholder:text-muted-foreground/70 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary/50 focus-visible:bg-background disabled:cursor-not-allowed disabled:opacity-50 transition-all duration-200"
        placeholder="Search features, DNS records, settings..."
        type="text"
      />
      <div className="absolute right-3 top-1/2 -translate-y-1/2 pointer-events-none hidden sm:flex items-center gap-0.5 select-none bg-background px-1.5 font-mono text-[9px] font-bold text-muted-foreground/80">
        <span>⌘</span><span>K</span>
      </div>

      {isOpen && (
        <div
          ref={dropdownRef}
          className="absolute top-full left-0 right-0 mt-2 bg-popover shadow-lg overflow-hidden z-50"
        >
          <div className="p-2 bg-muted/30">
            <p className="text-[10px] font-bold uppercase tracking-widest text-muted-foreground">
              {filtered.length} {filtered.length === 1 ? 'result' : 'results'} found
            </p>
          </div>
          <div className="max-h-72 overflow-y-auto p-1">
            {filtered.length === 0 ? (
              <div className="p-4 text-center text-sm text-muted-foreground">
                No results for "<span className="font-medium text-foreground">{query}</span>"
              </div>
            ) : (
              filtered.map((item, i) => {
                const Icon = item.icon
                return (
                  <button
                    key={`${item.path}-${item.label}`}
                    onMouseDown={e => { e.preventDefault(); selectItem(item) }}
                    onMouseEnter={() => setSelectedIndex(i)}
                    className={cn(
                      "w-full flex items-center gap-3 px-3 py-2.5 rounded-md text-left transition-colors",
                      i === selectedIndex
                        ? "bg-primary/10 text-primary"
                        : "text-foreground hover:bg-muted/50"
                    )}
                  >
                    <Icon className="h-4 w-4 shrink-0" />
                    <div className="flex-1 min-w-0">
                      <p className="text-sm font-medium truncate">{item.label}</p>
                    </div>
                    <ArrowRight className={cn(
                      "h-3.5 w-3.5 text-muted-foreground transition-opacity",
                      i === selectedIndex ? "opacity-100" : "opacity-0"
                    )} />
                  </button>
                )
              })
            )}
          </div>
          <div className="p-2 bg-muted/30 flex items-center gap-3 text-[9px] font-bold uppercase tracking-widest text-muted-foreground">
            <span>↑↓ <span className="text-foreground">Navigate</span></span>
            <span>↵ <span className="text-foreground">Open</span></span>
            <span>esc <span className="text-foreground">Close</span></span>
          </div>
        </div>
      )}
    </div>
  )
}
export default SearchBox
