import { describe, it, expect, vi, beforeEach, afterEach, type Mock } from 'vitest'
import { render, screen, waitFor, act, cleanup, fireEvent } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import type { Product, TagConvention } from '../api/products'
import ProductSettingsPage from './ProductSettingsPage'

// ─── Mocks ────────────────────────────────────────────────────

const mockGetTagConvention = vi.hoisted(() => vi.fn<() => Promise<TagConvention>>())
const mockSetTagConvention = vi.hoisted(() => vi.fn<() => Promise<TagConvention>>())
const mockClearTagConvention = vi.hoisted(() => vi.fn<() => Promise<void>>())

vi.mock('../api/products', () => ({
  getTagConvention: mockGetTagConvention,
  setTagConvention: mockSetTagConvention,
  clearTagConvention: mockClearTagConvention,
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

function makeTagConvention(overrides: Partial<TagConvention> = {}): TagConvention {
  return {
    regex: '^v\\d+\\.\\d+\\.\\d+$',
    source: 'product',
    ...overrides,
  }
}

function renderPage() {
  return render(
    <MemoryRouter initialEntries={[`/products/${mockSlug}/settings`]}>
      <ProductSettingsPage />
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
  mockGetTagConvention.mockResolvedValue(makeTagConvention())
  mockSetTagConvention.mockResolvedValue(makeTagConvention())
  mockClearTagConvention.mockResolvedValue(undefined)
})

afterEach(cleanup)

// ─── Tests ────────────────────────────────────────────────────

describe('ProductSettingsPage — not found', () => {
  it('renders ProductNotFound fallback when location.state is missing', () => {
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

describe('ProductSettingsPage — loading state', () => {
  it('shows loading indicator while getTagConvention is in flight', async () => {
    // Never resolve so the loading state persists for the duration of the assertion
    mockGetTagConvention.mockReturnValue(new Promise(() => {}))

    renderPage()

    expect(screen.getByText(/Loading tag convention/i)).toBeTruthy()
  })
})

describe('ProductSettingsPage — viewer role (read-only)', () => {
  beforeEach(() => {
    mockLocationState = makeProduct({ my_role: 'viewer' })
  })

  it('renders tag convention regex in read-only mode', async () => {
    renderPage()

    await waitFor(() => {
      expect(screen.getByText('^v\\d+\\.\\d+\\.\\d+$')).toBeTruthy()
    })
  })

  it('does not show Edit button for viewer role', async () => {
    renderPage()

    await waitFor(() => screen.getByText('^v\\d+\\.\\d+\\.\\d+$'))

    expect(screen.queryByRole('button', { name: /edit/i })).toBeNull()
  })
})

describe('ProductSettingsPage — admin/editor role (write access)', () => {
  it('renders tag convention regex with an Edit button for admin', async () => {
    mockLocationState = makeProduct({ my_role: 'admin' })

    renderPage()

    await waitFor(() => {
      expect(screen.getByText('^v\\d+\\.\\d+\\.\\d+$')).toBeTruthy()
    })

    expect(screen.getByRole('button', { name: /edit/i })).toBeTruthy()
  })

  it('renders tag convention regex with an Edit button for editor', async () => {
    mockLocationState = makeProduct({ my_role: 'editor' })

    renderPage()

    await waitFor(() => {
      expect(screen.getByText('^v\\d+\\.\\d+\\.\\d+$')).toBeTruthy()
    })

    expect(screen.getByRole('button', { name: /edit/i })).toBeTruthy()
  })
})

describe('ProductSettingsPage — edit mode', () => {
  beforeEach(() => {
    mockLocationState = makeProduct({ my_role: 'admin' })
  })

  it('clicking Edit shows the text input and Save/Cancel buttons', async () => {
    renderPage()

    await waitFor(() => screen.getByRole('button', { name: /edit/i }))

    act(() => {
      screen.getByRole('button', { name: /edit/i }).click()
    })

    expect(screen.getByLabelText('Regex pattern')).toBeTruthy()
    expect(screen.getByRole('button', { name: /save/i })).toBeTruthy()
    expect(screen.getByRole('button', { name: /cancel/i })).toBeTruthy()
  })

  it('pre-fills the input with the current regex value when Edit is clicked', async () => {
    mockGetTagConvention.mockResolvedValue(makeTagConvention({ regex: '^release-\\d+$' }))

    renderPage()

    await waitFor(() => screen.getByRole('button', { name: /edit/i }))

    act(() => {
      screen.getByRole('button', { name: /edit/i }).click()
    })

    expect(screen.getByLabelText<HTMLInputElement>('Regex pattern').value).toBe(String.raw`^release-\d+$`)
  })

  it('Cancel button hides edit form and restores read-only view', async () => {
    renderPage()

    await waitFor(() => screen.getByRole('button', { name: /edit/i }))

    act(() => {
      screen.getByRole('button', { name: /edit/i }).click()
    })

    expect(screen.getByLabelText('Regex pattern')).toBeTruthy()

    act(() => {
      screen.getByRole('button', { name: /cancel/i }).click()
    })

    expect(screen.queryByLabelText('Regex pattern')).toBeNull()
    expect(screen.getByText('^v\\d+\\.\\d+\\.\\d+$')).toBeTruthy()
  })
})

describe('ProductSettingsPage — save success', () => {
  beforeEach(() => {
    mockLocationState = makeProduct({ my_role: 'admin' })
  })

  it('submitting valid regex calls setTagConvention with correct args', async () => {
    const updated = makeTagConvention({ regex: '^v\\d+$', source: 'product' })
    mockSetTagConvention.mockResolvedValue(updated)

    renderPage()

    await waitFor(() => screen.getByRole('button', { name: /edit/i }))

    act(() => {
      screen.getByRole('button', { name: /edit/i }).click()
    })

    await act(async () => {
      fireEvent.change(screen.getByLabelText('Regex pattern'), { target: { value: '^v\\d+$' } })
    })

    await act(async () => {
      screen.getByRole('button', { name: /save/i }).click()
    })

    await waitFor(() => {
      expect(mockSetTagConvention).toHaveBeenCalledWith('test-token', 'platform-api', '^v\\d+$')
    })
  })

  it('updates displayed value after successful save', async () => {
    const updated = makeTagConvention({ regex: '^v\\d+$', source: 'product' })
    mockSetTagConvention.mockResolvedValue(updated)

    renderPage()

    await waitFor(() => screen.getByRole('button', { name: /edit/i }))

    act(() => {
      screen.getByRole('button', { name: /edit/i }).click()
    })

    await act(async () => {
      fireEvent.change(screen.getByLabelText('Regex pattern'), { target: { value: '^v\\d+$' } })
    })

    await act(async () => {
      screen.getByRole('button', { name: /save/i }).click()
    })

    await waitFor(() => {
      expect(screen.getByText('^v\\d+$')).toBeTruthy()
    })

    // Edit form should be dismissed after save
    expect(screen.queryByLabelText('Regex pattern')).toBeNull()
  })
})

describe('ProductSettingsPage — save error: empty regex', () => {
  beforeEach(() => {
    mockLocationState = makeProduct({ my_role: 'admin' })
  })

  it('shows "Regex is required" inline error when submitting empty value', async () => {
    renderPage()

    await waitFor(() => screen.getByRole('button', { name: /edit/i }))

    act(() => {
      screen.getByRole('button', { name: /edit/i }).click()
    })

    // Clear the input to empty
    await act(async () => {
      fireEvent.change(screen.getByLabelText('Regex pattern'), { target: { value: '' } })
    })

    await act(async () => {
      screen.getByRole('button', { name: /save/i }).click()
    })

    expect(screen.getByText('Regex is required')).toBeTruthy()
  })

  it('does not call setTagConvention when regex is empty', async () => {
    renderPage()

    await waitFor(() => screen.getByRole('button', { name: /edit/i }))

    act(() => {
      screen.getByRole('button', { name: /edit/i }).click()
    })

    await act(async () => {
      fireEvent.change(screen.getByLabelText('Regex pattern'), { target: { value: '   ' } })
    })

    await act(async () => {
      screen.getByRole('button', { name: /save/i }).click()
    })

    expect(mockSetTagConvention).not.toHaveBeenCalled()
    expect(screen.getByText('Regex is required')).toBeTruthy()
  })
})

describe('ProductSettingsPage — save error: API rejection', () => {
  beforeEach(() => {
    mockLocationState = makeProduct({ my_role: 'admin' })
  })

  it('shows editError when setTagConvention rejects with API error', async () => {
    mockSetTagConvention.mockRejectedValue(new Error('setTagConvention: 400'))

    renderPage()

    await waitFor(() => screen.getByRole('button', { name: /edit/i }))

    act(() => {
      screen.getByRole('button', { name: /edit/i }).click()
    })

    await act(async () => {
      fireEvent.change(screen.getByLabelText('Regex pattern'), { target: { value: String.raw`^v\d+$` } })
    })

    await act(async () => {
      screen.getByRole('button', { name: /save/i }).click()
    })

    await waitFor(() => {
      const errorEl = document.querySelector('.pd-field-error')
      expect(errorEl).toBeTruthy()
      expect(errorEl?.textContent).toContain('setTagConvention: 400')
    })
  })
})

describe('ProductSettingsPage — reset to default', () => {
  beforeEach(() => {
    mockLocationState = makeProduct({ my_role: 'admin' })
    mockGetTagConvention.mockResolvedValue(makeTagConvention({ source: 'product' }))
  })

  it('calls clearTagConvention then re-fetches when Reset to default is clicked', async () => {
    mockGetTagConvention
      .mockResolvedValueOnce(makeTagConvention({ source: 'product' }))
      .mockResolvedValueOnce(makeTagConvention({ source: 'default' }))

    renderPage()

    await waitFor(() => screen.getByRole('button', { name: /reset to default/i }))

    await act(async () => {
      screen.getByRole('button', { name: /reset to default/i }).click()
    })

    await waitFor(() => {
      expect(mockClearTagConvention).toHaveBeenCalledWith('test-token', 'platform-api')
      expect(mockGetTagConvention).toHaveBeenCalledTimes(2)
    })
  })

  it('updates badge to "global default" after successful reset', async () => {
    mockGetTagConvention
      .mockResolvedValueOnce(makeTagConvention({ source: 'product' }))
      .mockResolvedValueOnce(makeTagConvention({ source: 'default' }))

    renderPage()

    await waitFor(() => screen.getByRole('button', { name: /reset to default/i }))

    await act(async () => {
      screen.getByRole('button', { name: /reset to default/i }).click()
    })

    await waitFor(() => {
      expect(screen.getByText(/global default/i)).toBeTruthy()
    })
  })

  it('shows error banner when clearTagConvention rejects', async () => {
    mockClearTagConvention.mockRejectedValue(new Error('clearTagConvention: 403'))

    renderPage()

    await waitFor(() => screen.getByRole('button', { name: /reset to default/i }))

    await act(async () => {
      screen.getByRole('button', { name: /reset to default/i }).click()
    })

    await waitFor(() => {
      const errorEl = document.querySelector('.pd-error')
      expect(errorEl).toBeTruthy()
      expect(errorEl?.textContent).toContain('clearTagConvention: 403')
    })
  })

  it('Reset to default button is absent when source is "default"', async () => {
    mockGetTagConvention.mockResolvedValue(makeTagConvention({ source: 'default' }))

    renderPage()

    await waitFor(() => screen.getByText(/global default/i))

    expect(screen.queryByRole('button', { name: /reset to default/i })).toBeNull()
  })

  it('Reset to default button is absent for viewer role even when source is "product"', async () => {
    mockLocationState = makeProduct({ my_role: 'viewer' })

    renderPage()

    await waitFor(() => screen.getByText(/product override/i))

    expect(screen.queryByRole('button', { name: /reset to default/i })).toBeNull()
  })
})

describe('ProductSettingsPage — data loading', () => {
  it('calls getTagConvention with product slug and token', async () => {
    mockLocationState = makeProduct({ my_role: 'admin' })

    renderPage()

    await waitFor(() => {
      expect(mockGetTagConvention).toHaveBeenCalledWith('test-token', 'platform-api')
    })
  })

  it('shows error message when getTagConvention rejects', async () => {
    mockLocationState = makeProduct({ my_role: 'admin' })
    mockGetTagConvention.mockRejectedValue(new Error('Network error'))

    renderPage()

    await waitFor(() => {
      const errorEl = document.querySelector('.pd-error')
      expect(errorEl).toBeTruthy()
      expect(errorEl?.textContent).toContain('Network error')
    })
  })

  it('shows "global default" badge when source is default', async () => {
    mockLocationState = makeProduct({ my_role: 'admin' })
    mockGetTagConvention.mockResolvedValue(makeTagConvention({ source: 'default' }))

    renderPage()

    await waitFor(() => {
      expect(screen.getByText(/global default/i)).toBeTruthy()
    })
  })

  it('shows "product override" badge when source is product', async () => {
    mockLocationState = makeProduct({ my_role: 'admin' })
    mockGetTagConvention.mockResolvedValue(makeTagConvention({ source: 'product' }))

    renderPage()

    await waitFor(() => {
      expect(screen.getByText(/product override/i)).toBeTruthy()
    })
  })
})
