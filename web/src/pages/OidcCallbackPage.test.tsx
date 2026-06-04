import { StrictMode } from 'react'
import { render, waitFor } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi, type Mock } from 'vitest'
import OidcCallbackPage from './OidcCallbackPage'

const mockCreateUserManager = vi.fn()
const mockSigninRedirectCallback = vi.fn()
let capturedNavigate: Mock

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

describe('OidcCallbackPage', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    capturedNavigate = vi.fn()
    mockSigninRedirectCallback.mockResolvedValue(undefined)
    mockCreateUserManager.mockReturnValue({ signinRedirectCallback: mockSigninRedirectCallback })
  })

  it('handles the callback only once in StrictMode and navigates home on success', async () => {
    render(
      <StrictMode>
        <OidcCallbackPage />
      </StrictMode>,
    )

    await waitFor(() => {
      expect(mockSigninRedirectCallback).toHaveBeenCalledOnce()
    })
    expect(capturedNavigate).toHaveBeenCalledWith('/', { replace: true })
  })

  it('navigates back to login when callback handling fails', async () => {
    mockSigninRedirectCallback.mockRejectedValueOnce(new Error('callback failed'))

    render(<OidcCallbackPage />)

    await waitFor(() => {
      expect(capturedNavigate).toHaveBeenCalledWith('/login', { replace: true })
    })
  })
})