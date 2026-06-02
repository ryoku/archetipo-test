import { useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import { useAuth } from '../auth/AuthContext'
import { createUserManager } from '../auth/oidc'

export default function LoginPage() {
  const { isAuthenticated, loading } = useAuth()
  const navigate = useNavigate()

  useEffect(() => {
    if (loading) return
    if (isAuthenticated) {
      navigate('/', { replace: true })
      return
    }
    createUserManager().signinRedirect().catch((err) => {
      console.error('OIDC signinRedirect failed:', err)
    })
  }, [isAuthenticated, loading, navigate])

  return <p>Redirecting to login…</p>
}
