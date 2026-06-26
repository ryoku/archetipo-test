import { apiFetch } from './client'

export interface Product {
  id: string
  name: string
  slug: string
  description: string
  archived_at?: string
  created_at: string
  my_role?: 'admin' | 'editor' | 'viewer'
  last_deployed_at: string | null
  has_production_env: boolean
}

export interface Environment {
  id: string
  product_id: string
  name: string
  slug: string
  type: 'dev' | 'integration' | 'production'
  gitops_path: string
  created_at: string
}

export interface Workload {
  readonly name: string
  readonly image_repository: string
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
  data: { name: string; type: Environment['type']; slug: string }
): Promise<Environment> {
  const res = await apiFetch(`/api/v1/products/${productSlug}/environments`, token, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(data),
  })
  if (!res.ok) {
    const body = (await res.json().catch(() => null)) as { error?: string } | null
    throw new Error(body?.error ?? `createEnvironment: ${res.status}`)
  }
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

export interface DeployResult {
  deployment_id: string
}

export interface DeployConflictError {
  type: 'conflict'
  lock_holder: string | undefined
  locked_since: string | undefined
}

export interface DeployTagConventionError {
  type: 'tag_convention'
  rejected_tag: string
  applied_regex: string | undefined
  message: string | undefined
}

export type DeployError = DeployConflictError | DeployTagConventionError

export class DeployApiError extends Error {
  readonly detail: DeployError
  constructor(detail: DeployError) {
    super(`deploy failed: ${detail.type}`)
    this.name = 'DeployApiError'
    this.detail = detail
  }
}

export async function deployTag(
  token: string,
  productSlug: string,
  environmentId: string,
  workload: string,
  tag: string,
): Promise<DeployResult> {
  const res = await apiFetch(
    `/api/v1/products/${productSlug}/environments/${environmentId}/deployments`,
    token,
    {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ workload, tag }),
    },
  )
  if (res.status === 202 || res.status === 200) {
    const body: unknown = await res.json().catch((err: unknown) => {
      throw new Error(`deployTag: response body not valid JSON (status ${res.status}): ${err instanceof Error ? err.message : JSON.stringify(err)}`)
    })
    return body as DeployResult
  }
  if (res.status === 409) {
    const body = (await res.json().catch((err: unknown) => {
      console.warn('[deployTag] failed to parse 409 body:', err)
      return null
    })) as { lock_holder?: string; locked_since?: string } | null
    throw new DeployApiError({
      type: 'conflict',
      lock_holder: body?.lock_holder,
      locked_since: body?.locked_since,
    })
  }
  if (res.status === 422) {
    const body = (await res.json().catch((err: unknown) => {
      console.warn('[deployTag] failed to parse 422 body:', err)
      return null
    })) as { rejected_tag?: string; applied_regex?: string; message?: string } | null
    throw new DeployApiError({
      type: 'tag_convention',
      rejected_tag: body?.rejected_tag ?? tag,
      applied_regex: body?.applied_regex,
      message: body?.message,
    })
  }
  throw new Error(`deployTag: ${res.status}`)
}

export interface StatusResponse {
  workloads: Record<string, Record<string, string>>
  fetched_at: string
  stale: boolean
}

export async function getProductStatus(token: string, productSlug: string): Promise<StatusResponse> {
  const res = await apiFetch(`/api/v1/products/${productSlug}/status`, token)
  if (!res.ok) {
    const body = (await res.json().catch(() => null)) as { error?: string } | null
    throw new Error(body?.error ?? `getProductStatus: ${res.status}`)
  }
  return (await res.json()) as StatusResponse
}

export interface Deployment {
  id: string
  actor_display_name: string
  component_name: string
  environment_name: string
  tag: string
  deployed_at: string
  commit_sha: string | null
  outcome: 'success' | 'failure'
  error_message?: string
}

export interface DeploymentHistoryResponse {
  deployments: Deployment[]
  page: number
  page_size: number
  total: number
}

export async function listDeployments(
  token: string,
  productSlug: string,
  page: number
): Promise<DeploymentHistoryResponse> {
  const res = await apiFetch(`/api/v1/products/${productSlug}/deployments?page=${page}`, token)
  if (!res.ok) {
    const body = (await res.json().catch(() => null)) as { error?: string } | null
    throw new Error(body?.error ?? `listDeployments: ${res.status}`)
  }
  return (await res.json().catch((err: unknown) => {
    throw new Error(`listDeployments: invalid response body: ${err instanceof Error ? err.message : JSON.stringify(err)}`)
  })) as DeploymentHistoryResponse
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

export interface AdminProduct {
  id: string
  name: string
  slug: string
  description: string
  created_at: string
  environment_count: number
  last_deployed_at: string | null
}

export async function listAdminProducts(token: string): Promise<AdminProduct[]> {
  const res = await apiFetch('/api/v1/admin/products', token)
  if (!res.ok) {
    const body = (await res.json().catch(() => null)) as { error?: string } | null
    throw new Error(body?.error ?? `listAdminProducts: ${res.status}`)
  }
  return (await res.json().catch((err: unknown) => {
    throw new Error(`listAdminProducts: invalid response body: ${err instanceof Error ? err.message : JSON.stringify(err)}`)
  })) as AdminProduct[]
}

export interface ActivityEvent {
  id: string
  actor_display_name: string
  tag: string
  component_name: string
  product_slug: string
  environment_name: string
  deployed_at: string
  outcome: 'in_progress' | 'success' | 'failure'
  error_message?: string | null
}

export async function listAdminActivity(token: string): Promise<ActivityEvent[]> {
  const res = await apiFetch('/api/v1/admin/activity', token)
  if (!res.ok) {
    const body = (await res.json().catch(() => null)) as { error?: string } | null
    throw new Error(body?.error ?? `listAdminActivity: ${res.status}`)
  }
  return (await res.json().catch((err: unknown) => {
    throw new Error(`listAdminActivity: invalid response body: ${err instanceof Error ? err.message : JSON.stringify(err)}`)
  })) as ActivityEvent[]
}
