import { describe, it, expect, vi, beforeEach, afterEach, type Mock } from 'vitest'
import { render, screen, waitFor, act, cleanup, fireEvent } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import type { Product, DeploymentHistoryResponse } from '../api/products'
import HistoryPage from './HistoryPage'

// ─── Mocks ────────────────────────────────────────────────────

const mockListDeployments = vi.hoisted(() => vi.fn<() => Promise<DeploymentHistoryResponse>>())

vi.mock('../api/products', () => ({
  listDeployments: mockListDeployments,
}))

const mockUseAuth = vi.fn()

vi.mock('../auth/AuthContext', () => ({
  useAuth: () => mockUseAuth(),
}))

vi.mock('../components/ProductHero', () => ({
  default: ({ product }: { product: Product }) => <div data-testid="product-hero">{product.name}</div>,
}))

vi.mock('../components/ProductSubNav', () => ({
  default: () => <div data-testid="product-sub-nav" />,
}))

vi.mock('../components/ProductNotFound', () => ({
  default: () => <div data-testid="product-not-found">Product not found</div>,
}))

let capturedNavigate: Mock
let mockSlug = 'my-product'
let mockLocationState: Product | undefined

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
    slug: 'my-product',
    description: '',
    created_at: '2025-01-01T00:00:00Z',
    my_role: 'viewer',
    ...overrides,
  }
}

function makeHistoryResponse(count: number, total = count, page = 1): DeploymentHistoryResponse {
  const sha = 'abc123'
  return {
    deployments: Array.from({ length: count }, (_, i) => ({
      id: `id-${i}`,
      actor_display_name: `Alice Rossi`,
      component_name: 'api',
      environment_name: i % 3 === 0 ? 'production' : i % 3 === 1 ? 'staging' : 'development',
      tag: `v1.0.${i}`,
      deployed_at: '2026-06-01T12:00:00Z',
      commit_sha: sha,
      outcome: i % 2 === 0 ? 'success' : 'failure',
    })),
    page,
    page_size: 20,
    total,
  }
}

function renderPage() {
  return render(
    <MemoryRouter>
      <HistoryPage />
    </MemoryRouter>,
  )
}

// ─── Setup ────────────────────────────────────────────────────

beforeEach(() => {
  vi.clearAllMocks()
  capturedNavigate = vi.fn()
  mockSlug = 'my-product'
  mockLocationState = makeProduct()
  mockUseAuth.mockReturnValue({ accessToken: 'test-token' })
  mockListDeployments.mockResolvedValue(makeHistoryResponse(0))
})

afterEach(cleanup)

// ─── Tests ────────────────────────────────────────────────────

describe('HistoryPage — no product state', () => {
  it('renders ProductNotFound when location.state is missing', () => {
    mockLocationState = undefined
    renderPage()
    expect(screen.getByTestId('product-not-found')).toBeTruthy()
  })
})

describe('HistoryPage — loading state', () => {
  it('shows loading spinner while fetching', () => {
    mockListDeployments.mockReturnValue(new Promise(() => {}))
    renderPage()
    expect(screen.getByText(/Loading deployment history/i)).toBeTruthy()
  })
})

describe('HistoryPage — error state', () => {
  it('shows error message when fetch fails', async () => {
    mockListDeployments.mockRejectedValue(new Error('network error'))
    renderPage()
    await waitFor(() => {
      expect(screen.getByRole('alert')).toBeTruthy()
    })
    expect(screen.getByText(/network error/i)).toBeTruthy()
  })

  it('shows fallback message for non-Error rejections', async () => {
    mockListDeployments.mockRejectedValue('boom')
    renderPage()
    await waitFor(() => {
      expect(screen.getByRole('alert')).toBeTruthy()
    })
    expect(screen.getByText(/Failed to load deployment history/i)).toBeTruthy()
  })
})

describe('HistoryPage — empty list', () => {
  it('shows "No deployments yet" when list is empty', async () => {
    mockListDeployments.mockResolvedValue(makeHistoryResponse(0))
    renderPage()
    await waitFor(() => {
      expect(screen.getByText(/No deployments yet/i)).toBeTruthy()
    })
  })

  it('shows "0 deployments recorded" count', async () => {
    mockListDeployments.mockResolvedValue(makeHistoryResponse(0))
    renderPage()
    await waitFor(() => {
      expect(screen.getByText(/0 deployments recorded/i)).toBeTruthy()
    })
  })
})

describe('HistoryPage — data', () => {
  it('renders deployment rows', async () => {
    mockListDeployments.mockResolvedValue(makeHistoryResponse(3))
    renderPage()
    await waitFor(() => {
      expect(screen.getAllByText('Alice Rossi')).toHaveLength(3)
    })
  })

  it('renders product hero and subnav', async () => {
    mockListDeployments.mockResolvedValue(makeHistoryResponse(1))
    renderPage()
    await waitFor(() => {
      expect(screen.getByTestId('product-hero')).toBeTruthy()
      expect(screen.getByTestId('product-sub-nav')).toBeTruthy()
    })
  })

  it('shows total count', async () => {
    mockListDeployments.mockResolvedValue(makeHistoryResponse(3, 15))
    renderPage()
    await waitFor(() => {
      expect(screen.getByText(/15 deployments recorded/i)).toBeTruthy()
    })
  })

  it('renders success outcome badge', async () => {
    mockListDeployments.mockResolvedValue({
      ...makeHistoryResponse(1),
      deployments: [{ ...makeHistoryResponse(1).deployments[0], outcome: 'success' }],
    })
    renderPage()
    await waitFor(() => {
      expect(screen.getByText('success')).toBeTruthy()
    })
  })

  it('renders failure outcome badge', async () => {
    mockListDeployments.mockResolvedValue({
      ...makeHistoryResponse(1),
      deployments: [{ ...makeHistoryResponse(1).deployments[0], outcome: 'failure' }],
    })
    renderPage()
    await waitFor(() => {
      expect(screen.getByText('failure')).toBeTruthy()
    })
  })

  it('renders env badge for production env', async () => {
    mockListDeployments.mockResolvedValue({
      ...makeHistoryResponse(1),
      deployments: [{ ...makeHistoryResponse(1).deployments[0], environment_name: 'production' }],
    })
    renderPage()
    await waitFor(() => {
      expect(screen.getByText('production')).toBeTruthy()
    })
  })

  it('renders env badge for integration env', async () => {
    mockListDeployments.mockResolvedValue({
      ...makeHistoryResponse(1),
      deployments: [{ ...makeHistoryResponse(1).deployments[0], environment_name: 'integration' }],
    })
    renderPage()
    await waitFor(() => {
      expect(screen.getByText('integration')).toBeTruthy()
    })
  })

  it('renders actor initials in avatar', async () => {
    mockListDeployments.mockResolvedValue({
      ...makeHistoryResponse(1),
      deployments: [{ ...makeHistoryResponse(1).deployments[0], actor_display_name: 'Alice Rossi' }],
    })
    renderPage()
    await waitFor(() => {
      expect(screen.getByText('AR')).toBeTruthy()
    })
  })

  it('navigates home when breadcrumb Products is clicked', async () => {
    mockListDeployments.mockResolvedValue(makeHistoryResponse(0))
    renderPage()
    await waitFor(() => screen.getByText('Products'))
    act(() => { screen.getByText('Products').click() })
    expect(capturedNavigate).toHaveBeenCalledWith('/')
  })
})

describe('HistoryPage — pagination', () => {
  it('does not render pagination when no deployments', async () => {
    mockListDeployments.mockResolvedValue(makeHistoryResponse(0))
    renderPage()
    await waitFor(() => {
      expect(screen.queryByText('Previous')).toBeNull()
    })
  })

  it('renders pagination when total > 0', async () => {
    mockListDeployments.mockResolvedValue(makeHistoryResponse(3, 25))
    renderPage()
    await waitFor(() => {
      expect(screen.getByText('Previous')).toBeTruthy()
      expect(screen.getByText('Next')).toBeTruthy()
    })
  })

  it('Previous button is disabled on page 1', async () => {
    mockListDeployments.mockResolvedValue(makeHistoryResponse(3, 25))
    renderPage()
    await waitFor(() => {
      expect(screen.getByText('Previous').closest('button')).toBeDisabled()
    })
  })

  it('Next button is enabled when more pages exist', async () => {
    mockListDeployments.mockResolvedValue(makeHistoryResponse(3, 25))
    renderPage()
    await waitFor(() => {
      expect(screen.getByText('Next').closest('button')).not.toBeDisabled()
    })
  })

  it('clicking Next fetches the next page', async () => {
    mockListDeployments.mockResolvedValue(makeHistoryResponse(3, 25))
    renderPage()
    await waitFor(() => screen.getByText('Next'))
    act(() => { fireEvent.click(screen.getByRole('button', { name: /next/i })) })
    await waitFor(() => {
      expect(mockListDeployments).toHaveBeenCalledWith('test-token', 'my-product', 2)
    })
  })

  it('skips fetch when accessToken is missing', () => {
    mockUseAuth.mockReturnValue({ accessToken: null })
    renderPage()
    expect(mockListDeployments).not.toHaveBeenCalled()
  })
})
