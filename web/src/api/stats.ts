import { apiFetch } from './client'

export interface Stats {
  product_count: number
  environment_count: number
  component_count: number
  deployments_last_24h: number
}

export async function fetchStats(token: string): Promise<Stats> {
  const res = await apiFetch('/api/v1/stats', token)
  if (!res.ok) throw new Error(`fetchStats: ${res.status}`)
  return (await res.json()) as Stats
}
