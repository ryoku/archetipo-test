import { describe, it, expect, vi, beforeEach, afterEach, type Mock } from 'vitest'
import { render, screen, act, waitFor, cleanup } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import type { User } from 'oidc-client-ts'
import { AuthProvider, useAuth } from './AuthContext'

// ---------------------------------------------------------------------------
// Mock oidc module
// ---------------------------------------------------------------------------

const mockGetUser = vi.fn<() => Promise<User | null>>()
const mockSigninRedirect = vi.fn<() => Promise<void>>()
const mockSignoutRedirect = vi.fn<() => Promise<void>>()

const mockEvents = {
  addUserLoaded: vi.fn(),
  removeUserLoaded: vi.fn(),
  addAccessTokenExpired: vi.fn(),
  removeAccessTokenExpired: vi.fn(),
}

const mockUserManager = {
  getUser: mockGetUser,
  signinRedirect: mockSigninRedirect,
  signoutRedirect: mockSignoutRedirect,
  events: mockEvents,
}

vi.mock('./oidc', () => ({
  createUserManager: () => mockUserManager,
}))

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function makeUser(overrides: Partial<User> = {}): User {
  return {
    access_token: 'tok-abc',
    expired: false,
    ...overrides,
  } as unknown as User
}

let capturedNavigate: Mock

vi.mock('react-router-dom', async (importOriginal) => {
  const actual = await importOriginal<typeof import('react-router-dom')>()
  return {
    ...actual,
    useNavigate: () => capturedNavigate,
  }
})

// Consumer component that exposes context values via data-testid attributes.
function Consumer() {
  const { isAuthenticated, loading, accessToken, login, logout } = useAuth()
  return (
    <div>
      <span data-testid="auth">{String(isAuthenticated)}</span>
      <span data-testid="loading">{String(loading)}</span>
      <span data-testid="token">{accessToken ?? 'null'}</span>
      <button onClick={login} data-testid="login">login</button>
      <button onClick={logout} data-testid="logout">logout</button>
    </div>
  )
}

function renderProvider() {
  return render(
    <MemoryRouter>
      <AuthProvider>
        <Consumer />
      </AuthProvider>
    </MemoryRouter>,
  )
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

beforeEach(() => {
  vi.clearAllMocks()
  capturedNavigate = vi.fn()
  mockGetUser.mockResolvedValue(null)
})

afterEach(cleanup)

describe('AuthProvider — initial state', () => {
  it('renders unauthenticated when getUser resolves null', async () => {
    renderProvider()
    await waitFor(() => {
      expect(screen.getByTestId('auth').textContent).toBe('false')
    })
    expect(screen.getByTestId('token').textContent).toBe('null')
  })
})

describe('AuthProvider — user loaded on mount', () => {
  it('sets isAuthenticated and accessToken when getUser resolves a user', async () => {
    mockGetUser.mockResolvedValue(makeUser({ access_token: 'tok-xyz' }))
    renderProvider()
    await waitFor(() => {
      expect(screen.getByTestId('auth').textContent).toBe('true')
    })
    expect(screen.getByTestId('token').textContent).toBe('tok-xyz')
  })

  it('treats an expired user as unauthenticated', async () => {
    mockGetUser.mockResolvedValue(makeUser({ expired: true }))
    renderProvider()
    await waitFor(() => {
      expect(screen.getByTestId('auth').textContent).toBe('false')
    })
  })
})

describe('AuthProvider — login / logout', () => {
  it('calls signinRedirect when login() is invoked', async () => {
    mockSigninRedirect.mockResolvedValue(undefined)
    renderProvider()
    await waitFor(() => screen.getByTestId('login'))
    await act(async () => {
      screen.getByTestId('login').click()
    })
    expect(mockSigninRedirect).toHaveBeenCalledOnce()
  })

  it('calls signoutRedirect when logout() is invoked', async () => {
    mockSignoutRedirect.mockResolvedValue(undefined)
    renderProvider()
    await waitFor(() => screen.getByTestId('logout'))
    await act(async () => {
      screen.getByTestId('logout').click()
    })
    expect(mockSignoutRedirect).toHaveBeenCalledOnce()
  })
})

describe('AuthProvider — event subscriptions', () => {
  it('registers and unregisters userLoaded and accessTokenExpired listeners', async () => {
    const { unmount } = renderProvider()
    await waitFor(() => screen.getByTestId('auth'))

    expect(mockEvents.addUserLoaded).toHaveBeenCalledOnce()
    expect(mockEvents.addAccessTokenExpired).toHaveBeenCalledOnce()

    unmount()

    expect(mockEvents.removeUserLoaded).toHaveBeenCalledOnce()
    expect(mockEvents.removeAccessTokenExpired).toHaveBeenCalledOnce()
  })

  it('updates user when the userLoaded event fires', async () => {
    renderProvider()
    await waitFor(() => screen.getByTestId('auth'))

    const onUserLoaded: (u: User) => void = (mockEvents.addUserLoaded as Mock).mock.calls[0][0]

    act(() => {
      onUserLoaded(makeUser({ access_token: 'tok-loaded' }))
    })

    await waitFor(() => {
      expect(screen.getByTestId('token').textContent).toBe('tok-loaded')
    })
  })

  it('navigates to /login when accessTokenExpired fires', async () => {
    renderProvider()
    await waitFor(() => screen.getByTestId('auth'))

    const onTokenExpired: () => void = (mockEvents.addAccessTokenExpired as Mock).mock.calls[0][0]

    act(() => {
      onTokenExpired()
    })

    expect(capturedNavigate).toHaveBeenCalledWith('/login')
  })

  it('navigates to /login when auth:unauthorized is dispatched on window', async () => {
    renderProvider()
    await waitFor(() => screen.getByTestId('auth'))

    act(() => {
      window.dispatchEvent(new Event('auth:unauthorized'))
    })

    expect(capturedNavigate).toHaveBeenCalledWith('/login')
  })
})

describe('useAuth — outside provider', () => {
  it('throws when used outside AuthProvider', () => {
    const consoleError = vi.spyOn(console, 'error').mockImplementation(() => {})
    expect(() =>
      render(
        <MemoryRouter>
          <Consumer />
        </MemoryRouter>,
      ),
    ).toThrow('useAuth must be used within AuthProvider')
    consoleError.mockRestore()
  })
})
