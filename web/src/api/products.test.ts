import { describe, it, expect, vi, afterEach } from 'vitest'
import {
  listProducts,
  listComponents,
  createComponent,
  deleteComponent,
  listEnvironments,
  createEnvironment,
  deleteEnvironment,
  createProduct,
  getTagConvention,
  setTagConvention,
  clearTagConvention,
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

const mockEnv = {
  id: 'env-uuid-1',
  product_id: 'prod-uuid-1',
  name: 'staging',
  type: 'integration' as const,
  overlay_path: 'overlays/staging',
  created_at: '2025-11-14T10:00:00Z',
}

// ─── listEnvironments ──────────────────────────────────────────
describe('listEnvironments', () => {
  it('returns Environment[] on 200', async () => {
    vi.stubGlobal('fetch', makeFetchStub(200, [mockEnv]))

    const result = await listEnvironments('tok', 'my-product')
    expect(result).toEqual([mockEnv])
  })

  it('calls the correct URL with the product slug', async () => {
    const fetchStub = makeFetchStub(200, [])
    vi.stubGlobal('fetch', fetchStub)

    await listEnvironments('tok', 'my-product')

    const [url] = fetchStub.mock.calls[0] as [string, RequestInit]
    expect(url).toBe('/api/v1/products/my-product/environments')
  })

  it('throws on non-2xx response', async () => {
    vi.stubGlobal('fetch', makeFetchStub(500))

    await expect(listEnvironments('tok', 'my-product')).rejects.toThrow('listEnvironments: 500')
  })
})

// ─── createEnvironment ─────────────────────────────────────────
describe('createEnvironment', () => {
  it('returns new Environment on 201', async () => {
    vi.stubGlobal('fetch', makeFetchStub(201, mockEnv))

    const result = await createEnvironment('tok', 'my-product', {
      name: 'staging',
      type: 'integration',
      overlay_path: 'overlays/staging',
    })
    expect(result).toEqual(mockEnv)
  })

  it('sends a POST request with correct URL, body and Content-Type', async () => {
    const fetchStub = makeFetchStub(201, mockEnv)
    vi.stubGlobal('fetch', fetchStub)

    const data = { name: 'staging', type: 'integration' as const, overlay_path: 'overlays/staging' }
    await createEnvironment('tok', 'my-product', data)

    const [url, init] = fetchStub.mock.calls[0] as [string, RequestInit]
    expect(url).toBe('/api/v1/products/my-product/environments')
    expect(init.method).toBe('POST')
    expect(init.body).toBe(JSON.stringify(data))
    const headers = new Headers(init.headers)
    expect(headers.get('Content-Type')).toBe('application/json')
  })

  it('throws on non-2xx response', async () => {
    vi.stubGlobal('fetch', makeFetchStub(422))

    await expect(
      createEnvironment('tok', 'my-product', { name: 'staging', type: 'integration', overlay_path: 'overlays/staging' }),
    ).rejects.toThrow('createEnvironment: 422')
  })
})

// ─── deleteEnvironment ─────────────────────────────────────────
describe('deleteEnvironment', () => {
  it('resolves void on 204', async () => {
    vi.stubGlobal('fetch', makeFetchStub(204))

    await expect(deleteEnvironment('tok', 'my-product', 'env-uuid-1')).resolves.toBeUndefined()
  })

  it('sends a DELETE request to the correct URL', async () => {
    const fetchStub = makeFetchStub(204)
    vi.stubGlobal('fetch', fetchStub)

    await deleteEnvironment('tok', 'my-product', 'env-uuid-1')

    const [url, init] = fetchStub.mock.calls[0] as [string, RequestInit]
    expect(url).toBe('/api/v1/products/my-product/environments/env-uuid-1')
    expect(init.method).toBe('DELETE')
  })

  it('throws on non-2xx response', async () => {
    vi.stubGlobal('fetch', makeFetchStub(404))

    await expect(deleteEnvironment('tok', 'my-product', 'env-uuid-1')).rejects.toThrow('deleteEnvironment: 404')
  })
})

// ─── createProduct ─────────────────────────────────────────────
describe('createProduct', () => {
  it('returns the created product on 201', async () => {
    const created: import('./products').Product = {
      id: 'p1',
      name: 'Platform',
      slug: 'platform',
      description: 'A platform product',
      created_at: '2025-01-01T00:00:00Z',
    }
    vi.stubGlobal('fetch', makeFetchStub(201, created))

    const result = await createProduct('tok', { name: 'Platform', slug: 'platform', description: 'A platform product' })
    expect(result).toEqual(created)
  })

  it('sends a POST request with correct method, Content-Type header, and body', async () => {
    const created: import('./products').Product = {
      id: 'p1',
      name: 'Platform',
      slug: 'platform',
      description: 'A platform product',
      created_at: '2025-01-01T00:00:00Z',
    }
    const fetchStub = makeFetchStub(201, created)
    vi.stubGlobal('fetch', fetchStub)

    const data = { name: 'Platform', slug: 'platform', description: 'A platform product' }
    await createProduct('tok', data)

    const [url, init] = fetchStub.mock.calls[0] as [string, RequestInit]
    expect(url).toBe('/api/v1/products')
    expect(init.method).toBe('POST')
    expect(init.body).toBe(JSON.stringify(data))
    const headers = new Headers(init.headers)
    expect(headers.get('Content-Type')).toBe('application/json')
  })

  it('throws on non-ok response (409)', async () => {
    vi.stubGlobal('fetch', makeFetchStub(409))

    await expect(
      createProduct('tok', { name: 'Platform', slug: 'platform', description: 'A platform product' }),
    ).rejects.toThrow('createProduct: 409')
  })
})

// ─── getTagConvention ──────────────────────────────────────────
describe('getTagConvention', () => {
  it('returns parsed TagConvention on success', async () => {
    const tc = { regex: String.raw`^v\d+$`, source: 'product' }
    vi.stubGlobal('fetch', makeFetchStub(200, tc))

    const result = await getTagConvention('tok', 'my-product')
    expect(result).toEqual(tc)
  })

  it('calls the correct URL with the product slug', async () => {
    const fetchStub = makeFetchStub(200, { regex: String.raw`^v\d+$`, source: 'default' })
    vi.stubGlobal('fetch', fetchStub)

    await getTagConvention('tok', 'my-product')

    const [url] = fetchStub.mock.calls[0] as [string, RequestInit]
    expect(url).toBe('/api/v1/products/my-product/tag-convention')
  })

  it('sets Authorization header', async () => {
    const fetchStub = makeFetchStub(200, { regex: String.raw`^v\d+$`, source: 'default' })
    vi.stubGlobal('fetch', fetchStub)

    await getTagConvention('test-token', 'my-product')

    const [, init] = fetchStub.mock.calls[0] as [string, RequestInit]
    const headers = new Headers(init.headers)
    expect(headers.get('Authorization')).toBe('Bearer test-token')
  })

  it('throws on non-ok response', async () => {
    vi.stubGlobal('fetch', makeFetchStub(403))

    await expect(getTagConvention('tok', 'my-product')).rejects.toThrow('getTagConvention: 403')
  })
})

// ─── setTagConvention ──────────────────────────────────────────
describe('setTagConvention', () => {
  it('returns updated TagConvention on success', async () => {
    const tc = { regex: String.raw`^v\d+$`, source: 'product' }
    vi.stubGlobal('fetch', makeFetchStub(200, tc))

    const result = await setTagConvention('tok', 'my-product', String.raw`^v\d+$`)
    expect(result).toEqual(tc)
  })

  it('calls the correct URL and uses PUT with JSON body', async () => {
    const fetchStub = makeFetchStub(200, { regex: String.raw`^v\d+$`, source: 'product' })
    vi.stubGlobal('fetch', fetchStub)

    await setTagConvention('tok', 'my-product', String.raw`^v\d+$`)

    const [url, init] = fetchStub.mock.calls[0] as [string, RequestInit]
    expect(url).toBe('/api/v1/products/my-product/tag-convention')
    expect(init.method).toBe('PUT')
    expect(init.body).toBe(JSON.stringify({ regex: String.raw`^v\d+$` }))
    const headers = new Headers(init.headers)
    expect(headers.get('Content-Type')).toBe('application/json')
  })

  it('extracts error field from response body on failure', async () => {
    vi.stubGlobal('fetch', makeFetchStub(422, { error: 'regex must not be empty' }))

    await expect(setTagConvention('tok', 'my-product', '')).rejects.toThrow('regex must not be empty')
  })

  it('falls back to status code when error body has no error field', async () => {
    vi.stubGlobal('fetch', makeFetchStub(500))

    await expect(setTagConvention('tok', 'my-product', String.raw`^v\d+$`)).rejects.toThrow(
      'setTagConvention: 500',
    )
  })
})

// ─── clearTagConvention ────────────────────────────────────────
describe('clearTagConvention', () => {
  it('resolves without error on 204', async () => {
    vi.stubGlobal('fetch', makeFetchStub(204))

    await expect(clearTagConvention('tok', 'my-product')).resolves.toBeUndefined()
  })

  it('calls the correct URL and uses DELETE', async () => {
    const fetchStub = makeFetchStub(204)
    vi.stubGlobal('fetch', fetchStub)

    await clearTagConvention('tok', 'my-product')

    const [url, init] = fetchStub.mock.calls[0] as [string, RequestInit]
    expect(url).toBe('/api/v1/products/my-product/tag-convention')
    expect(init.method).toBe('DELETE')
  })

  it('throws on non-ok response', async () => {
    vi.stubGlobal('fetch', makeFetchStub(403))

    await expect(clearTagConvention('tok', 'my-product')).rejects.toThrow('clearTagConvention: 403')
  })
})
