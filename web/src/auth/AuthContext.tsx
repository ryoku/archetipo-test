import { createContext, useContext, useEffect, useMemo, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import type { User } from 'oidc-client-ts'
import { createUserManager } from './oidc'

interface AuthContextValue {
  user: User | null
  isAuthenticated: boolean
  loading: boolean
  login: () => Promise<void>
  logout: () => Promise<void>
  accessToken: string | null
}

const AuthContext = createContext<AuthContextValue | null>(null)

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const navigate = useNavigate()
  const userManager = useMemo(() => createUserManager(), [])
  const [user, setUser] = useState<User | null>(null)
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    userManager.getUser().then(u => { setUser(u); setLoading(false) })

    const onUserLoaded = (u: User) => setUser(u)
    const onTokenExpired = () => navigate('/login')

    userManager.events.addUserLoaded(onUserLoaded)
    userManager.events.addAccessTokenExpired(onTokenExpired)

    const onUnauthorized = () => navigate('/login')
    window.addEventListener('auth:unauthorized', onUnauthorized)

    return () => {
      userManager.events.removeUserLoaded(onUserLoaded)
      userManager.events.removeAccessTokenExpired(onTokenExpired)
      window.removeEventListener('auth:unauthorized', onUnauthorized)
    }
  }, [userManager, navigate])

  const value = useMemo<AuthContextValue>(
    () => ({
      user,
      isAuthenticated: !!user && !user.expired,
      loading,
      login: () => userManager.signinRedirect(),
      logout: () => userManager.signoutRedirect(),
      accessToken: user?.access_token ?? null,
    }),
    [user, loading, userManager],
  )

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>
}

// eslint-disable-next-line react-refresh/only-export-components
export function useAuth(): AuthContextValue {
  const ctx = useContext(AuthContext)
  if (!ctx) throw new Error('useAuth must be used within AuthProvider')
  return ctx
}
