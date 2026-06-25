import { describe, it, expect, vi, beforeEach, afterEach, type Mock } from 'vitest'
import { render, screen, waitFor, act, cleanup, fireEvent } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import type { Product } from '../api/products'
import type { Stats } from '../api/stats'
import HomePage from './HomePage'

// ─── Mocks ────────────────────────────────────────────────────

const mockListProducts = vi.hoisted(() => vi.fn<() => Promise<Product[]>>())
const mockCreateProduct = vi.hoisted(() => vi.fn<() => Promise<Product>>())

vi.mock('../api/products', () => ({
  listProducts: mockListProducts,
  createProduct: mockCreateProduct,
}))

const mockFetchStats = vi.hoisted(() => vi.fn<() => Promise<Stats>>())

vi.mock('../api/stats', () => ({
  fetchStats: mockFetchStats,
}))

const mockIsDevOpsAdmin = vi.hoisted(() => vi.fn<() => boolean>())

vi.mock('../auth/claims', () => ({ isDevOpsAdmin: mockIsDevOpsAdmin }))

const mockUseAuth = vi.fn()

vi.mock('../auth/AuthContext', () => ({
  useAuth: () => mockUseAuth(),
}))

let capturedNavigate: Mock

vi.mock('react-router-dom', async (importOriginal) => {
  const actual = await importOriginal<typeof import('react-router-dom')>()
  return {
    ...actual,
    useNavigate: () => capturedNavigate,
  }
})

// ─── Helpers ──────────────────────────────────────────────────

function makeProduct(overrides: Partial<Product> = {}): Product {
  return {
    id: '1',
    name: 'Platform API',
    slug: 'platform-api',
    description: 'Core platform API',
    created_at: '2025-01-01T00:00:00Z',
    last_deployed_at: null,
    has_production_env: false,
    ...overrides,
  }
}

function renderPage() {
  return render(
    <MemoryRouter>
      <HomePage />
    </MemoryRouter>,
  )
}

// ─── Setup ────────────────────────────────────────────────────

beforeEach(() => {
  vi.clearAllMocks()
  capturedNavigate = vi.fn()
  mockUseAuth.mockReturnValue({
    user: { profile: { name: 'Alice' } },
    logout: vi.fn(),
    accessToken: 'test-token',
  })
  // Default: stats resolves so it doesn't interfere with unrelated tests
  mockFetchStats.mockResolvedValue({
    product_count: 3,
    environment_count: 7,
    component_count: 5,
    deployments_last_24h: 12,
  })
})

afterEach(cleanup)

// ─── Tests ────────────────────────────────────────────────────

describe('HomePage', () => {
  it('shows loading state initially', () => {
    // listProducts never resolves during this test
    mockListProducts.mockReturnValue(new Promise(() => {}))

    renderPage()

    expect(screen.getByTestId('loading-state')).toBeTruthy()
  })

  it('renders product cards when listProducts resolves with data', async () => {
    const products = [
      makeProduct({ id: '1', name: 'Platform API', slug: 'platform-api' }),
      makeProduct({ id: '2', name: 'Worker Service', slug: 'worker-service' }),
    ]
    mockListProducts.mockResolvedValue(products)

    renderPage()

    await waitFor(() => {
      expect(screen.getByText('Platform API')).toBeTruthy()
      expect(screen.getByText('Worker Service')).toBeTruthy()
    })
  })

  it('shows slugs for each product card', async () => {
    const products = [makeProduct({ slug: 'platform-api' })]
    mockListProducts.mockResolvedValue(products)

    renderPage()

    await waitFor(() => {
      expect(screen.getByText('platform-api')).toBeTruthy()
    })
  })

  it('shows empty state when listProducts returns empty array', async () => {
    mockListProducts.mockResolvedValue([])

    renderPage()

    await waitFor(() => {
      expect(screen.getByTestId('empty-state')).toBeTruthy()
    })
  })

  it('shows error message when listProducts rejects', async () => {
    mockListProducts.mockRejectedValue(new Error('listProducts: 500'))

    renderPage()

    await waitFor(() => {
      expect(screen.getByRole('alert')).toBeTruthy()
      expect(screen.getByText(/listProducts: 500/)).toBeTruthy()
    })
  })

  it('navigates to product detail on card click', async () => {
    const product = makeProduct({ id: '1', name: 'Platform API', slug: 'platform-api' })
    mockListProducts.mockResolvedValue([product])

    renderPage()

    await waitFor(() => screen.getByText('Platform API'))

    act(() => {
      screen.getByText('Platform API').closest('button')?.click()
    })

    expect(capturedNavigate).toHaveBeenCalledWith('/products/platform-api', {
      state: product,
    })
  })

  it('calls listProducts with the access token', async () => {
    mockListProducts.mockResolvedValue([])

    renderPage()

    await waitFor(() => {
      expect(mockListProducts).toHaveBeenCalledWith('test-token')
    })
  })

  it('displays the user name', async () => {
    mockListProducts.mockResolvedValue([])

    renderPage()

    await waitFor(() => {
      expect(screen.getByText('Alice')).toBeTruthy()
    })
  })
})

describe('Add Product form', () => {
  it('shows Add Product button for DevOps Admin', async () => {
    mockIsDevOpsAdmin.mockReturnValue(true)
    mockListProducts.mockResolvedValue([])

    renderPage()

    await waitFor(() => {
      expect(screen.getByTestId('empty-state')).toBeTruthy()
    })

    expect(screen.getByText('Add Product')).toBeTruthy()
  })

  it('hides Add Product button for non-admin', async () => {
    mockIsDevOpsAdmin.mockReturnValue(false)
    mockListProducts.mockResolvedValue([])

    renderPage()

    await waitFor(() => {
      expect(screen.getByTestId('empty-state')).toBeTruthy()
    })

    expect(screen.queryByText('Add Product')).toBeNull()
  })

  it('shows inline form when Add Product is clicked', async () => {
    mockIsDevOpsAdmin.mockReturnValue(true)
    mockListProducts.mockResolvedValue([])

    renderPage()

    await waitFor(() => screen.getByText('Add Product'))

    act(() => {
      screen.getByText('Add Product').click()
    })

    expect(screen.getByLabelText('Name *')).toBeTruthy()
    expect(screen.getByLabelText('Slug *')).toBeTruthy()
  })

  it('validates required fields', async () => {
    mockIsDevOpsAdmin.mockReturnValue(true)
    mockListProducts.mockResolvedValue([])

    renderPage()

    await waitFor(() => screen.getByText('Add Product'))

    act(() => {
      screen.getByText('Add Product').click()
    })

    act(() => {
      screen.getByText('Save Product').click()
    })

    await waitFor(() => {
      expect(screen.getByText('Name is required')).toBeTruthy()
      expect(screen.getByText('Slug is required')).toBeTruthy()
    })

    expect(mockCreateProduct).not.toHaveBeenCalled()
  })

  it('creates product and prepends to list', async () => {
    const existingProduct = makeProduct({ id: '1', name: 'Platform API', slug: 'platform-api' })
    const newProduct = makeProduct({ id: '2', name: 'New App', slug: 'new-app' })

    mockIsDevOpsAdmin.mockReturnValue(true)
    mockListProducts.mockResolvedValue([existingProduct])
    mockCreateProduct.mockResolvedValue(newProduct)

    renderPage()

    await waitFor(() => screen.getByText('Add Product'))

    act(() => {
      screen.getByText('Add Product').click()
    })

    fireEvent.change(screen.getByLabelText('Name *'), { target: { value: 'New App' } })
    fireEvent.change(screen.getByLabelText('Slug *'), { target: { value: 'new-app' } })

    await act(async () => {
      screen.getByText('Save Product').click()
    })

    await waitFor(() => {
      expect(mockCreateProduct).toHaveBeenCalledWith('test-token', {
        name: 'New App',
        slug: 'new-app',
        description: '',
      })
    })

    await waitFor(() => {
      expect(screen.getByText('New App')).toBeTruthy()
      expect(screen.queryByLabelText('Name *')).toBeNull()
    })

    // New product should appear before the existing one
    const cards = screen.getAllByRole('button', { name: /New App|Platform API/ })
    expect(cards[0].textContent).toContain('New App')
  })

  it('shows conflict error inline without closing form', async () => {
    mockIsDevOpsAdmin.mockReturnValue(true)
    mockListProducts.mockResolvedValue([])
    mockCreateProduct.mockRejectedValue(new Error('createProduct: 409'))

    renderPage()

    await waitFor(() => screen.getByText('Add Product'))

    act(() => {
      screen.getByText('Add Product').click()
    })

    fireEvent.change(screen.getByLabelText('Name *'), { target: { value: 'Existing App' } })
    fireEvent.change(screen.getByLabelText('Slug *'), { target: { value: 'existing-app' } })

    await act(async () => {
      screen.getByText('Save Product').click()
    })

    await waitFor(() => {
      expect(screen.getByRole('alert')).toBeTruthy()
      expect(screen.getByText(/slug already exists/i)).toBeTruthy()
    })

    // Form should still be open
    expect(screen.getByLabelText('Name *')).toBeTruthy()
  })

  it('shows slug format error for invalid slug', async () => {
    mockIsDevOpsAdmin.mockReturnValue(true)
    mockListProducts.mockResolvedValue([])

    renderPage()

    await waitFor(() => screen.getByText('Add Product'))

    act(() => { screen.getByText('Add Product').click() })

    fireEvent.change(screen.getByLabelText('Name *'), { target: { value: 'My App' } })
    fireEvent.change(screen.getByLabelText('Slug *'), { target: { value: 'My Slug' } })

    act(() => { screen.getByText('Save Product').click() })

    await waitFor(() => {
      expect(screen.getByText('Slug must be lowercase letters, numbers, and hyphens only')).toBeTruthy()
    })
    expect(mockCreateProduct).not.toHaveBeenCalled()
  })

  it('closes form and restores Add Product button when Cancel is clicked', async () => {
    mockIsDevOpsAdmin.mockReturnValue(true)
    mockListProducts.mockResolvedValue([])

    renderPage()

    await waitFor(() => screen.getByText('Add Product'))

    act(() => { screen.getByText('Add Product').click() })

    expect(screen.getByLabelText('Name *')).toBeTruthy()

    fireEvent.change(screen.getByLabelText('Name *'), { target: { value: 'Some App' } })

    act(() => { screen.getByText('Cancel').click() })

    expect(screen.queryByLabelText('Name *')).toBeNull()
    expect(screen.getByText('Add Product')).toBeTruthy()
  })
})

describe('Stats strip', () => {
  it('renders 4 stat tiles', async () => {
    mockListProducts.mockResolvedValue([])

    renderPage()

    await waitFor(() => {
      expect(screen.getByTestId('stats-strip')).toBeTruthy()
      expect(screen.getAllByTestId('stat-tile')).toHaveLength(4)
    })
  })

  it('shows numeric values from API on success', async () => {
    mockListProducts.mockResolvedValue([])
    mockFetchStats.mockResolvedValue({
      product_count: 4,
      environment_count: 9,
      component_count: 6,
      deployments_last_24h: 3,
    })

    renderPage()

    await waitFor(() => {
      expect(screen.getByText('4')).toBeTruthy()
      expect(screen.getByText('9')).toBeTruthy()
      expect(screen.getByText('6')).toBeTruthy()
      expect(screen.getByText('3')).toBeTruthy()
    })
  })

  it('shows "--" in all tiles when API call fails', async () => {
    mockListProducts.mockResolvedValue([])
    mockFetchStats.mockRejectedValue(new Error('fetchStats: 500'))

    renderPage()

    await waitFor(() => {
      const tiles = screen.getAllByTestId('stat-tile')
      expect(tiles).toHaveLength(4)
      tiles.forEach((tile) => {
        expect(tile.textContent).toContain('--')
      })
    })
  })

  it('shows "…" in all tiles while loading, then values on resolve', async () => {
    mockListProducts.mockResolvedValue([])
    let resolveStats!: (s: Stats) => void
    mockFetchStats.mockReturnValue(new Promise<Stats>((res) => { resolveStats = res }))

    renderPage()

    // While fetch is pending, tiles should show the loading indicator
    const tiles = screen.getAllByTestId('stat-tile')
    tiles.forEach((tile) => expect(tile.textContent).toContain('…'))

    // Resolve the fetch and confirm values appear
    act(() => resolveStats({ product_count: 2, environment_count: 5, component_count: 3, deployments_last_24h: 1 }))
    await waitFor(() => expect(screen.getByText('2')).toBeTruthy())
  })

  it('tiles are not interactive — clicking does not navigate', async () => {
    mockListProducts.mockResolvedValue([])

    renderPage()

    await waitFor(() => screen.getByTestId('stats-strip'))

    const tiles = screen.getAllByTestId('stat-tile')
    tiles.forEach((tile) => {
      expect(tile.tagName.toLowerCase()).not.toBe('a')
      expect(tile.tagName.toLowerCase()).not.toBe('button')
      expect(tile.getAttribute('role')).not.toBe('link')
      expect(tile.getAttribute('role')).not.toBe('button')
    })

    expect(capturedNavigate).not.toHaveBeenCalled()
  })
})

describe('Search and filter', () => {
  const now = new Date().toISOString()
  const recent = new Date(Date.now() - 60 * 60 * 1000).toISOString() // 1 hour ago
  const old = new Date(Date.now() - 48 * 60 * 60 * 1000).toISOString() // 48 hours ago

  const products = [
    makeProduct({ id: '1', name: 'Platform API', slug: 'platform-api', has_production_env: true, last_deployed_at: recent }),
    makeProduct({ id: '2', name: 'Customer App', slug: 'customer-app', has_production_env: false, last_deployed_at: old }),
    makeProduct({ id: '3', name: 'Worker Service', slug: 'worker-svc', has_production_env: false, last_deployed_at: null }),
  ]

  beforeEach(() => {
    mockListProducts.mockResolvedValue(products)
  })

  it('shows search bar and chips after products load', async () => {
    renderPage()
    await waitFor(() => screen.getByTestId('search-filter-bar'))
    expect(screen.getByTestId('search-input')).toBeTruthy()
    expect(screen.getByTestId('chip-all')).toBeTruthy()
    expect(screen.getByTestId('chip-production')).toBeTruthy()
    expect(screen.getByTestId('chip-recently-deployed')).toBeTruthy()
  })

  it('filters by name with case-insensitive substring match', async () => {
    renderPage()
    await waitFor(() => screen.getByTestId('search-input'))

    fireEvent.change(screen.getByTestId('search-input'), { target: { value: 'platform' } })

    await waitFor(() => {
      expect(screen.getByText('Platform API')).toBeTruthy()
      expect(screen.queryByText('Customer App')).toBeNull()
      expect(screen.queryByText('Worker Service')).toBeNull()
    })
  })

  it('filters by slug substring', async () => {
    renderPage()
    await waitFor(() => screen.getByTestId('search-input'))

    fireEvent.change(screen.getByTestId('search-input'), { target: { value: 'svc' } })

    await waitFor(() => {
      expect(screen.getByText('Worker Service')).toBeTruthy()
      expect(screen.queryByText('Platform API')).toBeNull()
      expect(screen.queryByText('Customer App')).toBeNull()
    })
  })

  it('chip Production shows only products with has_production_env=true', async () => {
    renderPage()
    await waitFor(() => screen.getByTestId('chip-production'))

    act(() => { screen.getByTestId('chip-production').click() })

    await waitFor(() => {
      expect(screen.getByText('Platform API')).toBeTruthy()
      expect(screen.queryByText('Customer App')).toBeNull()
      expect(screen.queryByText('Worker Service')).toBeNull()
    })
  })

  it('chip Recently deployed shows only products deployed in last 24h', async () => {
    renderPage()
    await waitFor(() => screen.getByTestId('chip-recently-deployed'))

    act(() => { screen.getByTestId('chip-recently-deployed').click() })

    await waitFor(() => {
      expect(screen.getByText('Platform API')).toBeTruthy()
      expect(screen.queryByText('Customer App')).toBeNull()
      expect(screen.queryByText('Worker Service')).toBeNull()
    })
  })

  it('combines search text and chip filter', async () => {
    // Add a second product with production env to ensure AND logic
    const extraProducts = [
      ...products,
      makeProduct({ id: '4', name: 'Platform Payments', slug: 'platform-payments', has_production_env: true, last_deployed_at: null }),
    ]
    mockListProducts.mockResolvedValue(extraProducts)

    renderPage()
    await waitFor(() => screen.getByTestId('search-input'))

    act(() => { screen.getByTestId('chip-production').click() })
    fireEvent.change(screen.getByTestId('search-input'), { target: { value: 'platform' } })

    await waitFor(() => {
      expect(screen.getByText('Platform API')).toBeTruthy()
      expect(screen.getByText('Platform Payments')).toBeTruthy()
      expect(screen.queryByText('Customer App')).toBeNull()
      expect(screen.queryByText('Worker Service')).toBeNull()
    })
  })

  it('shows filter empty state with contextual message when no products match', async () => {
    renderPage()
    await waitFor(() => screen.getByTestId('search-input'))

    fireEvent.change(screen.getByTestId('search-input'), { target: { value: 'zzz-nonexistent' } })

    await waitFor(() => {
      expect(screen.getByTestId('filter-empty-state')).toBeTruthy()
      expect(screen.getByText(/No products match your search/i)).toBeTruthy()
    })
  })

  it('shows clear filters button in filter empty state', async () => {
    renderPage()
    await waitFor(() => screen.getByTestId('search-input'))

    fireEvent.change(screen.getByTestId('search-input'), { target: { value: 'zzz' } })

    await waitFor(() => screen.getByTestId('filter-empty-state'))
    expect(screen.getByText('Clear filters')).toBeTruthy()
  })

  it('clicking Clear filters resets search and chip to show all products', async () => {
    renderPage()
    await waitFor(() => screen.getByTestId('search-input'))

    act(() => { screen.getByTestId('chip-production').click() })
    fireEvent.change(screen.getByTestId('search-input'), { target: { value: 'zzz' } })

    await waitFor(() => screen.getByTestId('filter-empty-state'))

    act(() => { screen.getByText('Clear filters').click() })

    await waitFor(() => {
      expect(screen.getByText('Platform API')).toBeTruthy()
      expect(screen.getByText('Customer App')).toBeTruthy()
      expect(screen.getByText('Worker Service')).toBeTruthy()
      expect(screen.queryByTestId('filter-empty-state')).toBeNull()
    })
  })

  it('clear button in search input clears search text', async () => {
    renderPage()
    await waitFor(() => screen.getByTestId('search-input'))

    fireEvent.change(screen.getByTestId('search-input'), { target: { value: 'platform' } })
    await waitFor(() => expect(screen.queryByText('Customer App')).toBeNull())

    act(() => { screen.getByTestId('search-clear').click() })

    await waitFor(() => {
      expect(screen.getByText('Platform API')).toBeTruthy()
      expect(screen.getByText('Customer App')).toBeTruthy()
    })
  })

  it('shows result count when search or chip is active', async () => {
    renderPage()
    await waitFor(() => screen.getByTestId('search-input'))

    fireEvent.change(screen.getByTestId('search-input'), { target: { value: 'platform' } })

    await waitFor(() => {
      expect(screen.getByTestId('result-count')).toBeTruthy()
    })
  })

  it('does not show result count when All chip is active and no search', async () => {
    renderPage()
    await waitFor(() => screen.getByTestId('search-filter-bar'))
    await waitFor(() => {
      expect(screen.queryByTestId('result-count')).toBeNull()
    })
  })

  it('shows now as now', () => {
    // Ensure the `now` variable is within 24h range (sanity check for test helpers)
    expect(Date.now() - new Date(now).getTime()).toBeLessThan(5000)
  })
})
