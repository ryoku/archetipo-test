import { useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import { createUserManager } from '../auth/oidc'

export default function OidcCallbackPage() {
  const navigate = useNavigate()

  useEffect(() => {
    const userManager = createUserManager()
    userManager
      .signinRedirectCallback()
      .then(() => navigate('/'))
      .catch(() => navigate('/login'))
  }, [navigate])

  return <p>Redirecting…</p>
}
