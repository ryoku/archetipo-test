import { type ReactNode } from 'react'
import { Navigate } from 'react-router-dom'
import { useAuth } from './AuthContext'

export function ProtectedRoute({ children }: { children: ReactNode }) {
  const { isAuthenticated, loading } = useAuth()

  if (loading) return <p>Loading…</p>
  if (!isAuthenticated) return <Navigate to="/login" replace />
  return <>{children}</>
}
