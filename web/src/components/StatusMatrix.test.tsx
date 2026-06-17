import { describe, it, expect, afterEach } from 'vitest'
import { render, screen, cleanup } from '@testing-library/react'

afterEach(cleanup)
import StatusMatrix from './StatusMatrix'
import type { Environment, ProductStatus } from '../api/products'

const devEnv: Environment = {
  id: 'env-dev',
  product_id: 'prod-1',
  name: 'dev',
  slug: 'dev',
  type: 'dev',
  gitops_path: '',
  created_at: '2025-01-01T00:00:00Z',
}

const prodEnv: Environment = {
  id: 'env-prod',
  product_id: 'prod-1',
  name: 'production',
  slug: 'production',
  type: 'production',
  gitops_path: '',
  created_at: '2025-01-01T00:00:00Z',
}

const statusData: ProductStatus = {
  workloads: {
    api: { dev: 'v1.2.3', production: 'v1.0.0' },
    worker: { dev: 'v1.1.0', production: 'N/A' },
  },
  fetched_at: '2025-06-17T10:00:00Z',
  stale: false,
}

describe('StatusMatrix', () => {
  it('renders loading state', () => {
    render(
      <StatusMatrix
        status={null}
        environments={[]}
        loading={true}
        error={null}
        onRefresh={() => {}}
      />
    )
    expect(screen.getByTestId('status-loading')).toBeDefined()
  })

  it('renders error state with retry button', () => {
    render(
      <StatusMatrix
        status={null}
        environments={[]}
        loading={false}
        error="Failed to fetch"
        onRefresh={() => {}}
      />
    )
    expect(screen.getByTestId('status-error')).toBeDefined()
    expect(screen.getByText('Retry')).toBeDefined()
  })

  it('renders empty state when no workloads', () => {
    render(
      <StatusMatrix
        status={{ workloads: {}, fetched_at: '2025-06-17T10:00:00Z', stale: false }}
        environments={[devEnv]}
        loading={false}
        error={null}
        onRefresh={() => {}}
      />
    )
    expect(screen.getByTestId('status-empty')).toBeDefined()
  })

  it('renders matrix table with workloads and environments', () => {
    render(
      <StatusMatrix
        status={statusData}
        environments={[devEnv, prodEnv]}
        loading={false}
        error={null}
        onRefresh={() => {}}
      />
    )
    expect(screen.getByTestId('status-matrix')).toBeDefined()
    expect(screen.getByText('api')).toBeDefined()
    expect(screen.getByText('worker')).toBeDefined()
    expect(screen.getByText('v1.2.3')).toBeDefined()
    expect(screen.getByText('v1.0.0')).toBeDefined()
  })

  it('shows N/A for missing tags', () => {
    render(
      <StatusMatrix
        status={statusData}
        environments={[devEnv, prodEnv]}
        loading={false}
        error={null}
        onRefresh={() => {}}
      />
    )
    const naElements = screen.getAllByText('N/A')
    expect(naElements.length).toBeGreaterThan(0)
  })

  it('shows stale banner when stale is true', () => {
    render(
      <StatusMatrix
        status={{ ...statusData, stale: true }}
        environments={[devEnv]}
        loading={false}
        error={null}
        onRefresh={() => {}}
      />
    )
    expect(screen.getByTestId('stale-banner')).toBeDefined()
  })

  it('does not show stale banner when stale is false', () => {
    render(
      <StatusMatrix
        status={{ ...statusData, stale: false }}
        environments={[devEnv]}
        loading={false}
        error={null}
        onRefresh={() => {}}
      />
    )
    expect(screen.queryByTestId('stale-banner')).toBeNull()
  })
})
