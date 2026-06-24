import { type ReactNode } from 'react'
import { Navigate } from 'react-router-dom'
import { useAuth } from './AuthContext'
import { isDevOpsAdmin } from './claims'

type AdminRouteProps = Readonly<{
  children: ReactNode
}>

export function AdminRoute({ children }: AdminRouteProps) {
  const { isAuthenticated, loading, accessToken } = useAuth()

  if (loading) return <p>Loading…</p>
  if (!isAuthenticated) return <Navigate to="/login" replace />
  if (!isDevOpsAdmin(accessToken)) return <Navigate to="/" replace />
  return <>{children}</>
}
