import { Routes, Route, useLocation, Navigate } from 'react-router-dom'
import { Toaster } from 'sonner'
import { useAuth } from './hooks/useAuth'
import { TourProvider } from './contexts/TourContext'
import { DashboardLayout } from './components/layout/DashboardLayout'
import { ProtectedRoute } from './components/shared/ProtectedRoute'
import LoginPage from './pages/LoginPage'

// Feature page components — one per route
import { DashboardPage } from './features/dashboard'
import { LogsPage } from './features/logs'
import { RecordManager } from './features/records'
import { BlocklistManager } from './features/blocklist'
import { SteeringPage } from './features/steering'
import { SettingsPage } from './features/settings'
import { ProfilePage } from './features/profile'

// ── Route Definitions ────────────────────────────────────────────────────

/** Animated route container — re-triggers on path change. */
function AnimatedRoutes() {
  const location = useLocation()
  return (
    <Routes location={location}>
      <Route path="/" element={<DashboardPage />} />
      <Route path="/logs" element={<LogsPage />} />
      <Route path="/records" element={<RecordManager />} />
      <Route path="/blocklist" element={<BlocklistManager />} />
      <Route path="/steering" element={<SteeringPage />} />
      <Route path="/settings" element={<SettingsPage />} />
      <Route path="/profile" element={<ProfilePage />} />
      <Route path="*" element={<DashboardPage />} />
    </Routes>
  )
}

// ── App Root ──────────────────────────────────────────────────────────────

export default function App() {
  return (
    <>
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
    </>
  )
}
