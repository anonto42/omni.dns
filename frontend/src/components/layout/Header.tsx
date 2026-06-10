import React, { useState, useRef, useEffect, useCallback } from 'react';
import { useNavigate } from 'react-router-dom';
import { 
  Search, 
  Bell, 
  Cloud, 
  UserCircle, 
  ChevronDown,
  Menu,
  CheckCircle2,
  AlertCircle,
  Settings,
  LogOut,
  User,
  LayoutDashboard,
  Server,
  Route as RouteIcon,
  Shield,
  ListTodo,
  ArrowRight
} from 'lucide-react';
import { useLayout } from '../../hooks/useLayout';
import { Button } from '@/components/ui/button';
import { cn } from '@/lib/utils';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from '@/components/ui/popover';
import {
  Sheet,
  SheetContent,
  SheetTrigger,
} from '@/components/ui/sheet';
import { SidebarContent } from './Sidebar';

const searchItems = [
  { path: '/', label: 'Dashboard', icon: LayoutDashboard, keywords: 'overview home stats' },
  { path: '/records', label: 'DNS Records', icon: Server, keywords: 'zone dns record domain custom' },
  { path: '/steering', label: 'Traffic Steering', icon: RouteIcon, keywords: 'route forward redirect rule policy' },
  { path: '/blocklist', label: 'Blocklist Management', icon: Shield, keywords: 'block security threat domain filter' },
  { path: '/logs', label: 'Query Log', icon: ListTodo, keywords: 'query log activity traffic history' },
  { path: '/settings', label: 'Upstream DNS', icon: Settings, keywords: 'upstream provider resolver cloudflare google' },
  { path: '/settings', label: 'Settings', icon: Settings, keywords: 'configuration preferences general' },
  { path: '/profile', label: 'Profile', icon: User, keywords: 'account profile email name' },
  { path: '/cloud-sync', label: 'Cloud Sync', icon: Cloud, keywords: 'sync cloud backup cluster node' },
]

export const Header: React.FC = () => {
  const navigate = useNavigate();
  const { isSidebarCollapsed } = useLayout();
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
    const handleKeyDown = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key === 'k') {
        e.preventDefault()
        inputRef.current?.focus()
        setIsOpen(true)
      }
    }
    document.addEventListener('keydown', handleKeyDown)
    return () => document.removeEventListener('keydown', handleKeyDown)
  }, [])

  return (
    <header 
      className={cn(
        "h-16 fixed top-0 right-0 border-b border-border/50 bg-background/95 backdrop-blur supports-[backdrop-filter]:bg-background/60 flex justify-between items-center px-4 md:px-6 z-40 transition-all duration-300 shadow-sm",
        isSidebarCollapsed ? 'w-full md:w-[calc(100%-72px)]' : 'w-full md:w-[calc(100%-256px)]'
      )}
    >
      <div className="flex items-center gap-2 md:gap-4 flex-1">
        <Sheet>
          <SheetTrigger asChild>
            <Button 
              variant="ghost" 
              size="icon" 
              className="md:hidden text-muted-foreground hover:text-foreground" 
            >
              <Menu className="h-5 w-5" />
            </Button>
          </SheetTrigger>
          <SheetContent side="left" className="p-0 w-[280px] border-r border-border/50">
            <SidebarContent />
          </SheetContent>
        </Sheet>
        
        <div className="relative max-w-md w-full hidden md:block group">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-5 w-5 text-muted-foreground/70 group-focus-within:text-primary transition-colors" />
          <input
            ref={inputRef}
            value={query}
            onChange={e => { setQuery(e.target.value); setIsOpen(true) }}
            onFocus={() => setIsOpen(true)}
            onKeyDown={handleKeyDown}
            className="w-full bg-card border border-primary/20 rounded-lg pl-9 pr-10 py-2 text-sm text-foreground placeholder:italic placeholder:text-muted-foreground/70 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary/50 focus-visible:bg-background/90 disabled:cursor-not-allowed disabled:opacity-50 transition-all duration-200" 
            placeholder="Search features, DNS records, settings..." 
            type="text"
          />
          <div className="absolute right-3 top-1/2 -translate-y-1/2 pointer-events-none hidden sm:flex items-center gap-0.5 select-none rounded border border-border/70 bg-muted px-1.5 font-mono text-[9px] font-bold text-muted-foreground/80">
            <span>⌘</span><span>K</span>
          </div>

          {isOpen && (
            <div
              ref={dropdownRef}
              className="absolute top-full left-0 right-0 mt-3 bg-popover border border-border/50 rounded-lg shadow-lg overflow-hidden z-50"
            >
              <div className="p-2 border-b border-border/50 bg-muted/5">
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
              <div className="p-2 border-t border-border/50 bg-muted/5 flex items-center gap-3 text-[9px] font-bold uppercase tracking-widest text-muted-foreground">
                <span>↑↓ <span className="text-foreground">Navigate</span></span>
                <span>↵ <span className="text-foreground">Open</span></span>
                <span>esc <span className="text-foreground">Close</span></span>
              </div>
            </div>
          )}
        </div>
      </div>

      <div className="flex items-center gap-2 md:gap-4">
        <div className="flex items-center gap-1">
          <Popover>
            <PopoverTrigger asChild>
              <Button variant="ghost" size="icon" className="relative h-9 w-9 text-muted-foreground hover:text-foreground hover:scale-105 active:scale-95 transition-transform">
                <Bell className="h-5 w-5" />
                <span className="absolute top-2 right-2 flex h-2 w-2">
                  <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-destructive opacity-75"></span>
                  <span className="relative inline-flex rounded-full h-2 w-2 bg-destructive"></span>
                </span>
              </Button>
            </PopoverTrigger>
            <PopoverContent className="w-80 p-0 mt-2 border-border/50 shadow-lg" align="end">
              <div className="p-3 border-b border-border/50 flex items-center justify-between bg-muted/5">
                <h4 className="font-bold text-[10px] uppercase tracking-widest text-foreground">Notifications</h4>
                <Button variant="ghost" size="sm" className="h-auto p-0 text-[9px] font-bold uppercase tracking-widest text-muted-foreground hover:text-primary">Mark all as read</Button>
              </div>
              <div className="max-h-[300px] overflow-y-auto">
                <div className="p-3.5 flex gap-3 hover:bg-muted/30 transition-colors cursor-pointer border-b border-border/50">
                  <div className="h-8 w-8 rounded-full bg-destructive/10 flex items-center justify-center shrink-0 border border-destructive/20">
                    <AlertCircle className="h-4 w-4 text-destructive" />
                  </div>
                  <div className="space-y-1">
                    <p className="text-xs font-bold text-foreground leading-none">High Traffic Alert</p>
                    <p className="text-xs text-muted-foreground leading-snug">Anomaly detected in north-america-east-1 cluster.</p>
                    <p className="text-[9px] text-muted-foreground/70 font-bold uppercase">2 mins ago</p>
                  </div>
                </div>
                <div className="p-3.5 flex gap-3 hover:bg-muted/30 transition-colors cursor-pointer">
                  <div className="h-8 w-8 rounded-full bg-emerald-500/10 flex items-center justify-center shrink-0 border border-emerald-500/20">
                    <CheckCircle2 className="h-4 w-4 text-emerald-500" />
                  </div>
                  <div className="space-y-1">
                    <p className="text-xs font-bold text-foreground leading-none">Sync Complete</p>
                    <p className="text-xs text-muted-foreground leading-snug">All blocklists updated successfully.</p>
                    <p className="text-[9px] text-muted-foreground/70 font-bold uppercase">1 hour ago</p>
                  </div>
                </div>
              </div>
              <div className="p-2 border-t border-border/50 bg-muted/5">
                <Button variant="ghost" className="w-full text-[10px] font-bold uppercase tracking-widest text-primary justify-center h-8">View all notifications</Button>
              </div>
            </PopoverContent>
          </Popover>
        </div>

        <div className="h-6 w-[1px] bg-border/50 mx-1 hidden md:block"></div>

        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button variant="ghost" className="flex items-center gap-2 px-2 hover:bg-muted/50 h-9 transition-colors border border-transparent hover:border-border/30 rounded-lg">
              <div className="h-7 w-7 rounded-full bg-primary/5 flex items-center justify-center text-primary border border-primary/10">
                <UserCircle className="h-5 w-5" />
              </div>
              <ChevronDown className="h-3.5 w-3.5 text-muted-foreground hidden md:block" />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end" className="w-56 mt-2 border-border/50 shadow-md">
            <DropdownMenuLabel className="font-normal bg-muted/5">
              <div className="flex flex-col space-y-1">
                <p className="text-sm font-bold text-foreground leading-none">Enterprise User</p>
                <p className="text-[10px] font-bold text-muted-foreground uppercase tracking-widest leading-none">admin@netshield.local</p>
              </div>
            </DropdownMenuLabel>
            <DropdownMenuSeparator className="bg-border/50" />
            <DropdownMenuItem onClick={() => navigate('/profile')} className="cursor-pointer gap-2 text-xs font-bold uppercase tracking-widest text-muted-foreground hover:text-foreground focus:text-foreground">
              <User className="h-3.5 w-3.5" /> Profile
            </DropdownMenuItem>
            <DropdownMenuItem onClick={() => navigate('/settings')} className="cursor-pointer gap-2 text-xs font-bold uppercase tracking-widest text-muted-foreground hover:text-foreground focus:text-foreground">
              <Settings className="h-3.5 w-3.5" /> Settings
            </DropdownMenuItem>
            <DropdownMenuItem onClick={() => navigate('/cloud-sync')} className="cursor-pointer gap-2 text-xs font-bold uppercase tracking-widest text-muted-foreground hover:text-foreground focus:text-foreground">
              <Cloud className="h-3.5 w-3.5" /> Cloud Sync
            </DropdownMenuItem>
            <DropdownMenuSeparator className="bg-border/50" />
            <DropdownMenuItem className="text-destructive cursor-pointer gap-2 focus:bg-destructive/10 focus:text-destructive text-xs font-bold uppercase tracking-widest">
              <LogOut className="h-3.5 w-3.5" /> Sign out
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      </div>
    </header>
  );
};


