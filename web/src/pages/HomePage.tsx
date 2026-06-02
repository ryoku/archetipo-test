import { useAuth } from '../auth/AuthContext'

export default function HomePage() {
  const { user, logout } = useAuth()

  return (
    <main>
      <div>
        <span>{user?.profile?.name ?? user?.profile?.preferred_username ?? 'User'}</span>
        <button onClick={() => logout()}>Logout</button>
      </div>
    </main>
  )
}
