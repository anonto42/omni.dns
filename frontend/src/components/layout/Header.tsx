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
  ArrowRight,
  Sun,
  Moon,
  Monitor,
  X,
  Info,
  Trash2,
} from 'lucide-react';
import { useTheme, type Theme } from '../../hooks/useTheme';
import { useAuth } from '../../hooks/useAuth';
import { useLayout } from '../../hooks/useLayout';
import { Button } from '@/components/ui/button';
import { cn } from '@/lib/utils';
import {
  getNotifications,
  markAllAsRead,
  clearAllNotifications,
  deleteNotification,
  markAsRead,
  SystemNotification
} from '@/lib/notifications';
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
]

const THEME_CYCLE: Theme[] = ['light', 'dark', 'system']
const THEME_ICONS: Record<Theme, React.ElementType> = { light: Sun, dark: Moon, system: Monitor }
const THEME_LABELS: Record<Theme, string> = { light: 'Light', dark: 'Dark', system: 'System' }

export const Header: React.FC = () => {
  const navigate = useNavigate();
  const { isSidebarCollapsed } = useLayout();
  const { theme, setTheme } = useTheme();
  const { user, logout } = useAuth();
  const [notifications, setNotifications] = useState<SystemNotification[]>(getNotifications())

  useEffect(() => {
    const handleUpdate = () => {
      setNotifications(getNotifications())
    }
    window.addEventListener('netshield_notifications_update', handleUpdate)
    return () => window.removeEventListener('netshield_notifications_update', handleUpdate)
  }, [])

  const unreadCount = notifications.filter(n => !n.read).length

  const formatTime = (isoString: string): string => {
    const diff = Date.now() - new Date(isoString).getTime()
    const mins = Math.floor(diff / 60000)
    if (mins < 1) return 'Just now'
    if (mins < 60) return `${mins}m ago`
    const hours = Math.floor(mins / 60)
    if (hours < 24) return `${hours}h ago`
    return new Date(isoString).toLocaleDateString(undefined, { month: 'short', day: 'numeric' })
  }

  const cycleTheme = () => {
    const next = THEME_CYCLE[(THEME_CYCLE.indexOf(theme) + 1) % THEME_CYCLE.length]
    setTheme(next)
  }
  const ThemeIcon = THEME_ICONS[theme]
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
        "h-16 fixed top-0 right-0 bg-card flex justify-between items-center px-4 md:px-6 z-40 transition-all duration-300 shadow-sm",
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
          <SheetContent side="left" className="p-0 w-[280px]">
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
      </div>

      <div className="flex items-center gap-2 md:gap-4">
        <Button
          variant="ghost"
          size="icon"
          onClick={cycleTheme}
          title={`Theme: ${THEME_LABELS[theme]}`}
          className="h-9 w-9 text-muted-foreground hover:text-foreground hover:bg-muted"
        >
          <ThemeIcon className="h-4 w-4" />
        </Button>

        <div className="flex items-center gap-1">
          <Popover>
            <PopoverTrigger asChild>
              <Button variant="ghost" size="icon" className="relative h-9 w-9 text-muted-foreground hover:text-foreground hover:scale-105 active:scale-95 transition-transform cursor-pointer">
                <Bell className="h-5 w-5" />
                {unreadCount > 0 && (
                  <span className="absolute top-2 right-2 flex h-2 w-2">
                    <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-destructive opacity-75"></span>
                    <span className="relative inline-flex rounded-full h-2 w-2 bg-destructive"></span>
                  </span>
                )}
              </Button>
            </PopoverTrigger>
            <PopoverContent className="w-80 p-0 mt-2 shadow-lg glass-panel border-border" align="end">
              <div className="p-3 bg-muted/20 flex items-center justify-between border-b border-border">
                <h4 className="font-bold text-[10px] uppercase tracking-widest text-foreground">Notifications ({unreadCount})</h4>
                <div className="flex gap-2">
                  {unreadCount > 0 && (
                    <Button variant="ghost" size="sm" className="h-auto p-0 text-[9px] font-bold uppercase tracking-widest text-primary hover:underline cursor-pointer" onClick={() => markAllAsRead()}>Mark read</Button>
                  )}
                  {notifications.length > 0 && (
                    <Button variant="ghost" size="sm" className="h-auto p-0 text-[9px] font-bold uppercase tracking-widest text-destructive hover:underline cursor-pointer" onClick={() => clearAllNotifications()}>Clear all</Button>
                  )}
                </div>
              </div>
              <div className="max-h-[300px] overflow-y-auto divide-y divide-border/40">
                {notifications.length === 0 ? (
                  <div className="p-8 text-center text-muted-foreground text-[10px] font-bold uppercase tracking-widest">
                    No notifications
                  </div>
                ) : (
                  notifications.map(n => {
                    const Icon = n.type === 'success' ? CheckCircle2 : (n.type === 'warning' ? AlertCircle : Info)
                    const iconBg = n.type === 'success' ? 'bg-emerald-500/10 text-emerald-500' : (n.type === 'warning' ? 'bg-destructive/10 text-destructive' : 'bg-primary/10 text-primary')
                    return (
                      <div
                        key={n.id}
                        onClick={() => markAsRead(n.id)}
                        className={`p-3 flex gap-3 hover:bg-muted/40 transition-colors cursor-pointer relative group ${!n.read ? 'bg-primary/5' : ''}`}
                      >
                        <div className={`h-8 w-8 ${iconBg} flex items-center justify-center shrink-0 rounded-lg`}>
                          <Icon className="h-4 w-4" />
                        </div>
                        <div className="space-y-0.5 pr-6 flex-1">
                          <p className={`text-xs font-bold leading-tight ${!n.read ? 'text-foreground font-extrabold' : 'text-foreground/85'}`}>{n.title}</p>
                          <p className="text-[11px] text-muted-foreground leading-normal">{n.description}</p>
                          <p className="text-[9px] text-muted-foreground/60 font-bold uppercase tracking-wider">{formatTime(n.timestamp)}</p>
                        </div>
                        <button
                          onClick={(e) => {
                            e.stopPropagation()
                            deleteNotification(n.id)
                          }}
                          className="absolute right-2.5 top-1/2 -translate-y-1/2 opacity-0 group-hover:opacity-100 hover:text-destructive text-muted-foreground transition-all p-1 cursor-pointer hover:bg-muted/50 rounded"
                          title="Delete notification"
                        >
                          <Trash2 className="h-3.5 w-3.5" />
                        </button>
                      </div>
                    )
                  })
                )}
              </div>
            </PopoverContent>
          </Popover>
        </div>

        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button variant="ghost" className="flex items-center gap-2 px-2 hover:bg-muted h-9 transition-colors">
              <div className="h-7 w-7 bg-primary/15 flex items-center justify-center text-primary">
                <UserCircle className="h-5 w-5" />
              </div>
              <ChevronDown className="h-3.5 w-3.5 text-muted-foreground hidden md:block" />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end" className="w-56 mt-2 shadow-md">
            <DropdownMenuLabel className="font-normal bg-muted/20">
              <div className="flex flex-col space-y-1">
                <p className="text-sm font-bold text-foreground leading-none">{user?.name || 'Administrator'}</p>
                <p className="text-[10px] font-bold text-muted-foreground uppercase tracking-widest leading-none">{user?.email || 'admin@netshield.local'}</p>
              </div>
            </DropdownMenuLabel>
            <DropdownMenuSeparator className="bg-muted" />
            <DropdownMenuItem onClick={() => navigate('/profile')} className="cursor-pointer gap-2 text-xs font-bold uppercase tracking-widest text-muted-foreground hover:text-foreground focus:text-foreground">
              <User className="h-3.5 w-3.5" /> Profile
            </DropdownMenuItem>
            <DropdownMenuItem onClick={() => navigate('/settings')} className="cursor-pointer gap-2 text-xs font-bold uppercase tracking-widest text-muted-foreground hover:text-foreground focus:text-foreground">
              <Settings className="h-3.5 w-3.5" /> Settings
            </DropdownMenuItem>
            <DropdownMenuSeparator className="bg-muted" />
            <DropdownMenuItem
              className="text-destructive cursor-pointer gap-2 focus:bg-destructive/10 focus:text-destructive text-xs font-bold uppercase tracking-widest"
              onClick={async () => {
                const token = localStorage.getItem('auth_token')
                if (token) {
                  await fetch('/api/session', {
                    method: 'DELETE',
                    headers: { Authorization: `Bearer ${token}` },
                  }).catch(() => {})
                }
                logout()
                navigate('/login')
              }}
            >
              <LogOut className="h-3.5 w-3.5" /> Sign out
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      </div>
    </header>
  );
};


