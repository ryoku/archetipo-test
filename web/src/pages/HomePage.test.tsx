import { describe, it, expect, vi, beforeEach, afterEach, type Mock } from 'vitest'
import { render, screen, waitFor, act, cleanup, fireEvent } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import type { Product } from '../api/products'
import HomePage from './HomePage'

// ─── Mocks ────────────────────────────────────────────────────

const mockListProducts = vi.hoisted(() => vi.fn<() => Promise<Product[]>>())
const mockCreateProduct = vi.hoisted(() => vi.fn<() => Promise<Product>>())

vi.mock('../api/products', () => ({
  listProducts: mockListProducts,
  createProduct: mockCreateProduct,
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
})
