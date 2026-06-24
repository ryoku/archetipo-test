import { useEffect, useState } from 'react'
import { useParams, useLocation, useNavigate } from 'react-router-dom'
import { useAuth } from '../auth/AuthContext'
import { getProductStatus, type Product, type StatusResponse } from '../api/products'
import './ProductDetailPage.css'
import './ProductStatusPage.css'
import ProductHero from '../components/ProductHero'
import ProductSubNav from '../components/ProductSubNav'
import ProductNotFound from '../components/ProductNotFound'

interface StatusMatrixProps {
  readonly loading: boolean
  readonly status: StatusResponse | null
  readonly error: string | null
  readonly onRefresh: () => void
}

function StatusMatrix({ loading, status, error, onRefresh }: StatusMatrixProps) {
  if (loading) {
    return (
      <div className="pd-loading">
        <div className="pd-spinner" />
        <span>Loading deployment status…</span>
      </div>
    )
  }

  if (error) {
    return (
      <div className="pd-empty-state" data-testid="status-error">
        <div className="pd-empty-title">Failed to load status</div>
        <div className="pd-empty-sub">{error}</div>
        <button className="ps-btn-refresh" onClick={onRefresh} data-testid="status-retry">
          Retry
        </button>
      </div>
    )
  }

  if (!status) return null

  const cmp = (a: string, b: string) => a.localeCompare(b)
  const workloadNames = Object.keys(status.workloads).sort(cmp)
  const envSlugs = Array.from(
    new Set(workloadNames.flatMap((wl) => Object.keys(status.workloads[wl]))),
  ).sort(cmp)

  if (workloadNames.length === 0) {
    return (
      <div className="pd-empty-state" data-testid="status-empty">
        <div className="pd-empty-title">No workloads found</div>
        <div className="pd-empty-sub">No workloads with a deployed image tag were found in the gitops repo.</div>
      </div>
    )
  }

  return (
    <div data-testid="status-matrix">
      <div className="ps-matrix-header">
        {status.stale && (
          <span className="ps-stale-badge" data-testid="stale-badge" title="Data is older than the cache TTL">
            Stale
          </span>
        )}
        <span className="ps-fetched-at">
          Last fetched: {new Date(status.fetched_at).toLocaleTimeString()}
        </span>
        <button className="ps-btn-refresh" onClick={onRefresh} data-testid="status-refresh">
          Refresh
        </button>
      </div>
      <div className="pd-tbl-wrap">
        <table className="pd-comp-table ps-status-table">
          <thead>
            <tr>
              <th>Workload</th>
              {envSlugs.map((env) => (
                <th key={env}>{env}</th>
              ))}
            </tr>
          </thead>
          <tbody>
            {workloadNames.map((wl) => (
              <tr key={wl}>
                <td className="pd-comp-name-cell">
                  <div className="pd-comp-name-str">{wl}</div>
                </td>
                {envSlugs.map((env) => {
                  const tag = status.workloads[wl][env]
                  return (
                    <td key={env}>
                      <span
                        className={`ps-tag-chip${tag === 'N/A' ? ' ps-tag-chip--na' : ''}`}
                        data-testid={`tag-${wl}-${env}`}
                      >
                        {tag}
                      </span>
                    </td>
                  )
                })}
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  )
}

export default function ProductStatusPage() {
  const { slug } = useParams<{ slug: string }>()
  const location = useLocation()
  const navigate = useNavigate()
  const { accessToken } = useAuth()

  const product = location.state as Product | undefined

  const [status, setStatus] = useState<StatusResponse | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [refreshToken, setRefreshToken] = useState(0)

  useEffect(() => {
    if (!slug || !accessToken) return
    let cancelled = false
    getProductStatus(accessToken, slug)
      .then((s) => { if (!cancelled) { setStatus(s); setError(null) } })
      .catch((err: unknown) => {
        if (!cancelled) setError(err instanceof Error ? err.message : 'Failed to load status')
      })
      .finally(() => { if (!cancelled) setLoading(false) })
    return () => { cancelled = true }
  }, [slug, accessToken, refreshToken])

  function handleRefresh() {
    setLoading(true)
    setRefreshToken((t) => t + 1)
  }

  if (!product) {
    return <ProductNotFound />
  }

  return (
    <div className="pd-page">
      <div className="pd-page-top">
        <div className="pd-breadcrumb">
          <button className="pd-bc-link" onClick={() => navigate('/')}>
            Products
          </button>
          <span className="pd-bc-sep">/</span>
          <span>{product.slug}</span>
        </div>

        <ProductHero product={product} />

        <ProductSubNav activeTab="status" product={product} />
      </div>

      <div className="pd-page-body">
        <div className="pd-section-head">
          <div>
            <div className="pd-section-label">Deployment Status</div>
            <div className="pd-section-sub">Currently deployed image tag per workload and environment.</div>
          </div>
        </div>

        <div className="pd-card">
          <StatusMatrix
            loading={loading}
            status={status}
            error={error}
            onRefresh={handleRefresh}
          />
        </div>
      </div>
    </div>
  )
}
