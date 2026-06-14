import { lazy, Suspense, useEffect } from 'react'
import { Routes, Route, useLocation } from 'react-router-dom'
import { Toaster } from 'sonner'
import { TourProvider } from './contexts/TourContext'
import { DashboardLayout } from './components/layout/DashboardLayout'
import { ProtectedRoute } from './components/shared/ProtectedRoute'

const LoginPage = lazy(() => import('./pages/LoginPage'))
const DashboardPage = lazy(() => import('./pages/DashboardPage'))
const LogsPage = lazy(() => import('./pages/LogsPage'))
const RecordsPage = lazy(() => import('./pages/RecordsPage'))
const BlocklistPage = lazy(() => import('./pages/BlocklistPage'))
const SteeringPage = lazy(() => import('./pages/SteeringPage'))
const SettingsPage = lazy(() => import('./pages/SettingsPage'))
const ProfilePage = lazy(() => import('./pages/ProfilePage'))

// ── Dynamic Metadata Updater ──────────────────────────────────────────────
function MetadataUpdater() {
  const location = useLocation()

  useEffect(() => {
    const titleMap: Record<string, string> = {
      '/': 'Overview | OmniDNS',
      '/logs': 'Query Logs | OmniDNS',
      '/records': 'DNS Records | OmniDNS',
      '/blocklist': 'Blocklist | OmniDNS',
      '/steering': 'Traffic Steering | OmniDNS',
      '/settings': 'Settings | OmniDNS',
      '/profile': 'Profile | OmniDNS',
      '/login': 'Sign In | OmniDNS',
    }
    const currentTitle = titleMap[location.pathname] || 'OmniDNS'
    document.title = currentTitle

    let metaDesc = document.querySelector('meta[name="description"]')
    if (!metaDesc) {
      metaDesc = document.createElement('meta')
      metaDesc.setAttribute('name', 'description')
      document.head.appendChild(metaDesc)
    }
    metaDesc.setAttribute('content', `OmniDNS - ${currentTitle}`)
  }, [location.pathname])

  return null
}

// ── Route Definitions ────────────────────────────────────────────────────

/** Animated route container — re-triggers on path change. */
function AnimatedRoutes() {
  const location = useLocation()
  return (
    <Routes location={location}>
      <Route path="/" element={<DashboardPage />} />
      <Route path="/logs" element={<LogsPage />} />
      <Route path="/records" element={<RecordsPage />} />
      <Route path="/blocklist" element={<BlocklistPage />} />
      <Route path="/steering" element={<SteeringPage />} />
      <Route path="/settings" element={<SettingsPage />} />
      <Route path="/profile" element={<ProfilePage />} />
      <Route path="*" element={<DashboardPage />} />
    </Routes>
  )
}

function RouteFallback() {
  return <div className="min-h-[240px]" aria-busy="true" aria-label="Loading page" />
}

// ── App Root ──────────────────────────────────────────────────────────────

export default function App() {
  return (
    <>
      <MetadataUpdater />
      <Toaster
        position="bottom-right"
        toastOptions={{
          classNames: {
            toast: 'bg-card text-foreground shadow-lg',
            title: 'text-sm font-bold',
            description: 'text-xs text-muted-foreground',
            success: 'border-emerald-500/30',
            error: 'border-destructive/30',
            warning: 'border-amber-500/30',
          },
        }}
      />
      <Suspense fallback={<RouteFallback />}>
        <Routes>
          <Route path="/login" element={<LoginPage />} />
          <Route path="/*" element={
            <ProtectedRoute>
              <TourProvider>
                <DashboardLayout>
                  <AnimatedRoutes />
                </DashboardLayout>
              </TourProvider>
            </ProtectedRoute>
          } />
        </Routes>
      </Suspense>
    </>
  )
}
