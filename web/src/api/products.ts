import { apiFetch } from './client'

export interface Product {
  id: string
  name: string
  slug: string
  description: string
  archived_at?: string
  created_at: string
  my_role?: string // caller's effective role: 'editor' | 'viewer' | 'admin'
}

export interface Component {
  id: string
  product_id: string
  name: string
  slug: string
  gcr_image_path: string
  created_at: string
}

export async function listProducts(token: string): Promise<Product[]> {
  const res = await apiFetch('/api/v1/products', token)
  if (!res.ok) throw new Error(`listProducts: ${res.status}`)
  return res.json()
}

export async function listComponents(token: string, productSlug: string): Promise<Component[]> {
  const res = await apiFetch(`/api/v1/products/${productSlug}/components`, token)
  if (!res.ok) throw new Error(`listComponents: ${res.status}`)
  return res.json()
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
  return res.json()
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
