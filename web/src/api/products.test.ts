import { describe, it, expect, vi, afterEach } from 'vitest'
import {
  listProducts,
  listComponents,
  createComponent,
  deleteComponent,
} from './products'

// Helper to create a fetch stub that returns a given status + body
function makeFetchStub(status: number, body: unknown = null): ReturnType<typeof vi.fn> {
  return vi.fn().mockResolvedValue(
    new Response(body !== null ? JSON.stringify(body) : null, { status }),
  )
}

afterEach(() => {
  vi.restoreAllMocks()
})

// ─── listProducts ──────────────────────────────────────────────
describe('listProducts', () => {
  it('returns parsed JSON on success', async () => {
    const products = [
      { id: '1', name: 'Platform', slug: 'platform', description: 'desc', created_at: '2025-01-01T00:00:00Z' },
    ]
    vi.stubGlobal('fetch', makeFetchStub(200, products))

    const result = await listProducts('my-token')
    expect(result).toEqual(products)
  })

  it('sets Authorization header when token is provided', async () => {
    const fetchStub = makeFetchStub(200, [])
    vi.stubGlobal('fetch', fetchStub)

    await listProducts('test-token')

    const [, init] = fetchStub.mock.calls[0] as [string, RequestInit]
    const headers = new Headers(init.headers)
    expect(headers.get('Authorization')).toBe('Bearer test-token')
  })

  it('throws an error on non-ok response (404)', async () => {
    vi.stubGlobal('fetch', makeFetchStub(404))

    await expect(listProducts('my-token')).rejects.toThrow('listProducts: 404')
  })

  it('calls /api/v1/products', async () => {
    const fetchStub = makeFetchStub(200, [])
    vi.stubGlobal('fetch', fetchStub)

    await listProducts('tok')

    const [url] = fetchStub.mock.calls[0] as [string, RequestInit]
    expect(url).toBe('/api/v1/products')
  })
})

// ─── listComponents ────────────────────────────────────────────
describe('listComponents', () => {
  it('returns parsed JSON on success', async () => {
    const components = [
      {
        id: 'c1',
        product_id: 'p1',
        name: 'api',
        slug: 'api',
        gcr_image_path: 'europe-west1-docker.pkg.dev/acme/platform/api',
        created_at: '2025-01-01T00:00:00Z',
      },
    ]
    vi.stubGlobal('fetch', makeFetchStub(200, components))

    const result = await listComponents('tok', 'platform')
    expect(result).toEqual(components)
  })

  it('calls the correct URL with the product slug', async () => {
    const fetchStub = makeFetchStub(200, [])
    vi.stubGlobal('fetch', fetchStub)

    await listComponents('tok', 'my-product')

    const [url] = fetchStub.mock.calls[0] as [string, RequestInit]
    expect(url).toBe('/api/v1/products/my-product/components')
  })

  it('throws on non-ok response', async () => {
    vi.stubGlobal('fetch', makeFetchStub(500))

    await expect(listComponents('tok', 'platform')).rejects.toThrow('listComponents: 500')
  })
})

// ─── createComponent ───────────────────────────────────────────
describe('createComponent', () => {
  it('returns the created component on success', async () => {
    const created = {
      id: 'c2',
      product_id: 'p1',
      name: 'worker',
      slug: 'worker',
      gcr_image_path: 'europe-west1-docker.pkg.dev/acme/platform/worker',
      created_at: '2025-01-01T00:00:00Z',
    }
    vi.stubGlobal('fetch', makeFetchStub(201, created))

    const result = await createComponent('tok', 'platform', {
      name: 'worker',
      slug: 'worker',
      gcr_image_path: 'europe-west1-docker.pkg.dev/acme/platform/worker',
    })
    expect(result).toEqual(created)
  })

  it('sends a POST request with correct body and Content-Type', async () => {
    const fetchStub = makeFetchStub(201, { id: 'c2', product_id: 'p1', name: 'worker', slug: 'worker', gcr_image_path: 'path', created_at: '2025-01-01T00:00:00Z' })
    vi.stubGlobal('fetch', fetchStub)

    const data = { name: 'worker', slug: 'worker', gcr_image_path: 'path' }
    await createComponent('tok', 'platform', data)

    const [url, init] = fetchStub.mock.calls[0] as [string, RequestInit]
    expect(url).toBe('/api/v1/products/platform/components')
    expect(init.method).toBe('POST')
    expect(init.body).toBe(JSON.stringify(data))
    const headers = new Headers(init.headers)
    expect(headers.get('Content-Type')).toBe('application/json')
  })

  it('sets Authorization header', async () => {
    const fetchStub = makeFetchStub(201, { id: 'c2', product_id: 'p1', name: 'w', slug: 'w', gcr_image_path: 'p', created_at: '2025-01-01T00:00:00Z' })
    vi.stubGlobal('fetch', fetchStub)

    await createComponent('my-token', 'platform', { name: 'w', slug: 'w', gcr_image_path: 'p' })

    const [, init] = fetchStub.mock.calls[0] as [string, RequestInit]
    const headers = new Headers(init.headers)
    expect(headers.get('Authorization')).toBe('Bearer my-token')
  })

  it('throws on non-ok response', async () => {
    vi.stubGlobal('fetch', makeFetchStub(422))

    await expect(
      createComponent('tok', 'platform', { name: 'w', slug: 'w', gcr_image_path: 'p' }),
    ).rejects.toThrow('createComponent: 422')
  })
})

// ─── deleteComponent ───────────────────────────────────────────
describe('deleteComponent', () => {
  it('resolves without error on 204', async () => {
    vi.stubGlobal('fetch', makeFetchStub(204))

    await expect(deleteComponent('tok', 'platform', 'api')).resolves.toBeUndefined()
  })

  it('sends a DELETE request to the correct URL', async () => {
    const fetchStub = makeFetchStub(204)
    vi.stubGlobal('fetch', fetchStub)

    await deleteComponent('tok', 'my-product', 'my-comp')

    const [url, init] = fetchStub.mock.calls[0] as [string, RequestInit]
    expect(url).toBe('/api/v1/products/my-product/components/my-comp')
    expect(init.method).toBe('DELETE')
  })

  it('throws on non-ok response', async () => {
    vi.stubGlobal('fetch', makeFetchStub(404))

    await expect(deleteComponent('tok', 'platform', 'api')).rejects.toThrow('deleteComponent: 404')
  })
})
