import { describe, it, expect, vi, afterEach } from 'vitest'
import {
  listProducts,
  listWorkloads,
  listEnvironments,
  createEnvironment,
  deleteEnvironment,
  createProduct,
  getTagConvention,
  setTagConvention,
  clearTagConvention,
  listTags,
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

const mockEnv = {
  id: 'env-uuid-1',
  product_id: 'prod-uuid-1',
  name: 'staging',
  slug: 'staging',
  type: 'integration' as const,
  gitops_path: 'apps/staging/my-product/my-product-helmrelease.yaml',
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
      slug: 'staging',
    })
    expect(result).toEqual(mockEnv)
  })

  it('sends a POST request with correct URL, body and Content-Type', async () => {
    const fetchStub = makeFetchStub(201, mockEnv)
    vi.stubGlobal('fetch', fetchStub)

    const data = { name: 'staging', type: 'integration' as const, slug: 'staging' }
    await createEnvironment('tok', 'my-product', data)

    const [url, init] = fetchStub.mock.calls[0] as [string, RequestInit]
    expect(url).toBe('/api/v1/products/my-product/environments')
    expect(init.method).toBe('POST')
    expect(init.body).toBe(JSON.stringify(data))
    const headers = new Headers(init.headers)
    expect(headers.get('Content-Type')).toBe('application/json')
  })

  it('throws with server error message on non-2xx response', async () => {
    vi.stubGlobal('fetch', makeFetchStub(409, { error: 'environment slug already exists for this product' }))

    await expect(
      createEnvironment('tok', 'my-product', { name: 'staging', type: 'integration', slug: 'staging' }),
    ).rejects.toThrow('environment slug already exists for this product')
  })

  it('falls back to status code when error body is unparseable', async () => {
    vi.stubGlobal('fetch', makeFetchStub(422))

    await expect(
      createEnvironment('tok', 'my-product', { name: 'staging', type: 'integration', slug: 'staging' }),
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

  it('extracts error field from response body on failure', async () => {
    vi.stubGlobal('fetch', makeFetchStub(403, { error: 'access denied' }))

    await expect(clearTagConvention('tok', 'my-product')).rejects.toThrow('access denied')
  })

  it('falls back to status code when error body has no error field', async () => {
    vi.stubGlobal('fetch', makeFetchStub(403))

    await expect(clearTagConvention('tok', 'my-product')).rejects.toThrow('clearTagConvention: 403')
  })
})

// ─── listWorkloads ─────────────────────────────────────────────
describe('listWorkloads', () => {
  it('calls the correct URL with productSlug and environmentId', async () => {
    const fetchStub = makeFetchStub(200, [])
    vi.stubGlobal('fetch', fetchStub)

    await listWorkloads('tok', 'my-product', 'env-id')

    const [url] = fetchStub.mock.calls[0] as [string, RequestInit]
    expect(url).toBe('/api/v1/products/my-product/environments/env-id/workloads')
  })

  it('returns Workload[] on 200', async () => {
    const workloads = [
      { name: 'api', image_repository: 'europe-west1-docker.pkg.dev/acme/platform/api' },
    ]
    vi.stubGlobal('fetch', makeFetchStub(200, workloads))

    const result = await listWorkloads('tok', 'my-product', 'env-id')
    expect(result).toEqual(workloads)
  })

  it('throws with status code in message on non-ok response', async () => {
    vi.stubGlobal('fetch', makeFetchStub(500))

    await expect(listWorkloads('tok', 'my-product', 'env-id')).rejects.toThrow('listWorkloads: 500')
  })

  it('throws with exact "listWorkloads: 404" message on 404', async () => {
    vi.stubGlobal('fetch', makeFetchStub(404))

    await expect(listWorkloads('tok', 'my-product', 'env-id')).rejects.toThrow('listWorkloads: 404')
  })
})

// ─── listTags ──────────────────────────────────────────────────
describe('listTags', () => {
  it('returns parsed tags response on success', async () => {
    const body = {
      tags: [{ name: 'v1.0.0', digest: 'sha256:abc', pushed_at: '2026-06-01T10:00:00Z' }],
      next_page_token: 'tok2',
    }
    vi.stubGlobal('fetch', makeFetchStub(200, body))

    const result = await listTags('token', 'my-product', 'env-id', 'wl-name')
    expect(result).toEqual(body)
  })

  it('builds correct URL with no options', async () => {
    const fetchStub = makeFetchStub(200, { tags: [], next_page_token: '' })
    vi.stubGlobal('fetch', fetchStub)

    await listTags('token', 'my-product', 'env-id', 'wl-name')

    const [url] = fetchStub.mock.calls[0] as [string, RequestInit]
    expect(url).toBe('/api/v1/products/my-product/environments/env-id/workloads/wl-name/tags')
  })

  it('includes filter in URL query string', async () => {
    const fetchStub = makeFetchStub(200, { tags: [], next_page_token: '' })
    vi.stubGlobal('fetch', fetchStub)

    await listTags('token', 'my-product', 'env-id', 'wl-name', { filter: 'v1' })

    const [url] = fetchStub.mock.calls[0] as [string, RequestInit]
    expect(url).toContain('filter=v1')
  })

  it('includes page_token in URL query string', async () => {
    const fetchStub = makeFetchStub(200, { tags: [], next_page_token: '' })
    vi.stubGlobal('fetch', fetchStub)

    await listTags('token', 'my-product', 'env-id', 'wl-name', { pageToken: 'tok2' })

    const [url] = fetchStub.mock.calls[0] as [string, RequestInit]
    expect(url).toContain('page_token=tok2')
  })

  it('throws on non-ok response', async () => {
    vi.stubGlobal('fetch', makeFetchStub(502))

    await expect(listTags('token', 'my-product', 'env-id', 'wl-name')).rejects.toThrow('listTags: 502')
  })
})
