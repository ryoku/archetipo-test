import { render, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi, type Mock } from 'vitest'
import LoginPage from './LoginPage'

const mockUseAuth = vi.fn()
const mockCreateUserManager = vi.fn()
const mockSigninRedirect = vi.fn()
let capturedNavigate: Mock

vi.mock('../auth/AuthContext', () => ({
  useAuth: () => mockUseAuth(),
}))

vi.mock('../auth/oidc', () => ({
  createUserManager: () => mockCreateUserManager(),
}))

vi.mock('react-router-dom', async (importOriginal) => {
  const actual = await importOriginal<typeof import('react-router-dom')>()
  return {
    ...actual,
    useNavigate: () => capturedNavigate,
  }
})

describe('LoginPage', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    capturedNavigate = vi.fn()
    mockSigninRedirect.mockResolvedValue(undefined)
    mockCreateUserManager.mockReturnValue({ signinRedirect: mockSigninRedirect })
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('waits while auth state is loading', () => {
    mockUseAuth.mockReturnValue({ isAuthenticated: false, loading: true })

    render(<LoginPage />)

    expect(capturedNavigate).not.toHaveBeenCalled()
    expect(mockSigninRedirect).not.toHaveBeenCalled()
  })

  it('redirects home when the user is already authenticated', async () => {
    mockUseAuth.mockReturnValue({ isAuthenticated: true, loading: false })

    render(<LoginPage />)

    await waitFor(() => {
      expect(capturedNavigate).toHaveBeenCalledWith('/', { replace: true })
    })
    expect(mockSigninRedirect).not.toHaveBeenCalled()
  })

  it('starts the OIDC signin redirect when unauthenticated', async () => {
    mockUseAuth.mockReturnValue({ isAuthenticated: false, loading: false })

    render(<LoginPage />)

    await waitFor(() => {
      expect(mockSigninRedirect).toHaveBeenCalledOnce()
    })
  })

  it('logs the signin error when the redirect promise rejects', async () => {
    const error = new Error('redirect failed')
    const consoleError = vi.spyOn(console, 'error').mockImplementation(() => {})
    mockUseAuth.mockReturnValue({ isAuthenticated: false, loading: false })
    mockSigninRedirect.mockRejectedValueOnce(error)

    render(<LoginPage />)

    await waitFor(() => {
      expect(consoleError).toHaveBeenCalledWith('OIDC signinRedirect failed:', error)
    })
  })
})