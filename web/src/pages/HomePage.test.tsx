import { describe, it, expect, vi, beforeEach, afterEach, type Mock } from 'vitest'
import { render, screen, waitFor, act, cleanup } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import type { Product } from '../api/products'
import HomePage from './HomePage'

// ─── Mocks ────────────────────────────────────────────────────

const mockListProducts = vi.hoisted(() => vi.fn<() => Promise<Product[]>>())

vi.mock('../api/products', () => ({
  listProducts: mockListProducts,
}))

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
