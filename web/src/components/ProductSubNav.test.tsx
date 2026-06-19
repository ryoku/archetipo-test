import { describe, it, expect, vi, afterEach } from 'vitest'
import { render, screen, fireEvent, cleanup } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import ProductSubNav from './ProductSubNav'
import type { Product } from '../api/products'

const mockNavigate = vi.hoisted(() => vi.fn())
vi.mock('react-router-dom', async (importOriginal) => {
  const actual = await importOriginal<typeof import('react-router-dom')>()
  return { ...actual, useNavigate: () => mockNavigate }
})

const fakeProduct: Product = {
  id: 'prod-1',
  name: 'My Product',
  slug: 'my-product',
  description: '',
  created_at: '2026-01-01T00:00:00Z',
}

afterEach(() => {
  cleanup()
  vi.clearAllMocks()
})

describe('ProductSubNav', () => {
  it('renders all four tab buttons with icons', () => {
    render(
      <MemoryRouter>
        <ProductSubNav activeTab="workloads" product={fakeProduct} />
      </MemoryRouter>
    )
    expect(screen.getByRole('button', { name: /Workloads/i })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /Environments/i })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /Status/i })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /Settings/i })).toBeInTheDocument()
  })

  it('applies active class only to the active tab', () => {
    render(
      <MemoryRouter>
        <ProductSubNav activeTab="status" product={fakeProduct} />
      </MemoryRouter>
    )
    expect(screen.getByRole('button', { name: /Status/i })).toHaveClass('pd-subnav-link--active')
    expect(screen.getByRole('button', { name: /Workloads/i })).not.toHaveClass('pd-subnav-link--active')
  })

  it('navigates to the correct path when an inactive tab is clicked', () => {
    render(
      <MemoryRouter>
        <ProductSubNav activeTab="workloads" product={fakeProduct} />
      </MemoryRouter>
    )
    fireEvent.click(screen.getByRole('button', { name: /Status/i }))
    expect(mockNavigate).toHaveBeenCalledWith('/products/my-product/status', { state: fakeProduct })
  })

  it('does not navigate when the already-active tab is clicked', () => {
    render(
      <MemoryRouter>
        <ProductSubNav activeTab="workloads" product={fakeProduct} />
      </MemoryRouter>
    )
    fireEvent.click(screen.getByRole('button', { name: /Workloads/i }))
    expect(mockNavigate).not.toHaveBeenCalled()
  })
})
