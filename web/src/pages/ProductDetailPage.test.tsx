import { describe, it, expect, vi, beforeEach, afterEach, type Mock } from 'vitest'
import { render, screen, waitFor, act, cleanup, fireEvent } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import type { Product, Environment, Workload } from '../api/products'
import ProductDetailPage from './ProductDetailPage'

// ─── Mocks ────────────────────────────────────────────────────

const mockListEnvironments = vi.hoisted(() => vi.fn<() => Promise<Environment[]>>())
const mockListWorkloads = vi.hoisted(() => vi.fn<() => Promise<Workload[]>>())

vi.mock('../api/products', () => ({
  listEnvironments: mockListEnvironments,
  listWorkloads: mockListWorkloads,
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
  default: () => (
    <div>
      <span>Product not found</span>
      <button onClick={() => capturedNavigate('/')}>Back to Products</button>
    </div>
  ),
}))

let capturedOnDeploySuccess: ((tag: string, sha: string) => void) | undefined

vi.mock('../components/DeployDialog', () => ({
  DeployDialog: ({ onDeploySuccess }: { onDeploySuccess?: (tag: string, sha: string) => void }) => {
    capturedOnDeploySuccess = onDeploySuccess
    return <div data-testid="deploy-dialog" />
  },
}))

let capturedNavigate: Mock

// Per-test mutable state for router mocks
let mockSlug = 'test-product'
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
    slug: 'test-product',
    description: 'The platform API product',
    created_at: '2025-01-01T00:00:00Z',
    my_role: 'editor',
    last_deployed_at: null,
    has_production_env: false,
    ...overrides,
  }
}

function makeEnvironment(overrides: Partial<Environment> = {}): Environment {
  return {
    id: 'e1',
    product_id: 'p1',
    name: 'development',
    slug: 'development',
    type: 'dev',
    gitops_path: 'apps/development/platform-api/platform-api-helmrelease.yaml',
    created_at: '2025-01-01T00:00:00Z',
    ...overrides,
  }
}

function makeWorkload(overrides: Partial<Workload> = {}): Workload {
  return {
    name: 'api',
    image_repository: 'europe-west1-docker.pkg.dev/acme/platform/api',
    ...overrides,
  }
}

function renderPage() {
  return render(
    <MemoryRouter initialEntries={[`/products/${mockSlug}`]}>
      <ProductDetailPage />
    </MemoryRouter>,
  )
}

// ─── Setup ────────────────────────────────────────────────────

beforeEach(() => {
  vi.clearAllMocks()
  capturedNavigate = vi.fn()
  mockSlug = 'test-product'
  mockLocationState = makeProduct()
  mockUseAuth.mockReturnValue({
    user: { profile: { name: 'Alice' } },
    logout: vi.fn(),
    accessToken: 'test-token',
  })
  mockListEnvironments.mockResolvedValue([])
  mockListWorkloads.mockResolvedValue([])
})

afterEach(() => {
  cleanup()
  vi.useRealTimers()
})

// ─── Tests ────────────────────────────────────────────────────

describe('ProductDetailPage — not found', () => {
  it('renders "product not found" fallback when location.state is missing', () => {
    mockLocationState = undefined

    renderPage()

    expect(screen.getByText(/Product not found/)).toBeTruthy()
    expect(screen.getByText(/Back to Products/)).toBeTruthy()
  })

  it('navigates home when "Back to Products" is clicked in not-found state', () => {
    mockLocationState = undefined

    renderPage()

    act(() => {
      screen.getByText(/Back to Products/).click()
    })

    expect(capturedNavigate).toHaveBeenCalledWith('/')
  })
})

describe('ProductDetailPage — workloads table', () => {
  it('shows workloads table rows when listWorkloads returns data', async () => {
    const env = makeEnvironment({ id: 'e1', name: 'development' })
    const workloads = [
      makeWorkload({ name: 'api', image_repository: 'repo/api' }),
      makeWorkload({ name: 'worker', image_repository: 'repo/worker' }),
    ]
    mockListEnvironments.mockResolvedValue([env])
    mockListWorkloads.mockResolvedValue(workloads)

    renderPage()

    await waitFor(() => {
      expect(screen.getByText('api')).toBeTruthy()
      expect(screen.getByText('worker')).toBeTruthy()
    })
  })

  it('renders image_repository values in the workloads table', async () => {
    const env = makeEnvironment({ id: 'e1' })
    const workload = makeWorkload({ name: 'api', image_repository: 'europe-west1-docker.pkg.dev/acme/platform/api' })
    mockListEnvironments.mockResolvedValue([env])
    mockListWorkloads.mockResolvedValue([workload])

    renderPage()

    await waitFor(() => {
      expect(screen.getByText('europe-west1-docker.pkg.dev/acme/platform/api')).toBeTruthy()
    })
  })

  it('shows empty state when listWorkloads returns empty array', async () => {
    const env = makeEnvironment({ id: 'e1' })
    mockListEnvironments.mockResolvedValue([env])
    mockListWorkloads.mockResolvedValue([])

    renderPage()

    await waitFor(() => {
      expect(screen.getByTestId('empty-workloads')).toBeTruthy()
    })
  })

  it('shows error banner when listEnvironments rejects', async () => {
    mockListEnvironments.mockRejectedValue(new Error('listEnvironments: 500'))

    renderPage()

    await waitFor(() => {
      expect(screen.getByRole('alert')).toBeTruthy()
    })
  })

  it('shows HelmRelease not found state when listWorkloads rejects with 404', async () => {
    const env = makeEnvironment({ id: 'e1' })
    mockListEnvironments.mockResolvedValue([env])
    mockListWorkloads.mockRejectedValue(new Error('listWorkloads: 404'))

    renderPage()

    await waitFor(() => {
      expect(screen.getByTestId('workloads-not-found')).toBeTruthy()
    })
  })

  it('shows error banner when listWorkloads rejects with a non-404 error', async () => {
    const env = makeEnvironment({ id: 'e1' })
    mockListEnvironments.mockResolvedValue([env])
    mockListWorkloads.mockRejectedValue(new Error('listWorkloads: 500'))

    renderPage()

    await waitFor(() => {
      expect(screen.getByRole('alert')).toBeTruthy()
    })
  })

  it('shows no environments state when listEnvironments returns empty array', async () => {
    mockListEnvironments.mockResolvedValue([])

    renderPage()

    await waitFor(() => {
      expect(screen.getByText(/No environments configured/)).toBeTruthy()
    })
  })
})

describe('ProductDetailPage — environment selector', () => {
  it('auto-selects the first environment and calls listWorkloads with its id', async () => {
    const env = makeEnvironment({ id: 'e1', name: 'development' })
    mockListEnvironments.mockResolvedValue([env])
    mockListWorkloads.mockResolvedValue([])

    renderPage()

    await waitFor(() => {
      expect(mockListWorkloads).toHaveBeenCalledWith('test-token', 'test-product', 'e1')
    })
  })

  it('calls listWorkloads with second env id when selector changes', async () => {
    const envs = [
      makeEnvironment({ id: 'e1', name: 'development' }),
      makeEnvironment({ id: 'e2', name: 'production', type: 'production' }),
    ]
    mockListEnvironments.mockResolvedValue(envs)
    mockListWorkloads.mockResolvedValue([])

    renderPage()

    // Wait for selector to appear
    await waitFor(() => screen.getByRole('combobox'))

    await act(async () => {
      fireEvent.change(screen.getByRole('combobox'), { target: { value: 'e2' } })
    })

    await waitFor(() => {
      expect(mockListWorkloads).toHaveBeenCalledWith('test-token', 'test-product', 'e2')
    })
  })

  it('renders an option for each environment in the dropdown', async () => {
    const envs = [
      makeEnvironment({ id: 'e1', name: 'development' }),
      makeEnvironment({ id: 'e2', name: 'production', type: 'production' }),
    ]
    mockListEnvironments.mockResolvedValue(envs)
    mockListWorkloads.mockResolvedValue([])

    renderPage()

    await waitFor(() => {
      expect(screen.getByRole('option', { name: 'development' })).toBeTruthy()
      expect(screen.getByRole('option', { name: 'production' })).toBeTruthy()
    })
  })
})

describe('ProductDetailPage — loading state', () => {
  it('shows loading spinner while listWorkloads is pending', async () => {
    const env = makeEnvironment({ id: 'e1' })
    mockListEnvironments.mockResolvedValue([env])
    // Never resolves
    mockListWorkloads.mockReturnValue(new Promise(() => {}))

    renderPage()

    await waitFor(() => {
      expect(screen.getByText(/Loading workloads/i)).toBeTruthy()
    })
  })
})

describe('ProductDetailPage — RBAC: Deploy button', () => {
  it('shows Deploy button for editor role', async () => {
    mockLocationState = makeProduct({ my_role: 'editor' })
    const env = makeEnvironment({ id: 'e1' })
    const workload = makeWorkload({ name: 'api' })
    mockListEnvironments.mockResolvedValue([env])
    mockListWorkloads.mockResolvedValue([workload])

    renderPage()

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /Deploy api/i })).toBeTruthy()
    })
  })

  it('shows Deploy button for admin role', async () => {
    mockLocationState = makeProduct({ my_role: 'admin' })
    const env = makeEnvironment({ id: 'e1' })
    const workload = makeWorkload({ name: 'api' })
    mockListEnvironments.mockResolvedValue([env])
    mockListWorkloads.mockResolvedValue([workload])

    renderPage()

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /Deploy api/i })).toBeTruthy()
    })
  })

  it('does not show Deploy button for viewer role', async () => {
    mockLocationState = makeProduct({ my_role: 'viewer' })
    const env = makeEnvironment({ id: 'e1' })
    const workload = makeWorkload({ name: 'api' })
    mockListEnvironments.mockResolvedValue([env])
    mockListWorkloads.mockResolvedValue([workload])

    renderPage()

    await waitFor(() => screen.getByText('api'))

    expect(screen.queryByRole('button', { name: /Deploy api/i })).toBeNull()
  })

  it('opens DeployDialog when Deploy button is clicked', async () => {
    mockLocationState = makeProduct({ my_role: 'editor' })
    const env = makeEnvironment({ id: 'e1' })
    const workload = makeWorkload({ name: 'api' })
    mockListEnvironments.mockResolvedValue([env])
    mockListWorkloads.mockResolvedValue([workload])

    renderPage()

    await waitFor(() => screen.getByRole('button', { name: /Deploy api/i }))

    act(() => {
      screen.getByRole('button', { name: /Deploy api/i }).click()
    })

    expect(screen.getByTestId('deploy-dialog')).toBeTruthy()
  })
})

describe('ProductDetailPage — deploy toast', () => {
  it('shows deploy toast with tag and sha after onDeploySuccess is called', async () => {
    mockLocationState = makeProduct({ my_role: 'editor' })
    const env = makeEnvironment({ id: 'e1' })
    const workload = makeWorkload({ name: 'api' })
    mockListEnvironments.mockResolvedValue([env])
    mockListWorkloads.mockResolvedValue([workload])

    renderPage()

    await waitFor(() => screen.getByRole('button', { name: /Deploy api/i }))

    act(() => {
      screen.getByRole('button', { name: /Deploy api/i }).click()
    })

    await waitFor(() => screen.getByTestId('deploy-dialog'))

    act(() => {
      capturedOnDeploySuccess?.('v1.14.2', 'abc1234')
    })

    await waitFor(() => {
      expect(screen.getByTestId('deploy-toast')).toBeTruthy()
      expect(screen.getByText(/v1\.14\.2/)).toBeTruthy()
      expect(screen.getByText(/abc1234/)).toBeTruthy()
    })
  })

  it('hides deploy toast after 6 seconds', async () => {
    mockLocationState = makeProduct({ my_role: 'editor' })
    const env = makeEnvironment({ id: 'e1' })
    const workload = makeWorkload({ name: 'api' })
    mockListEnvironments.mockResolvedValue([env])
    mockListWorkloads.mockResolvedValue([workload])

    renderPage()

    await waitFor(() => screen.getByRole('button', { name: /Deploy api/i }))

    vi.useFakeTimers()

    act(() => {
      screen.getByRole('button', { name: /Deploy api/i }).click()
    })

    act(() => {
      capturedOnDeploySuccess?.('v2.0.0', 'deadbeef')
    })

    expect(screen.getByTestId('deploy-toast')).toBeTruthy()

    act(() => {
      vi.advanceTimersByTime(6000)
    })

    expect(screen.queryByTestId('deploy-toast')).toBeNull()
  })

  it('hides deploy toast when close button is clicked', async () => {
    mockLocationState = makeProduct({ my_role: 'editor' })
    const env = makeEnvironment({ id: 'e1' })
    const workload = makeWorkload({ name: 'api' })
    mockListEnvironments.mockResolvedValue([env])
    mockListWorkloads.mockResolvedValue([workload])

    renderPage()

    await waitFor(() => screen.getByRole('button', { name: /Deploy api/i }))

    act(() => {
      screen.getByRole('button', { name: /Deploy api/i }).click()
    })

    act(() => {
      capturedOnDeploySuccess?.('v1.0.0', 'cafebabe')
    })

    await waitFor(() => screen.getByTestId('deploy-toast'))

    act(() => {
      screen.getByRole('button', { name: /Chiudi notifica/i }).click()
    })

    await waitFor(() => expect(screen.queryByTestId('deploy-toast')).toBeNull())
  })
})

describe('ProductDetailPage — API calls', () => {
  it('calls listEnvironments with token and product slug', async () => {
    renderPage()

    await waitFor(() => {
      expect(mockListEnvironments).toHaveBeenCalledWith('test-token', 'test-product')
    })
  })

  it('calls listWorkloads with token, slug, and first env id once environments load', async () => {
    const env = makeEnvironment({ id: 'e1' })
    mockListEnvironments.mockResolvedValue([env])
    mockListWorkloads.mockResolvedValue([])

    renderPage()

    await waitFor(() => {
      expect(mockListWorkloads).toHaveBeenCalledWith('test-token', 'test-product', 'e1')
    })
  })

  it('does not call listWorkloads when there are no environments', async () => {
    mockListEnvironments.mockResolvedValue([])

    renderPage()

    await waitFor(() => screen.getByText(/No environments configured/))

    expect(mockListWorkloads).not.toHaveBeenCalled()
  })
})
