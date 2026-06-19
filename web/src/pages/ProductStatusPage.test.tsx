import { describe, it, expect, vi, beforeEach, afterEach, type Mock } from 'vitest'
import { render, screen, waitFor, cleanup, fireEvent } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import type { Product, StatusResponse } from '../api/products'
import ProductStatusPage from './ProductStatusPage'

// ─── Mocks ────────────────────────────────────────────────────

const mockGetProductStatus = vi.hoisted(() => vi.fn<() => Promise<StatusResponse>>())

vi.mock('../api/products', () => ({
  getProductStatus: mockGetProductStatus,
}))

const mockUseAuth = vi.fn()
vi.mock('../auth/AuthContext', () => ({
  useAuth: () => mockUseAuth(),
}))

let capturedNavigate: Mock
let mockSlug = 'platform-api'
let mockLocationState: Product | undefined = undefined

vi.mock('react-router-dom', async (importOriginal) => {
  const actual = await importOriginal<typeof import('react-router-dom')>()
  return {
    ...actual,
    useNavigate: () => capturedNavigate,
    useParams: () => ({ slug: mockSlug }),
    useLocation: () => ({ state: mockLocationState }),
  }
})

// ─── Helpers ──────────────────────────────────────────────────

function makeProduct(overrides: Partial<Product> = {}): Product {
  return {
    id: 'p1',
    name: 'Platform API',
    slug: 'platform-api',
    description: '',
    created_at: '2025-01-01T00:00:00Z',
    my_role: 'viewer',
    ...overrides,
  }
}

function makeStatus(overrides: Partial<StatusResponse> = {}): StatusResponse {
  return {
    workloads: {
      api: { dev: 'v1.2.0', production: 'v1.1.0' },
      worker: { dev: 'v1.0.0', production: 'N/A' },
    },
    fetched_at: '2025-01-01T10:00:00Z',
    stale: false,
    ...overrides,
  }
}

function renderPage() {
  return render(
    <MemoryRouter initialEntries={[`/products/${mockSlug}/status`]}>
      <ProductStatusPage />
    </MemoryRouter>,
  )
}

// ─── Setup ────────────────────────────────────────────────────

beforeEach(() => {
  vi.clearAllMocks()
  capturedNavigate = vi.fn()
  mockSlug = 'platform-api'
  mockLocationState = makeProduct()
  mockUseAuth.mockReturnValue({ accessToken: 'test-token' })
  mockGetProductStatus.mockResolvedValue(makeStatus())
})

afterEach(cleanup)

// ─── Tests ────────────────────────────────────────────────────

describe('ProductStatusPage — not found', () => {
  it('renders ProductNotFound when location.state is missing', () => {
    mockLocationState = undefined
    renderPage()
    expect(screen.getByText(/Product not found/i)).toBeTruthy()
  })
})

describe('ProductStatusPage — loading state', () => {
  it('shows spinner while fetching', () => {
    mockGetProductStatus.mockReturnValue(new Promise(() => {}))
    renderPage()
    expect(screen.getByText(/Loading deployment status/i)).toBeTruthy()
  })
})

describe('ProductStatusPage — status matrix', () => {
  it('renders workload rows and environment columns', async () => {
    renderPage()
    await waitFor(() => {
      expect(screen.getByTestId('status-matrix')).toBeTruthy()
    })
    expect(screen.getByText('api')).toBeTruthy()
    expect(screen.getByText('worker')).toBeTruthy()
    expect(screen.getByTestId('tag-api-dev').textContent).toBe('v1.2.0')
    expect(screen.getByTestId('tag-api-production').textContent).toBe('v1.1.0')
  })

  it('renders N/A chip for missing tags', async () => {
    renderPage()
    await waitFor(() => {
      expect(screen.getByTestId('tag-worker-production').textContent).toBe('N/A')
    })
  })

  it('shows stale badge when stale is true', async () => {
    mockGetProductStatus.mockResolvedValue(makeStatus({ stale: true }))
    renderPage()
    await waitFor(() => {
      expect(screen.getByTestId('stale-badge')).toBeTruthy()
    })
  })

  it('does not show stale badge when stale is false', async () => {
    renderPage()
    await waitFor(() => {
      expect(screen.getByTestId('status-matrix')).toBeTruthy()
    })
    expect(screen.queryByTestId('stale-badge')).toBeNull()
  })

  it('shows empty state when workloads map is empty', async () => {
    mockGetProductStatus.mockResolvedValue(makeStatus({ workloads: {} }))
    renderPage()
    await waitFor(() => {
      expect(screen.getByTestId('status-empty')).toBeTruthy()
    })
  })
})

describe('ProductStatusPage — refresh', () => {
  it('re-fetches when Refresh is clicked', async () => {
    renderPage()
    await waitFor(() => {
      expect(screen.getByTestId('status-refresh')).toBeTruthy()
    })
    fireEvent.click(screen.getByTestId('status-refresh'))
    await waitFor(() => {
      expect(mockGetProductStatus).toHaveBeenCalledTimes(2)
    })
  })
})

describe('ProductStatusPage — error state', () => {
  it('shows error message and retry button on fetch failure', async () => {
    mockGetProductStatus.mockRejectedValue(new Error('getProductStatus: 500'))
    renderPage()
    await waitFor(() => {
      expect(screen.getByTestId('status-error')).toBeTruthy()
    })
    expect(screen.getByText(/getProductStatus: 500/i)).toBeTruthy()
    expect(screen.getByTestId('status-retry')).toBeTruthy()
  })

  it('re-fetches when Retry is clicked after error', async () => {
    mockGetProductStatus
      .mockRejectedValueOnce(new Error('network error'))
      .mockResolvedValue(makeStatus())
    renderPage()
    await waitFor(() => {
      expect(screen.getByTestId('status-retry')).toBeTruthy()
    })
    fireEvent.click(screen.getByTestId('status-retry'))
    await waitFor(() => {
      expect(screen.getByTestId('status-matrix')).toBeTruthy()
    })
  })
})
