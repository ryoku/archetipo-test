import { apiFetch } from './client'

export interface Product {
  id: string
  name: string
  slug: string
  description: string
  archived_at?: string
  created_at: string
  my_role?: 'admin' | 'editor' | 'viewer'
}

export interface Environment {
  id: string
  product_id: string
  name: string
  type: 'dev' | 'integration' | 'production'
  overlay_path: string
  created_at: string
}

export interface Workload {
  name: string
  image_repository: string
}

export async function listProducts(token: string): Promise<Product[]> {
  const res = await apiFetch('/api/v1/products', token)
  if (!res.ok) throw new Error(`listProducts: ${res.status}`)
  return (await res.json()) as Product[]
}

export async function listEnvironments(token: string, productSlug: string): Promise<Environment[]> {
  const res = await apiFetch(`/api/v1/products/${productSlug}/environments`, token)
  if (!res.ok) throw new Error(`listEnvironments: ${res.status}`)
  return (await res.json()) as Environment[]
}

export async function createEnvironment(
  token: string,
  productSlug: string,
  data: { name: string; type: Environment['type']; overlay_path: string }
): Promise<Environment> {
  const res = await apiFetch(`/api/v1/products/${productSlug}/environments`, token, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(data),
  })
  if (!res.ok) throw new Error(`createEnvironment: ${res.status}`)
  return (await res.json()) as Environment
}

export async function deleteEnvironment(
  token: string,
  productSlug: string,
  environmentId: string
): Promise<void> {
  const res = await apiFetch(
    `/api/v1/products/${productSlug}/environments/${environmentId}`,
    token,
    { method: 'DELETE' }
  )
  if (!res.ok) throw new Error(`deleteEnvironment: ${res.status}`)
}

export async function listWorkloads(token: string, productSlug: string, environmentId: string): Promise<Workload[]> {
  const res = await apiFetch(`/api/v1/products/${productSlug}/environments/${environmentId}/workloads`, token)
  if (!res.ok) throw new Error(`listWorkloads: ${res.status}`)
  return (await res.json()) as Workload[]
}

export async function createProduct(
  token: string,
  data: { name: string; slug: string; description: string }
): Promise<Product> {
  const res = await apiFetch('/api/v1/products', token, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(data),
  })
  if (!res.ok) throw new Error(`createProduct: ${res.status}`)
  return (await res.json()) as Product
}

export interface TagConvention {
  regex: string
  source: 'product' | 'default'
}

export async function getTagConvention(
  token: string,
  productSlug: string
): Promise<TagConvention> {
  const res = await apiFetch(`/api/v1/products/${productSlug}/tag-convention`, token)
  if (!res.ok) throw new Error(`getTagConvention: ${res.status}`)
  return (await res.json()) as TagConvention
}

export async function setTagConvention(
  token: string,
  productSlug: string,
  regex: string
): Promise<TagConvention> {
  const res = await apiFetch(`/api/v1/products/${productSlug}/tag-convention`, token, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ regex }),
  })
  if (!res.ok) {
    const body = (await res.json().catch(() => null)) as { error?: string } | null
    throw new Error(body?.error ?? `setTagConvention: ${res.status}`)
  }
  return (await res.json()) as TagConvention
}

export async function clearTagConvention(token: string, productSlug: string): Promise<void> {
  const res = await apiFetch(`/api/v1/products/${productSlug}/tag-convention`, token, {
    method: 'DELETE',
  })
  if (!res.ok) {
    const body = (await res.json().catch(() => null)) as { error?: string } | null
    throw new Error(body?.error ?? `clearTagConvention: ${res.status}`)
  }
}

export interface Tag {
  name: string
  digest: string
  pushed_at: string
}

export interface ListTagsResponse {
  tags: Tag[]
  next_page_token?: string
}

export async function listTags(
  token: string,
  productSlug: string,
  environmentId: string,
  workloadName: string,
  options?: { pageToken?: string; pageSize?: number; filter?: string }
): Promise<ListTagsResponse> {
  const params = new URLSearchParams()
  if (options?.pageToken !== undefined) params.set('page_token', options.pageToken)
  if (options?.pageSize !== undefined) params.set('page_size', String(options.pageSize))
  if (options?.filter !== undefined && options.filter !== '') params.set('filter', options.filter)
  const qs = params.toString()
  const url = `/api/v1/products/${productSlug}/environments/${environmentId}/workloads/${workloadName}/tags${qs ? '?' + qs : ''}`
  const res = await apiFetch(url, token)
  if (!res.ok) throw new Error(`listTags: ${res.status}`)
  return (await res.json()) as ListTagsResponse
}
