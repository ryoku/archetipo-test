import { describe, it, expect, vi, beforeEach, afterEach, type Mock } from 'vitest'
import { render, screen, waitFor, act, cleanup, fireEvent } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import type { Product, Component } from '../api/products'
import ProductDetailPage from './ProductDetailPage'

// ─── Mocks ────────────────────────────────────────────────────

const mockListComponents = vi.hoisted(() => vi.fn<() => Promise<Component[]>>())
const mockCreateComponent = vi.hoisted(() => vi.fn<() => Promise<Component>>())
const mockDeleteComponent = vi.hoisted(() => vi.fn<() => Promise<void>>())

vi.mock('../api/products', () => ({
  listComponents: mockListComponents,
  createComponent: mockCreateComponent,
  deleteComponent: mockDeleteComponent,
}))

const mockUseAuth = vi.fn()

vi.mock('../auth/AuthContext', () => ({
  useAuth: () => mockUseAuth(),
}))

let capturedNavigate: Mock

// We need to be able to change what useParams and useLocation return per test
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
    ...overrides,
  }
}

function makeComponent(overrides: Partial<Component> = {}): Component {
  return {
    id: 'c1',
    product_id: 'p1',
    name: 'api',
    slug: 'api',
    gcr_image_path: 'europe-west1-docker.pkg.dev/acme/platform/api',
    created_at: '2025-01-01T00:00:00Z',
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
  mockListComponents.mockResolvedValue([])
  mockCreateComponent.mockResolvedValue(makeComponent())
  mockDeleteComponent.mockResolvedValue(undefined)
})

afterEach(cleanup)

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

describe('ProductDetailPage — with product', () => {
  it('shows product name when state is provided', () => {
    renderPage()

    expect(screen.getByText('Platform API')).toBeTruthy()
  })

  it('shows product slug as tag chip', async () => {
    renderPage()

    // slug appears in multiple places (breadcrumb + tag chip); use getAllBy and check at least one
    await waitFor(() => {
      const elements = screen.getAllByText('test-product')
      expect(elements.length).toBeGreaterThan(0)
    })
  })

  it('renders component table when listComponents returns data', async () => {
    const components = [
      makeComponent({ id: 'c1', name: 'api', slug: 'api' }),
      makeComponent({ id: 'c2', name: 'worker', slug: 'worker-svc', gcr_image_path: 'path/worker' }),
    ]
    mockListComponents.mockResolvedValue(components)

    renderPage()

    // Each name appears in name+slug cells; use getAllByText
    await waitFor(() => {
      const apiNames = screen.getAllByText('api')
      expect(apiNames.length).toBeGreaterThan(0)
      expect(screen.getByText('worker')).toBeTruthy()
    })
  })

  it('shows GCR image path in the table', async () => {
    const comp = makeComponent({
      gcr_image_path: 'europe-west1-docker.pkg.dev/acme/platform/api',
    })
    mockListComponents.mockResolvedValue([comp])

    renderPage()

    await waitFor(() => {
      expect(screen.getByText('europe-west1-docker.pkg.dev/acme/platform/api')).toBeTruthy()
    })
  })

  it('shows empty state when no components', async () => {
    mockListComponents.mockResolvedValue([])

    renderPage()

    await waitFor(() => {
      expect(screen.getByTestId('empty-components')).toBeTruthy()
    })
  })

  it('shows add component form when "Add Component" is clicked', async () => {
    renderPage()

    await waitFor(() => screen.getByText('Add Component'))

    act(() => {
      screen.getByText('Add Component').click()
    })

    expect(screen.getByText('New Component')).toBeTruthy()
    expect(screen.getByLabelText('Name *')).toBeTruthy()
    expect(screen.getByLabelText('Slug *')).toBeTruthy()
    expect(screen.getByLabelText('GCR Image Path *')).toBeTruthy()
  })

  it('calls createComponent on form submit and adds to list', async () => {
    const newComp = makeComponent({ id: 'c99', name: 'frontend', slug: 'frontend', gcr_image_path: 'path/frontend' })
    mockCreateComponent.mockResolvedValue(newComp)
    mockListComponents.mockResolvedValue([])

    renderPage()

    await waitFor(() => screen.getByText('Add Component'))

    act(() => {
      screen.getByText('Add Component').click()
    })

    const nameInput = screen.getByLabelText('Name *')
    const slugInput = screen.getByLabelText('Slug *')
    const gcrInput = screen.getByLabelText('GCR Image Path *')

    // Use fireEvent.change to properly trigger React's synthetic events
    await act(async () => {
      fireEvent.change(nameInput, { target: { value: 'frontend' } })
      fireEvent.change(slugInput, { target: { value: 'frontend' } })
      fireEvent.change(gcrInput, { target: { value: 'path/frontend' } })
    })

    await act(async () => {
      screen.getByText('Save Component').click()
    })

    await waitFor(() => {
      expect(mockCreateComponent).toHaveBeenCalledWith('test-token', 'test-product', {
        name: 'frontend',
        slug: 'frontend',
        gcr_image_path: 'path/frontend',
      })
    })
  })

  it('adds the new component to the list after successful create', async () => {
    const newComp = makeComponent({ id: 'c99', name: 'frontend', slug: 'frontend', gcr_image_path: 'path/frontend' })
    mockCreateComponent.mockResolvedValue(newComp)
    mockListComponents.mockResolvedValue([])

    renderPage()

    await waitFor(() => screen.getByText('Add Component'))

    act(() => {
      screen.getByText('Add Component').click()
    })

    await act(async () => {
      fireEvent.change(screen.getByLabelText('Name *'), { target: { value: 'frontend' } })
      fireEvent.change(screen.getByLabelText('Slug *'), { target: { value: 'frontend' } })
      fireEvent.change(screen.getByLabelText('GCR Image Path *'), { target: { value: 'path/frontend' } })
    })

    await act(async () => {
      screen.getByText('Save Component').click()
    })

    await waitFor(() => {
      const els = screen.getAllByText('frontend')
      expect(els.length).toBeGreaterThan(0)
    })
  })

  it('hides the form when Cancel is clicked', async () => {
    renderPage()

    await waitFor(() => screen.getByText('Add Component'))

    act(() => {
      screen.getByText('Add Component').click()
    })

    expect(screen.getByText('New Component')).toBeTruthy()

    act(() => {
      screen.getByText('Cancel').click()
    })

    expect(screen.queryByText('New Component')).toBeNull()
  })

  it('calls listComponents with the product slug and token', async () => {
    renderPage()

    await waitFor(() => {
      expect(mockListComponents).toHaveBeenCalledWith('test-token', 'test-product')
    })
  })

  it('resets to loading state and clears stale components when slug changes', async () => {
    // First render: foo product with one distinctly-named component
    mockSlug = 'foo'
    mockLocationState = makeProduct({ slug: 'foo' })
    const fooComp = makeComponent({ id: 'c-foo', name: 'Foo Service', slug: 'foo-svc' })
    mockListComponents.mockResolvedValueOnce([fooComp])

    const { rerender } = renderPage()

    await waitFor(() => expect(screen.getByText('Foo Service')).toBeTruthy())

    // Simulate navigation to a different product (slug changes, new fetch never resolves)
    mockSlug = 'bar'
    mockLocationState = makeProduct({ slug: 'bar' })
    mockListComponents.mockReturnValueOnce(new Promise(() => {}))

    rerender(
      <MemoryRouter initialEntries={['/products/bar']}>
        <ProductDetailPage />
      </MemoryRouter>,
    )

    // Stale component from 'foo' must NOT be visible while 'bar' is loading
    await waitFor(() => expect(screen.queryByText('Foo Service')).toBeNull())
  })
})

describe('ProductDetailPage — delete confirm dialog', () => {
  it('renders delete confirmation as a native <dialog> element', async () => {
    const comp = makeComponent({ name: 'api-gw', slug: 'api-gw' })
    mockListComponents.mockResolvedValue([comp])
    renderPage()
    await waitFor(() => screen.getByRole('button', { name: /Delete api-gw/i }))
    act(() => { screen.getByRole('button', { name: /Delete api-gw/i }).click() })
    const dialog = screen.getByRole('dialog')
    expect(dialog.tagName.toLowerCase()).toBe('dialog')
  })

  it('closes confirm dialog when Escape key is pressed on window', async () => {
    const comp = makeComponent({ name: 'api-gw', slug: 'api-gw' })
    mockListComponents.mockResolvedValue([comp])
    renderPage()
    await waitFor(() => screen.getByRole('button', { name: /Delete api-gw/i }))
    act(() => { screen.getByRole('button', { name: /Delete api-gw/i }).click() })
    expect(screen.getByRole('dialog')).toBeTruthy()
    // Escape must work even when the dialog is not focused — window-level listener required
    act(() => { fireEvent.keyDown(window, { key: 'Escape' }) })
    expect(screen.queryByRole('dialog')).toBeNull()
  })

  it('shows delete confirm dialog when Delete button is clicked', async () => {
    const comp = makeComponent({ name: 'api-gw', slug: 'api-gw' })
    mockListComponents.mockResolvedValue([comp])

    renderPage()

    await waitFor(() => screen.getByRole('button', { name: /Delete api-gw/i }))

    act(() => {
      screen.getByRole('button', { name: /Delete api-gw/i }).click()
    })

    expect(screen.getByText('Remove Component')).toBeTruthy()
    expect(screen.getByText(/cannot be undone/i)).toBeTruthy()
  })

  it('closes dialog when Cancel is clicked', async () => {
    const comp = makeComponent({ name: 'api-gw', slug: 'api-gw' })
    mockListComponents.mockResolvedValue([comp])

    renderPage()

    await waitFor(() => screen.getByRole('button', { name: /Delete api-gw/i }))

    act(() => {
      screen.getByRole('button', { name: /Delete api-gw/i }).click()
    })

    expect(screen.getByText('Remove Component')).toBeTruthy()

    act(() => {
      // Click the Cancel button inside the dialog footer (last one in the DOM)
      const cancelButtons = screen.getAllByText('Cancel')
      cancelButtons[cancelButtons.length - 1].click()
    })

    expect(screen.queryByText('Remove Component')).toBeNull()
  })

  it('calls deleteComponent and removes from list on confirm', async () => {
    const comp = makeComponent({ id: 'c1', name: 'api-gw', slug: 'api-gw' })
    mockListComponents.mockResolvedValue([comp])
    mockDeleteComponent.mockResolvedValue(undefined)

    renderPage()

    await waitFor(() => screen.getByRole('button', { name: /Delete api-gw/i }))

    act(() => {
      screen.getByRole('button', { name: /Delete api-gw/i }).click()
    })

    await act(async () => {
      screen.getByText('Delete Component').click()
    })

    await waitFor(() => {
      expect(mockDeleteComponent).toHaveBeenCalledWith('test-token', 'test-product', 'api-gw')
    })

    // Component should be removed from the list
    await waitFor(() => {
      expect(screen.getByTestId('empty-components')).toBeTruthy()
    })
  })
})
