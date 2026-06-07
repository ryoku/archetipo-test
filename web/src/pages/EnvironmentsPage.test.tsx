import { describe, it, expect, vi, beforeEach, afterEach, type Mock } from 'vitest'
import { render, screen, waitFor, act, cleanup, fireEvent } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import type { Product, Environment } from '../api/products'
import EnvironmentsPage from './EnvironmentsPage'

// ─── Mocks ────────────────────────────────────────────────────

const mockListEnvironments = vi.hoisted(() => vi.fn<() => Promise<Environment[]>>())
const mockCreateEnvironment = vi.hoisted(() => vi.fn<() => Promise<Environment>>())
const mockDeleteEnvironment = vi.hoisted(() => vi.fn<() => Promise<void>>())

vi.mock('../api/products', () => ({
  listEnvironments: mockListEnvironments,
  createEnvironment: mockCreateEnvironment,
  deleteEnvironment: mockDeleteEnvironment,
}))

const mockUseAuth = vi.fn()

vi.mock('../auth/AuthContext', () => ({
  useAuth: () => mockUseAuth(),
}))

let capturedNavigate: Mock

// Per-test mutable state for router mocks
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
    my_role: 'admin',
    ...overrides,
  }
}

function makeEnvironment(overrides: Partial<Environment> = {}): Environment {
  return {
    id: 'e1',
    product_id: 'p1',
    name: 'development',
    type: 'dev',
    overlay_path: 'overlays/dev',
    created_at: '2025-11-14T10:00:00Z',
    ...overrides,
  }
}

const mockEnvs: Environment[] = [
  { id: 'e1', product_id: 'p1', name: 'development', type: 'dev', overlay_path: 'overlays/dev', created_at: '2025-11-14T10:00:00Z' },
  { id: 'e2', product_id: 'p1', name: 'production', type: 'production', overlay_path: 'overlays/prod', created_at: '2025-11-20T10:00:00Z' },
]

function renderPage() {
  return render(
    <MemoryRouter initialEntries={[`/products/${mockSlug}/environments`]}>
      <EnvironmentsPage />
    </MemoryRouter>,
  )
}

// ─── Setup ────────────────────────────────────────────────────

beforeEach(() => {
  vi.clearAllMocks()
  capturedNavigate = vi.fn()
  mockSlug = 'platform-api'
  mockLocationState = makeProduct()
  mockUseAuth.mockReturnValue({
    user: { profile: { name: 'Alice' } },
    logout: vi.fn(),
    accessToken: 'test-token',
  })
  mockListEnvironments.mockResolvedValue([])
  mockCreateEnvironment.mockResolvedValue(makeEnvironment())
  mockDeleteEnvironment.mockResolvedValue(undefined)
})

afterEach(cleanup)

// ─── Tests ────────────────────────────────────────────────────

describe('EnvironmentsPage — not found', () => {
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

describe('EnvironmentsPage — admin view', () => {
  it('shows "Add Environment" button when my_role is admin', async () => {
    mockLocationState = makeProduct({ my_role: 'admin' })

    renderPage()

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /add environment/i })).toBeTruthy()
    })
  })

  it('renders delete button for each environment row when my_role is admin', async () => {
    mockLocationState = makeProduct({ my_role: 'admin' })
    mockListEnvironments.mockResolvedValue(mockEnvs)

    renderPage()

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /delete development/i })).toBeTruthy()
      expect(screen.getByRole('button', { name: /delete production/i })).toBeTruthy()
    })
  })

  it('renders environment rows in table', async () => {
    mockLocationState = makeProduct({ my_role: 'admin' })
    mockListEnvironments.mockResolvedValue(mockEnvs)

    renderPage()

    await waitFor(() => {
      expect(screen.getByText('development')).toBeTruthy()
      expect(screen.getAllByText('production')[0]).toBeTruthy()
    })
  })
})

describe('EnvironmentsPage — non-admin view', () => {
  it('does not show "Add Environment" button when my_role is editor', async () => {
    mockLocationState = makeProduct({ my_role: 'editor' })
    mockListEnvironments.mockResolvedValue(mockEnvs)

    renderPage()

    await waitFor(() => screen.getByText('development'))

    expect(screen.queryByRole('button', { name: /add environment/i })).toBeNull()
  })

  it('does not render delete buttons when my_role is editor', async () => {
    mockLocationState = makeProduct({ my_role: 'editor' })
    mockListEnvironments.mockResolvedValue(mockEnvs)

    renderPage()

    await waitFor(() => screen.getByText('development'))

    expect(screen.queryByRole('button', { name: /delete development/i })).toBeNull()
    expect(screen.queryByRole('button', { name: /delete production/i })).toBeNull()
  })

  it('shows environment table in read-only mode for editor', async () => {
    mockLocationState = makeProduct({ my_role: 'editor' })
    mockListEnvironments.mockResolvedValue(mockEnvs)

    renderPage()

    await waitFor(() => {
      expect(screen.getByText('development')).toBeTruthy()
      expect(screen.getAllByText('production')[0]).toBeTruthy()
    })

    // No Add Environment button and no Delete buttons — read-only table
    expect(screen.queryByRole('button', { name: /add environment/i })).toBeNull()
  })

  it('does not show "Add Environment" button when my_role is viewer', async () => {
    mockLocationState = makeProduct({ my_role: 'viewer' })

    renderPage()

    await waitFor(() => screen.getByTestId('empty-environments'))

    expect(screen.queryByRole('button', { name: /add environment/i })).toBeNull()
  })
})

describe('EnvironmentsPage — create flow (admin)', () => {
  beforeEach(() => {
    mockLocationState = makeProduct({ my_role: 'admin' })
    mockListEnvironments.mockResolvedValue([])
  })

  it('shows inline form when "Add Environment" is clicked', async () => {
    renderPage()

    await waitFor(() => screen.getByRole('button', { name: /add environment/i }))

    act(() => {
      screen.getByRole('button', { name: /add environment/i }).click()
    })

    expect(screen.getByText('New Environment')).toBeTruthy()
    expect(screen.getByLabelText('Name *')).toBeTruthy()
    expect(screen.getByLabelText('Type *')).toBeTruthy()
    expect(screen.getByLabelText('Overlay Path *')).toBeTruthy()
  })

  it('calls createEnvironment with correct args on form submit', async () => {
    const newEnv = makeEnvironment({ id: 'e99', name: 'staging', type: 'integration', overlay_path: 'overlays/staging' })
    mockCreateEnvironment.mockResolvedValue(newEnv)

    renderPage()

    await waitFor(() => screen.getByRole('button', { name: /add environment/i }))

    act(() => {
      screen.getByRole('button', { name: /add environment/i }).click()
    })

    await act(async () => {
      fireEvent.change(screen.getByLabelText('Name *'), { target: { value: 'staging' } })
      fireEvent.change(screen.getByLabelText('Type *'), { target: { value: 'integration' } })
      fireEvent.change(screen.getByLabelText('Overlay Path *'), { target: { value: 'overlays/staging' } })
    })

    await act(async () => {
      screen.getByText('Save Environment').click()
    })

    await waitFor(() => {
      expect(mockCreateEnvironment).toHaveBeenCalledWith('test-token', 'platform-api', {
        name: 'staging',
        type: 'integration',
        overlay_path: 'overlays/staging',
      })
    })
  })

  it('adds new environment to table after successful create (no page reload)', async () => {
    const newEnv = makeEnvironment({ id: 'e99', name: 'staging', type: 'integration', overlay_path: 'overlays/staging' })
    mockCreateEnvironment.mockResolvedValue(newEnv)

    renderPage()

    await waitFor(() => screen.getByRole('button', { name: /add environment/i }))

    act(() => {
      screen.getByRole('button', { name: /add environment/i }).click()
    })

    await act(async () => {
      fireEvent.change(screen.getByLabelText('Name *'), { target: { value: 'staging' } })
      fireEvent.change(screen.getByLabelText('Type *'), { target: { value: 'integration' } })
      fireEvent.change(screen.getByLabelText('Overlay Path *'), { target: { value: 'overlays/staging' } })
    })

    await act(async () => {
      screen.getByText('Save Environment').click()
    })

    await waitFor(() => {
      expect(screen.getByText('staging')).toBeTruthy()
    })

    // Form should be hidden after successful save
    expect(screen.queryByText('New Environment')).toBeNull()
  })

  it('hides the form when Cancel is clicked', async () => {
    renderPage()

    await waitFor(() => screen.getByRole('button', { name: /add environment/i }))

    act(() => {
      screen.getByRole('button', { name: /add environment/i }).click()
    })

    expect(screen.getByText('New Environment')).toBeTruthy()

    act(() => {
      screen.getByText('Cancel').click()
    })

    expect(screen.queryByText('New Environment')).toBeNull()
  })
})

describe('EnvironmentsPage — overlay_path validation', () => {
  beforeEach(() => {
    mockLocationState = makeProduct({ my_role: 'admin' })
    mockListEnvironments.mockResolvedValue([])
  })

  it('shows error and does not call createEnvironment when overlay_path starts with /', async () => {
    renderPage()

    await waitFor(() => screen.getByRole('button', { name: /add environment/i }))

    act(() => {
      screen.getByRole('button', { name: /add environment/i }).click()
    })

    await act(async () => {
      fireEvent.change(screen.getByLabelText('Name *'), { target: { value: 'prod' } })
      fireEvent.change(screen.getByLabelText('Type *'), { target: { value: 'production' } })
      fireEvent.change(screen.getByLabelText('Overlay Path *'), { target: { value: '/overlays/prod' } })
    })

    await act(async () => {
      screen.getByText('Save Environment').click()
    })

    expect(mockCreateEnvironment).not.toHaveBeenCalled()
    expect(screen.getByText('Path must not start with /')).toBeTruthy()
  })

  it('shows error when name is empty', async () => {
    renderPage()

    await waitFor(() => screen.getByRole('button', { name: /add environment/i }))

    act(() => {
      screen.getByRole('button', { name: /add environment/i }).click()
    })

    await act(async () => {
      screen.getByText('Save Environment').click()
    })

    expect(mockCreateEnvironment).not.toHaveBeenCalled()
    expect(screen.getByText('Name is required')).toBeTruthy()
  })

  it('shows error when type is not selected', async () => {
    renderPage()

    await waitFor(() => screen.getByRole('button', { name: /add environment/i }))

    act(() => {
      screen.getByRole('button', { name: /add environment/i }).click()
    })

    await act(async () => {
      fireEvent.change(screen.getByLabelText('Name *'), { target: { value: 'staging' } })
      // Leave type unselected
      fireEvent.change(screen.getByLabelText('Overlay Path *'), { target: { value: 'overlays/staging' } })
    })

    await act(async () => {
      screen.getByText('Save Environment').click()
    })

    expect(mockCreateEnvironment).not.toHaveBeenCalled()
    expect(screen.getByText('Type is required')).toBeTruthy()
  })
})

describe('EnvironmentsPage — delete flow (admin)', () => {
  beforeEach(() => {
    mockLocationState = makeProduct({ my_role: 'admin' })
    mockListEnvironments.mockResolvedValue(mockEnvs)
  })

  it('renders confirm dialog as a native <dialog> element when delete is clicked', async () => {
    renderPage()

    await waitFor(() => screen.getByRole('button', { name: /delete development/i }))

    act(() => {
      screen.getByRole('button', { name: /delete development/i }).click()
    })

    const dialog = screen.getByRole('dialog')
    expect(dialog.tagName.toLowerCase()).toBe('dialog')
  })

  it('shows confirm dialog with environment name when delete button is clicked', async () => {
    renderPage()

    await waitFor(() => screen.getByRole('button', { name: /delete development/i }))

    act(() => {
      screen.getByRole('button', { name: /delete development/i }).click()
    })

    expect(screen.getByText('Remove Environment')).toBeTruthy()
    expect(screen.getByText(/cannot be undone/i)).toBeTruthy()
    // The dialog shows the environment name in the confirm body
    const dialog = screen.getByRole('dialog')
    expect(dialog.textContent).toContain('development')
  })

  it('closes confirm dialog when Cancel is clicked', async () => {
    renderPage()

    await waitFor(() => screen.getByRole('button', { name: /delete development/i }))

    act(() => {
      screen.getByRole('button', { name: /delete development/i }).click()
    })

    expect(screen.getByText('Remove Environment')).toBeTruthy()

    act(() => {
      const cancelButtons = screen.getAllByText('Cancel')
      cancelButtons[cancelButtons.length - 1].click()
    })

    expect(screen.queryByText('Remove Environment')).toBeNull()
  })

  it('closes confirm dialog when Escape key is pressed', async () => {
    renderPage()

    await waitFor(() => screen.getByRole('button', { name: /delete development/i }))

    act(() => {
      screen.getByRole('button', { name: /delete development/i }).click()
    })

    expect(screen.getByRole('dialog')).toBeTruthy()

    act(() => {
      fireEvent.keyDown(window, { key: 'Escape' })
    })

    expect(screen.queryByRole('dialog')).toBeNull()
  })

  it('calls deleteEnvironment with correct env id on confirm', async () => {
    renderPage()

    await waitFor(() => screen.getByRole('button', { name: /delete development/i }))

    act(() => {
      screen.getByRole('button', { name: /delete development/i }).click()
    })

    await act(async () => {
      screen.getByText('Delete Environment').click()
    })

    await waitFor(() => {
      expect(mockDeleteEnvironment).toHaveBeenCalledWith('test-token', 'platform-api', 'e1')
    })
  })

  it('removes the deleted row from the table after confirm', async () => {
    renderPage()

    await waitFor(() => screen.getByRole('button', { name: /delete development/i }))

    act(() => {
      screen.getByRole('button', { name: /delete development/i }).click()
    })

    await act(async () => {
      screen.getByText('Delete Environment').click()
    })

    await waitFor(() => {
      expect(screen.queryByText('development')).toBeNull()
      // production should still be visible
      expect(screen.getAllByText('production')[0]).toBeTruthy()
    })
  })
})

describe('EnvironmentsPage — API error on create', () => {
  it('shows page-level error message when createEnvironment rejects', async () => {
    mockLocationState = makeProduct({ my_role: 'admin' })
    mockListEnvironments.mockResolvedValue([])
    mockCreateEnvironment.mockRejectedValue(new Error('Internal server error'))

    renderPage()

    await waitFor(() => screen.getByRole('button', { name: /add environment/i }))

    act(() => {
      screen.getByRole('button', { name: /add environment/i }).click()
    })

    await act(async () => {
      fireEvent.change(screen.getByLabelText('Name *'), { target: { value: 'staging' } })
      fireEvent.change(screen.getByLabelText('Type *'), { target: { value: 'integration' } })
      fireEvent.change(screen.getByLabelText('Overlay Path *'), { target: { value: 'overlays/staging' } })
    })

    await act(async () => {
      screen.getByText('Save Environment').click()
    })

    await waitFor(() => {
      const errorEl = document.querySelector('.pd-error')
      expect(errorEl).toBeTruthy()
      expect(errorEl?.textContent).toContain('Internal server error')
    })
  })
})

describe('EnvironmentsPage — data loading', () => {
  it('calls listEnvironments with product slug and token', async () => {
    mockLocationState = makeProduct({ my_role: 'admin' })

    renderPage()

    await waitFor(() => {
      expect(mockListEnvironments).toHaveBeenCalledWith('test-token', 'platform-api')
    })
  })

  it('shows empty state when no environments exist', async () => {
    mockLocationState = makeProduct({ my_role: 'admin' })
    mockListEnvironments.mockResolvedValue([])

    renderPage()

    await waitFor(() => {
      expect(screen.getByTestId('empty-environments')).toBeTruthy()
    })
  })

  it('renders overlay paths in the table', async () => {
    mockLocationState = makeProduct({ my_role: 'admin' })
    mockListEnvironments.mockResolvedValue(mockEnvs)

    renderPage()

    await waitFor(() => {
      expect(screen.getByText('overlays/dev')).toBeTruthy()
      expect(screen.getByText('overlays/prod')).toBeTruthy()
    })
  })
})
