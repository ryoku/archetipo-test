import { describe, it, expect, vi, afterEach } from 'vitest'
import { render, screen, fireEvent, act, cleanup } from '@testing-library/react'
import { DeployDialog } from './DeployDialog'

vi.mock('../api/products', () => ({
  listTags: vi.fn(),
}))

import * as productsApi from '../api/products'

const mockListTags = productsApi.listTags as ReturnType<typeof vi.fn>

const defaultComponent = {
  slug: 'my-comp',
  name: 'My Component',
  gcr_image_path: 'europe-west1-docker.pkg.dev/acme/platform/my-comp',
}

const defaultProps = {
  open: true,
  token: 'test-token',
  productSlug: 'my-product',
  component: defaultComponent,
  onClose: vi.fn(),
  onDeploy: vi.fn(),
}

afterEach(() => {
  cleanup()
  vi.clearAllMocks()
  vi.useRealTimers()
})

describe('DeployDialog', () => {
  it('does not render when open is false', () => {
    // No API calls expected since the component returns null early
    const { container } = render(<DeployDialog {...defaultProps} open={false} />)
    expect(container.firstChild).toBeNull()
  })

  it('shows loading state while fetching', async () => {
    vi.useFakeTimers()
    // Return a never-resolving promise to keep loading state
    mockListTags.mockReturnValue(new Promise(() => { /* never resolves */ }))

    render(<DeployDialog {...defaultProps} />)

    // The first fetchTags call is triggered synchronously via useEffect (open, fetchTags)
    // setLoading(true) is called before the await, so loading is true immediately
    await act(async () => {
      vi.advanceTimersByTime(0)
    })

    expect(screen.getByText(/Caricamento tag/i)).toBeInTheDocument()

    vi.useRealTimers()
  })

  it('renders tag list with name and formatted date', async () => {
    vi.useFakeTimers()
    mockListTags.mockResolvedValue({
      tags: [{ name: 'v1.0.0', digest: 'd', pushed_at: '2026-06-01T10:00:00Z' }],
      next_page_token: '',
    })

    render(<DeployDialog {...defaultProps} />)

    // Let the initial fetchTags fire and resolve
    await act(async () => {
      await Promise.resolve()
    })

    expect(screen.getByText('v1.0.0')).toBeInTheDocument()

    vi.useRealTimers()
  })

  it('shows empty state when no tags match filter', async () => {
    vi.useFakeTimers()

    mockListTags.mockResolvedValue({ tags: [], next_page_token: '' })

    render(<DeployDialog {...defaultProps} />)

    // Settle initial load
    await act(async () => {
      await Promise.resolve()
    })

    const filterInput = screen.getByPlaceholderText(/Filtra per prefisso/i)
    fireEvent.change(filterInput, { target: { value: 'xyz' } })

    // Advance past the 300ms debounce
    await act(async () => {
      vi.advanceTimersByTime(300)
      await Promise.resolve()
    })

    expect(screen.getByText(/Nessun tag corrisponde al filtro/i)).toBeInTheDocument()

    vi.useRealTimers()
  })

  it('shows GCR error banner on fetch failure', async () => {
    vi.useFakeTimers()
    mockListTags.mockRejectedValue(new Error('network error'))

    render(<DeployDialog {...defaultProps} />)

    // Settle initial fetch (rejection)
    await act(async () => {
      await Promise.resolve()
      await Promise.resolve()
    })

    expect(screen.getByText(/Impossibile caricare i tag/i)).toBeInTheDocument()

    vi.useRealTimers()
  })

  it('enables manual input when GCR error', async () => {
    vi.useFakeTimers()
    mockListTags.mockRejectedValue(new Error('network error'))

    render(<DeployDialog {...defaultProps} />)

    await act(async () => {
      await Promise.resolve()
      await Promise.resolve()
    })

    // Manual input appears only in error state
    expect(screen.getByPlaceholderText(/es\. v1\.14\.1-rc\.1/i)).toBeInTheDocument()

    vi.useRealTimers()
  })

  it('enables Deploy button after tag selection', async () => {
    vi.useFakeTimers()
    mockListTags.mockResolvedValue({
      tags: [{ name: 'v1.0.0', digest: 'd', pushed_at: '2026-06-01T10:00:00Z' }],
      next_page_token: '',
    })

    render(<DeployDialog {...defaultProps} />)

    await act(async () => {
      await Promise.resolve()
    })

    expect(screen.getByText('v1.0.0')).toBeInTheDocument()

    const deployBtn = screen.getByRole('button', { name: /Deploy/i })
    expect(deployBtn).toBeDisabled()

    fireEvent.click(screen.getByText('v1.0.0'))

    expect(deployBtn).not.toBeDisabled()

    vi.useRealTimers()
  })

  it('enables Deploy button after manual input when in error state', async () => {
    vi.useFakeTimers()
    mockListTags.mockRejectedValue(new Error('network error'))

    render(<DeployDialog {...defaultProps} />)

    await act(async () => {
      await Promise.resolve()
      await Promise.resolve()
    })

    const deployBtn = screen.getByRole('button', { name: /Deploy/i })
    expect(deployBtn).toBeDisabled()

    const manualInput = screen.getByPlaceholderText(/es\. v1\.14\.1-rc\.1/i)
    fireEvent.change(manualInput, { target: { value: 'v2.0.0' } })

    expect(deployBtn).not.toBeDisabled()

    vi.useRealTimers()
  })

  it('calls onClose when backdrop clicked', async () => {
    vi.useFakeTimers()
    mockListTags.mockResolvedValue({ tags: [], next_page_token: '' })

    const onClose = vi.fn()
    render(<DeployDialog {...defaultProps} onClose={onClose} />)

    await act(async () => {
      await Promise.resolve()
    })

    // The backdrop is the dd-backdrop div which is the parent of dd-modal (role="dialog")
    const dialog = screen.getByRole('dialog')
    const backdrop = dialog.parentElement
    if (!backdrop) throw new Error('backdrop not found')
    fireEvent.click(backdrop)

    expect(onClose).toHaveBeenCalled()

    vi.useRealTimers()
  })

  it('calls onClose on Escape key', async () => {
    vi.useFakeTimers()
    mockListTags.mockResolvedValue({ tags: [], next_page_token: '' })

    const onClose = vi.fn()
    render(<DeployDialog {...defaultProps} onClose={onClose} />)

    await act(async () => {
      await Promise.resolve()
    })

    fireEvent.keyDown(document, { key: 'Escape' })

    expect(onClose).toHaveBeenCalled()

    vi.useRealTimers()
  })
})
