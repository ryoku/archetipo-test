import { Navigate } from 'react-router-dom'
import { useAuth } from './AuthContext'

export function ProtectedRoute({ children }: { children: React.ReactNode }) {
  const { isAuthenticated, loading } = useAuth()

  if (loading) return <p>Loading…</p>
  if (!isAuthenticated) return <Navigate to="/login" replace />
  return <>{children}</>
}
