import { useEffect, useRef } from 'react'
import { useNavigate } from 'react-router-dom'
import { createUserManager } from '../auth/oidc'

export default function OidcCallbackPage() {
  const navigate = useNavigate()
  // Prevent React StrictMode's double-effect from calling signinRedirectCallback()
  // twice: authorization codes are single-use and the second call would fail.
  const handled = useRef(false)

  useEffect(() => {
    if (handled.current) return
    handled.current = true

    createUserManager()
      .signinRedirectCallback()
      .then(() => navigate('/', { replace: true }))
      .catch(() => navigate('/login', { replace: true }))
  }, [navigate])

  return <p>Redirecting…</p>
}
