import React, { useState, useEffect, useCallback } from 'react';
import { useNavigate } from 'react-router-dom';
import {
  Bell,
  UserCircle,
  ChevronDown,
  Menu,
  CheckCircle2,
  AlertCircle,
  Settings,
  LogOut,
  User,
  Sun,
  Moon,
  Monitor,
  Info,
  Trash2,
} from 'lucide-react';
import { useTheme, type Theme } from '../../hooks/useTheme';
import { useAuth } from '@/features/auth';
import { useLayout } from '../../hooks/useLayout';
import { Button } from '@/components/ui/button';
import { cn } from '@/lib/utils';
import { SearchBox } from './SearchBox';
import {
  fetchNotifications,
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

const THEME_CYCLE: Theme[] = ['light', 'dark', 'system']
const THEME_ICONS: Record<Theme, React.ElementType> = { light: Sun, dark: Moon, system: Monitor }
const THEME_LABELS: Record<Theme, string> = { light: 'Light', dark: 'Dark', system: 'System' }

export const Header: React.FC = () => {
  const navigate = useNavigate();
  const { isSidebarCollapsed } = useLayout();
  const { theme, setTheme } = useTheme();
  const { user, logout } = useAuth();
  const [notifications, setNotifications] = useState<SystemNotification[]>([])

  const loadNotifications = useCallback(async () => {
    const list = await fetchNotifications()
    setNotifications(list)
  }, [])

  useEffect(() => {
    loadNotifications()
    window.addEventListener('omnidns_notifications_update', loadNotifications)
    const interval = setInterval(loadNotifications, 10000)

    return () => {
      window.removeEventListener('omnidns_notifications_update', loadNotifications)
      clearInterval(interval)
    }
  }, [loadNotifications])

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

  return (
    <header 
      className={cn(
        "h-16 fixed top-0 right-0 left-0 bg-card flex justify-between items-center px-3 sm:px-4 md:px-6 z-40 transition-all duration-300 shadow-sm border-b border-border",
        isSidebarCollapsed ? 'md:left-[72px]' : 'md:left-64'
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
        
        <SearchBox />
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
              <Button variant="ghost" size="icon" data-tour="notification-bell" className="relative h-9 w-9 text-muted-foreground hover:text-foreground hover:scale-105 active:scale-95 transition-transform cursor-pointer">
                <Bell className="h-5 w-5" />
                {unreadCount > 0 && (
                  <span className="absolute top-2 right-2 flex h-2 w-2">
                    <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-destructive opacity-75"></span>
                    <span className="relative inline-flex rounded-full h-2 w-2 bg-destructive"></span>
                  </span>
                )}
              </Button>
            </PopoverTrigger>
            <PopoverContent className="w-[calc(100vw-32px)] sm:w-80 p-0 mt-2 shadow-lg glass-panel border-border" align="end">
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
                          <p className="text-[11px] text-muted-foreground leading-normal">{n.message}</p>
                          <p className="text-[9px] text-muted-foreground/60 font-bold uppercase tracking-wider">{formatTime(n.created_at)}</p>
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
                <p className="text-[10px] font-bold text-muted-foreground uppercase tracking-widest leading-none">{user?.email || 'admin@omnidns.local'}</p>
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
