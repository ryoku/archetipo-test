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

export interface Component {
  id: string
  product_id: string
  name: string
  slug: string
  gcr_image_path: string
  created_at: string
}

export interface Environment {
  id: string
  product_id: string
  name: string
  type: 'dev' | 'integration' | 'production'
  overlay_path: string
  created_at: string
}

export async function listProducts(token: string): Promise<Product[]> {
  const res = await apiFetch('/api/v1/products', token)
  if (!res.ok) throw new Error(`listProducts: ${res.status}`)
  return (await res.json()) as Product[]
}

export async function listComponents(token: string, productSlug: string): Promise<Component[]> {
  const res = await apiFetch(`/api/v1/products/${productSlug}/components`, token)
  if (!res.ok) throw new Error(`listComponents: ${res.status}`)
  return (await res.json()) as Component[]
}

export async function createComponent(
  token: string,
  productSlug: string,
  data: { name: string; slug: string; gcr_image_path: string }
): Promise<Component> {
  const res = await apiFetch(`/api/v1/products/${productSlug}/components`, token, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(data),
  })
  if (!res.ok) throw new Error(`createComponent: ${res.status}`)
  return (await res.json()) as Component
}

export async function deleteComponent(
  token: string,
  productSlug: string,
  componentSlug: string
): Promise<void> {
  const res = await apiFetch(
    `/api/v1/products/${productSlug}/components/${componentSlug}`,
    token,
    { method: 'DELETE' }
  )
  if (!res.ok) throw new Error(`deleteComponent: ${res.status}`)
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
    const body = await res.json().catch(() => null)
    const msg = (body as { error?: string } | null)?.error ?? `setTagConvention: ${res.status}`
    throw new Error(msg)
  }
  return (await res.json()) as TagConvention
}
