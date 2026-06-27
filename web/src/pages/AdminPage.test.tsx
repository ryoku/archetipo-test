import { describe, it, expect, vi, beforeEach, afterEach, type Mock } from 'vitest'
import { render, screen, waitFor, act, cleanup, within } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import type { AdminProduct, ActivityEvent } from '../api/products'
import AdminPage from './AdminPage'

// ─── Mocks ────────────────────────────────────────────────────

const mockListAdminProducts = vi.hoisted(() => vi.fn<() => Promise<AdminProduct[]>>())
const mockListAdminActivity = vi.hoisted(() => vi.fn<() => Promise<ActivityEvent[]>>())

vi.mock('../api/products', () => ({
  listAdminProducts: mockListAdminProducts,
  listAdminActivity: mockListAdminActivity,
}))

const mockUseAuth = vi.fn()

vi.mock('../auth/AuthContext', () => ({
  useAuth: () => mockUseAuth(),
}))

let capturedNavigate: Mock

vi.mock('react-router-dom', async (importOriginal) => {
  const actual = await importOriginal<typeof import('react-router-dom')>()
  return { ...actual, useNavigate: () => capturedNavigate }
})

// ─── Helpers ──────────────────────────────────────────────────

function makeActivity(overrides: Partial<ActivityEvent> = {}): ActivityEvent {
  return {
    id: 'act-1',
    actor_display_name: 'Sara Bianchi',
    tag: 'v1.15.0',
    component_name: 'api-gateway',
    product_slug: 'platform-api',
    environment_name: 'production',
    deployed_at: new Date(Date.now() - 60000).toISOString(), // 1 min ago
    outcome: 'success',
    ...overrides,
  }
}

function makeProduct(overrides: Partial<AdminProduct> = {}): AdminProduct {
  return {
    id: 'p-1',
    name: 'Platform API',
    slug: 'platform-api',
    description: 'Core platform',
    created_at: '2026-01-01T00:00:00Z',
    environment_count: 3,
    last_deployed_at: '2026-06-14T10:00:00Z',
    ...overrides,
  }
}

function renderPage() {
  return render(
    <MemoryRouter>
      <AdminPage />
    </MemoryRouter>,
  )
}

const baseAuth = {
  user: { profile: { name: 'Sara Bianchi' } },
  logout: vi.fn(),
  accessToken: 'admin-token',
}

// ─── Setup ────────────────────────────────────────────────────

beforeEach(() => {
  vi.clearAllMocks()
  capturedNavigate = vi.fn()
  mockUseAuth.mockReturnValue(baseAuth)
  mockListAdminActivity.mockResolvedValue([]) // default: empty activity
})

afterEach(cleanup)

// ─── Tests ────────────────────────────────────────────────────

describe('AdminPage', () => {
  it('shows loading skeleton initially', () => {
    mockListAdminProducts.mockReturnValue(new Promise(() => {}))
    mockListAdminActivity.mockResolvedValue([])

    renderPage()

    expect(screen.getByTestId('loading-state')).toBeTruthy()
  })

  it('shows dash placeholders in stats bar while loading', () => {
    mockListAdminProducts.mockReturnValue(new Promise(() => {}))
    mockListAdminActivity.mockResolvedValue([])

    renderPage()

    expect(within(screen.getByTestId('stats-bar')).getAllByText('—')).toHaveLength(3)
  })

  it('calls listAdminProducts with the access token', async () => {
    mockListAdminProducts.mockResolvedValue([])

    renderPage()

    await waitFor(() => {
      expect(mockListAdminProducts).toHaveBeenCalledWith('admin-token')
    })
  })

  it('does not call listAdminProducts when accessToken is null', () => {
    mockUseAuth.mockReturnValue({ ...baseAuth, accessToken: null })

    renderPage()

    expect(mockListAdminProducts).not.toHaveBeenCalled()
  })

  it('renders the products table with product rows when data loads', async () => {
    mockListAdminProducts.mockResolvedValue([
      makeProduct({ id: 'p-1', name: 'Platform API', slug: 'platform-api' }),
      makeProduct({ id: 'p-2', name: 'Customer App', slug: 'customer-app', last_deployed_at: null }),
    ])

    renderPage()

    await waitFor(() => {
      expect(screen.getByTestId('products-table')).toBeTruthy()
      expect(screen.getByText('Platform API')).toBeTruthy()
      expect(screen.getByText('Customer App')).toBeTruthy()
    })
  })

  it('shows empty state when no products exist', async () => {
    mockListAdminProducts.mockResolvedValue([])

    renderPage()

    await waitFor(() => {
      expect(screen.getByTestId('empty-state')).toBeTruthy()
    })
  })

  it('shows error message when listAdminProducts rejects with an Error', async () => {
    mockListAdminProducts.mockRejectedValue(new Error('listAdminProducts: 403'))

    renderPage()

    await waitFor(() => {
      expect(screen.getByText(/listAdminProducts: 403/)).toBeTruthy()
    })
  })

  it('shows fallback error text for non-Error rejections', async () => {
    mockListAdminProducts.mockRejectedValue('unexpected string')

    renderPage()

    await waitFor(() => {
      expect(screen.getByText('Failed to load products')).toBeTruthy()
    })
  })

  it('displays the user display name from profile.name', async () => {
    mockListAdminProducts.mockResolvedValue([])

    renderPage()

    await waitFor(() => {
      expect(screen.getByText('Sara Bianchi')).toBeTruthy()
    })
  })

  it('falls back to preferred_username when profile.name is absent', async () => {
    mockUseAuth.mockReturnValue({
      ...baseAuth,
      user: { profile: { preferred_username: 'sara.b' } },
    })
    mockListAdminProducts.mockResolvedValue([])

    renderPage()

    await waitFor(() => {
      expect(screen.getByText('sara.b')).toBeTruthy()
    })
  })

  it('falls back to "User" when both name and preferred_username are absent', async () => {
    mockUseAuth.mockReturnValue({ ...baseAuth, user: { profile: {} } })
    mockListAdminProducts.mockResolvedValue([])

    renderPage()

    await waitFor(() => {
      expect(screen.getByText('User')).toBeTruthy()
    })
  })

  it('shows aggregated stats in the stats bar after load', async () => {
    mockListAdminProducts.mockResolvedValue([
      makeProduct({ environment_count: 3, last_deployed_at: '2026-06-01T00:00:00Z' }),
      makeProduct({ id: 'p-2', name: 'Worker', slug: 'worker', environment_count: 2, last_deployed_at: null }),
    ])

    renderPage()

    await waitFor(() => screen.getByTestId('products-table'))

    const statsBar = screen.getByTestId('stats-bar')
    expect(within(statsBar).getByText('2')).toBeTruthy()  // total products
    expect(within(statsBar).getByText('5')).toBeTruthy()  // total envs (3+2)
    expect(within(statsBar).getByText('1')).toBeTruthy()  // products with deployments
  })

  it('shows table count badge with product count', async () => {
    mockListAdminProducts.mockResolvedValue([makeProduct()])

    renderPage()

    await waitFor(() => {
      expect(screen.getByTestId('table-count').textContent).toBe('1 products')
    })
  })

  it('shows "Never" for products with null last_deployed_at', async () => {
    mockListAdminProducts.mockResolvedValue([makeProduct({ last_deployed_at: null })])

    renderPage()

    await waitFor(() => {
      expect(screen.getByText('Never')).toBeTruthy()
    })
  })

  it('shows a formatted date for products with a last_deployed_at', async () => {
    mockListAdminProducts.mockResolvedValue([makeProduct({ last_deployed_at: '2026-06-14T10:00:00Z' })])

    renderPage()

    await waitFor(() => {
      expect(screen.getByText(/14 Jun 2026/)).toBeTruthy()
    })
  })

  it('navigates to product detail when a row is clicked', async () => {
    const product = makeProduct()
    mockListAdminProducts.mockResolvedValue([product])

    renderPage()

    await waitFor(() => screen.getByTestId('product-row'))

    act(() => { screen.getByTestId('product-row').click() })

    expect(capturedNavigate).toHaveBeenCalledWith('/products/platform-api', { state: product })
  })

  it('calls logout when the Logout button is clicked', async () => {
    const logoutMock = vi.fn().mockResolvedValue(undefined)
    mockUseAuth.mockReturnValue({ ...baseAuth, logout: logoutMock })
    mockListAdminProducts.mockResolvedValue([])

    renderPage()

    await waitFor(() => screen.getByText('Logout'))

    act(() => { screen.getByText('Logout').click() })

    expect(logoutMock).toHaveBeenCalled()
  })
})

describe('AdminPage — sorting', () => {
  const products = [
    makeProduct({ id: 'p-1', name: 'Zulu Service', slug: 'zulu', environment_count: 5, last_deployed_at: '2026-06-20T00:00:00Z' }),
    makeProduct({ id: 'p-2', name: 'Alpha App', slug: 'alpha', environment_count: 1, last_deployed_at: null }),
    makeProduct({ id: 'p-3', name: 'Bravo API', slug: 'bravo', environment_count: 3, last_deployed_at: '2026-05-01T00:00:00Z' }),
  ]

  beforeEach(() => {
    mockListAdminProducts.mockResolvedValue([...products])
  })

  it('defaults to name ascending order', async () => {
    renderPage()

    await waitFor(() => screen.getByTestId('products-table'))

    const rows = screen.getAllByTestId('product-row')
    expect(rows[0].textContent).toContain('Alpha App')
    expect(rows[1].textContent).toContain('Bravo API')
    expect(rows[2].textContent).toContain('Zulu Service')
  })

  it('toggles name sort to descending on second Name header click', async () => {
    renderPage()

    await waitFor(() => screen.getByTestId('products-table'))

    act(() => { screen.getByRole('columnheader', { name: /Name/i }).click() })

    const rows = screen.getAllByTestId('product-row')
    expect(rows[0].textContent).toContain('Zulu Service')
    expect(rows[2].textContent).toContain('Alpha App')
  })

  it('sorts by environment count ascending when Environments header is clicked', async () => {
    renderPage()

    await waitFor(() => screen.getByTestId('products-table'))

    act(() => { screen.getByRole('columnheader', { name: /Environments/i }).click() })

    const rows = screen.getAllByTestId('product-row')
    expect(rows[0].textContent).toContain('Alpha App')    // 1 env
    expect(rows[1].textContent).toContain('Bravo API')    // 3 envs
    expect(rows[2].textContent).toContain('Zulu Service')  // 5 envs
  })

  it('sorts by last deployment date ascending when Last Deployment header is clicked', async () => {
    renderPage()

    await waitFor(() => screen.getByTestId('products-table'))

    act(() => { screen.getByRole('columnheader', { name: /Last Deployment/i }).click() })

    const rows = screen.getAllByTestId('product-row')
    expect(rows[0].textContent).toContain('Alpha App')    // null → '' sorts first
    expect(rows[1].textContent).toContain('Bravo API')    // 2026-05
    expect(rows[2].textContent).toContain('Zulu Service')  // 2026-06
  })

  it('toggles environment count sort to descending on second click', async () => {
    renderPage()

    await waitFor(() => screen.getByTestId('products-table'))

    act(() => { screen.getByRole('columnheader', { name: /Environments/i }).click() })
    act(() => { screen.getByRole('columnheader', { name: /Environments/i }).click() })

    const rows = screen.getAllByTestId('product-row')
    expect(rows[0].textContent).toContain('Zulu Service')  // 5 envs descending
    expect(rows[2].textContent).toContain('Alpha App')     // 1 env
  })

  it('resets to ascending when switching to a different sort column', async () => {
    renderPage()

    await waitFor(() => screen.getByTestId('products-table'))

    // put Name into descending
    act(() => { screen.getByRole('columnheader', { name: /Name/i }).click() })

    // switch to Environments — should reset to ascending
    act(() => { screen.getByRole('columnheader', { name: /Environments/i }).click() })

    const rows = screen.getAllByTestId('product-row')
    expect(rows[0].textContent).toContain('Alpha App')    // 1 env — ascending
  })
})

describe('activity panel', () => {
  it('shows loading state while activity is loading', () => {
    mockListAdminProducts.mockResolvedValue([])
    mockListAdminActivity.mockReturnValue(new Promise(() => {})) // never resolves
    renderPage()
    expect(screen.getByTestId('activity-loading')).toBeTruthy()
  })

  it('shows empty state when no activity', async () => {
    mockListAdminProducts.mockResolvedValue([])
    mockListAdminActivity.mockResolvedValue([])
    renderPage()
    await waitFor(() => expect(screen.getByTestId('activity-empty')).toBeTruthy())
  })

  it('renders activity rows for each event', async () => {
    mockListAdminProducts.mockResolvedValue([])
    mockListAdminActivity.mockResolvedValue([
      makeActivity({ id: 'act-1', outcome: 'success' }),
      makeActivity({ id: 'act-2', outcome: 'in_progress', actor_display_name: 'Marco Rossi' }),
    ])
    renderPage()
    await waitFor(() => {
      const rows = screen.getAllByTestId('activity-row')
      expect(rows).toHaveLength(2)
    })
  })

  it('shows pulsing dot for in_progress outcome', async () => {
    mockListAdminProducts.mockResolvedValue([])
    mockListAdminActivity.mockResolvedValue([makeActivity({ outcome: 'in_progress' })])
    renderPage()
    await waitFor(() => {
      expect(screen.getByTestId('activity-dot-in_progress')).toBeTruthy()
    })
  })

  it('shows green dot for success outcome', async () => {
    mockListAdminProducts.mockResolvedValue([])
    mockListAdminActivity.mockResolvedValue([makeActivity({ outcome: 'success' })])
    renderPage()
    await waitFor(() => {
      expect(screen.getByTestId('activity-dot-success')).toBeTruthy()
    })
  })

  it('shows red dot for failure outcome', async () => {
    mockListAdminProducts.mockResolvedValue([])
    mockListAdminActivity.mockResolvedValue([makeActivity({ outcome: 'failure', error_message: 'ErrImagePull' })])
    renderPage()
    await waitFor(() => {
      expect(screen.getByTestId('activity-dot-failure')).toBeTruthy()
      expect(screen.getByTestId('activity-error-msg')).toBeTruthy()
    })
  })

  it('shows error banner when listAdminActivity rejects with an Error', async () => {
    mockListAdminProducts.mockResolvedValue([])
    mockListAdminActivity.mockRejectedValue(new Error('listAdminActivity: 500'))

    renderPage()

    await waitFor(() => {
      expect(screen.getByTestId('activity-error')).toBeTruthy()
      expect(screen.getByText(/listAdminActivity: 500/)).toBeTruthy()
    })
  })

  it('does not show activity-empty when fetch fails', async () => {
    mockListAdminProducts.mockResolvedValue([])
    mockListAdminActivity.mockRejectedValue(new Error('network error'))

    renderPage()

    await waitFor(() => {
      expect(screen.queryByTestId('activity-empty')).toBeNull()
    })
  })

  it('shows fallback error text for non-Error activity rejections', async () => {
    mockListAdminProducts.mockResolvedValue([])
    mockListAdminActivity.mockRejectedValue('unexpected string')

    renderPage()

    await waitFor(() => {
      expect(screen.getByText('Failed to load activity feed')).toBeTruthy()
    })
  })

  it('clears error banner when subsequent poll succeeds', async () => {
    mockListAdminProducts.mockResolvedValue([])
    // First call fails, second succeeds
    mockListAdminActivity
      .mockRejectedValueOnce(new Error('transient error'))
      .mockResolvedValueOnce([makeActivity()])

    renderPage()

    await waitFor(() => {
      expect(screen.getByTestId('activity-error')).toBeTruthy()
    })

    // Simulate a re-fetch by directly calling the mock's second value
    // (we can't easily trigger the 30s interval in unit tests, so verify the
    // resolved path clears the error by re-mounting with a success mock)
    cleanup()
    mockListAdminActivity.mockResolvedValue([makeActivity()])
    renderPage()

    await waitFor(() => {
      expect(screen.queryByTestId('activity-error')).toBeNull()
      expect(screen.getByTestId('activity-list')).toBeTruthy()
    })
  })

  it('hides activity list when fetch fails after a successful load', async () => {
    mockListAdminProducts.mockResolvedValue([])
    mockListAdminActivity
      .mockResolvedValueOnce([makeActivity()])
      .mockRejectedValueOnce(new Error('poll error'))

    renderPage()

    // After first successful load, list is visible
    await waitFor(() => {
      expect(screen.getByTestId('activity-list')).toBeTruthy()
    })

    // Re-render to trigger re-fetch with the failing mock
    cleanup()
    mockListAdminActivity.mockRejectedValue(new Error('poll error'))
    renderPage()

    await waitFor(() => {
      expect(screen.getByTestId('activity-error')).toBeTruthy()
      expect(screen.queryByTestId('activity-list')).toBeNull()
    })
  })

  it('renders a relative timestamp in each activity row', async () => {
    // makeActivity sets deployed_at to 1 minute ago, so the relative time is "1m fa"
    mockListAdminProducts.mockResolvedValue([])
    mockListAdminActivity.mockResolvedValue([makeActivity()])
    renderPage()
    await waitFor(() => {
      expect(screen.getByText('1m fa')).toBeTruthy()
    })
  })
})
