import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, fireEvent, cleanup } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import ProductSubNav from './ProductSubNav'
import type { Product } from '../api/products'

const mockNavigate = vi.hoisted(() => vi.fn())
vi.mock('react-router-dom', async (importOriginal) => {
  const actual = await importOriginal<typeof import('react-router-dom')>()
  return { ...actual, useNavigate: () => mockNavigate }
})

const product: Product = {
  id: 'p1',
  name: 'Platform API',
  slug: 'platform-api',
  description: '',
  created_at: '2025-01-01T00:00:00Z',
  last_deployed_at: null,
  has_production_env: false,
}

function renderNav(activeTab: 'workloads' | 'environments' | 'status' | 'history' | 'settings') {
  return render(
    <MemoryRouter>
      <ProductSubNav activeTab={activeTab} product={product} />
    </MemoryRouter>,
  )
}

beforeEach(() => {
  mockNavigate.mockReset()
  cleanup()
})

describe('ProductSubNav', () => {
  it('renders all five tabs', () => {
    renderNav('workloads')
    expect(screen.getByText('Workloads')).toBeTruthy()
    expect(screen.getByText('Environments')).toBeTruthy()
    expect(screen.getByText('Status')).toBeTruthy()
    expect(screen.getByText('History')).toBeTruthy()
    expect(screen.getByText('Settings')).toBeTruthy()
  })

  it('marks the active tab with the active class and others without it', () => {
    renderNav('status')
    expect(screen.getByText('Status').closest('button')).toHaveClass('pd-subnav-link--active')
    expect(screen.getByText('Workloads').closest('button')).not.toHaveClass('pd-subnav-link--active')
  })

  it('navigates to workloads path when Workloads tab is clicked', () => {
    renderNav('environments')
    fireEvent.click(screen.getByText('Workloads'))
    expect(mockNavigate).toHaveBeenCalledWith('/products/platform-api', { state: product })
  })

  it('navigates to environments path when Environments tab is clicked', () => {
    renderNav('workloads')
    fireEvent.click(screen.getByText('Environments'))
    expect(mockNavigate).toHaveBeenCalledWith('/products/platform-api/environments', { state: product })
  })

  it('navigates to status path when Status tab is clicked', () => {
    renderNav('workloads')
    fireEvent.click(screen.getByText('Status'))
    expect(mockNavigate).toHaveBeenCalledWith('/products/platform-api/status', { state: product })
  })

  it('navigates to history path when History tab is clicked', () => {
    renderNav('workloads')
    fireEvent.click(screen.getByText('History'))
    expect(mockNavigate).toHaveBeenCalledWith('/products/platform-api/history', { state: product })
  })

  it('navigates to settings path when Settings tab is clicked', () => {
    renderNav('workloads')
    fireEvent.click(screen.getByText('Settings'))
    expect(mockNavigate).toHaveBeenCalledWith('/products/platform-api/settings', { state: product })
  })

  it('does not call navigate when clicking the already-active tab', () => {
    renderNav('workloads')
    fireEvent.click(screen.getByText('Workloads'))
    expect(mockNavigate).not.toHaveBeenCalled()
  })
})
