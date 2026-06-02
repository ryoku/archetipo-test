import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { vi, describe, it, expect } from 'vitest'
import { ProtectedRoute } from './ProtectedRoute'

vi.mock('./AuthContext')

import { useAuth } from './AuthContext'
const mockUseAuth = vi.mocked(useAuth)

const baseAuth = {
  user: null,
  loading: false,
  login: vi.fn(),
  logout: vi.fn(),
  accessToken: null,
}

describe('ProtectedRoute', () => {
  it('renders loading indicator while loading', () => {
    mockUseAuth.mockReturnValue({ ...baseAuth, isAuthenticated: false, loading: true })

    render(
      <MemoryRouter initialEntries={['/protected']}>
        <ProtectedRoute>
          <span>secret content</span>
        </ProtectedRoute>
      </MemoryRouter>,
    )

    expect(screen.queryByText('secret content')).toBeNull()
    expect(screen.getByText('Loading…')).toBeTruthy()
  })

  it('does not render children when unauthenticated', () => {
    mockUseAuth.mockReturnValue({ ...baseAuth, isAuthenticated: false })

    render(
      <MemoryRouter initialEntries={['/protected']}>
        <ProtectedRoute>
          <span>secret content</span>
        </ProtectedRoute>
      </MemoryRouter>,
    )

    expect(screen.queryByText('secret content')).toBeNull()
  })

  it('renders children when authenticated and not loading', () => {
    mockUseAuth.mockReturnValue({ ...baseAuth, isAuthenticated: true })

    render(
      <MemoryRouter initialEntries={['/protected']}>
        <ProtectedRoute>
          <span>secret content</span>
        </ProtectedRoute>
      </MemoryRouter>,
    )

    expect(screen.getByText('secret content')).toBeTruthy()
  })
})
