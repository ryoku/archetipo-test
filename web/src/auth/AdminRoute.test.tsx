import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { vi, describe, it, expect } from 'vitest'
import { AdminRoute } from './AdminRoute'

vi.mock('./AuthContext')

import { useAuth } from './AuthContext'
const mockUseAuth = vi.mocked(useAuth)

// Fake JWT with realm_access.roles: ['kubegate:devops-admin']
const ADMIN_TOKEN =
  'header.eyJyZWFsbV9hY2Nlc3MiOiB7InJvbGVzIjogWyJrdWJlZ2F0ZTpkZXZvcHMtYWRtaW4iXX19.sig'

// Fake JWT with realm_access.roles: ['kubegate:viewer'] (not admin)
const NON_ADMIN_TOKEN =
  'header.eyJyZWFsbV9hY2Nlc3MiOiB7InJvbGVzIjogWyJrdWJlZ2F0ZTp2aWV3ZXIiXX19.sig'

const baseAuth = {
  user: null,
  loading: false,
  login: vi.fn(),
  logout: vi.fn(),
  accessToken: null,
}

describe('AdminRoute', () => {
  it('redirects to login when not authenticated', () => {
    mockUseAuth.mockReturnValue({ ...baseAuth, isAuthenticated: false })

    render(
      <MemoryRouter initialEntries={['/admin']}>
        <AdminRoute>
          <span>admin content</span>
        </AdminRoute>
      </MemoryRouter>,
    )

    expect(screen.queryByText('admin content')).toBeNull()
  })

  it('redirects to home when authenticated but not admin', () => {
    mockUseAuth.mockReturnValue({
      ...baseAuth,
      isAuthenticated: true,
      accessToken: NON_ADMIN_TOKEN,
    })

    render(
      <MemoryRouter initialEntries={['/admin']}>
        <AdminRoute>
          <span>admin content</span>
        </AdminRoute>
      </MemoryRouter>,
    )

    expect(screen.queryByText('admin content')).toBeNull()
  })

  it('renders children when authenticated admin', () => {
    mockUseAuth.mockReturnValue({
      ...baseAuth,
      isAuthenticated: true,
      accessToken: ADMIN_TOKEN,
    })

    render(
      <MemoryRouter initialEntries={['/admin']}>
        <AdminRoute>
          <span>admin content</span>
        </AdminRoute>
      </MemoryRouter>,
    )

    expect(screen.getByText('admin content')).toBeTruthy()
  })
})
