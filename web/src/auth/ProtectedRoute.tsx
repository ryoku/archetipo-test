import { type ReactNode } from 'react'
import { Navigate } from 'react-router-dom'
import { useAuth } from './AuthContext'

type ProtectedRouteProps = Readonly<{
  children: ReactNode
}>

export function ProtectedRoute({ children }: ProtectedRouteProps) {
  const { isAuthenticated, loading } = useAuth()

  if (loading) return <p>Loading…</p>
  if (!isAuthenticated) return <Navigate to="/login" replace />
  return <>{children}</>
}
