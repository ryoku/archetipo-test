import { useEffect, useState, useCallback } from 'react'
import { useParams, useLocation, useNavigate } from 'react-router-dom'
import { useAuth } from '../auth/AuthContext'
import { listEnvironments, getProductStatus, type Environment, type Product, type ProductStatus } from '../api/products'
import ProductHero from '../components/ProductHero'
import ProductSubNav from '../components/ProductSubNav'
import ProductNotFound from '../components/ProductNotFound'
import StatusMatrix from '../components/StatusMatrix'
import './ProductDetailPage.css'

export default function StatusPage() {
  const { slug } = useParams<{ slug: string }>()
  const location = useLocation()
  const navigate = useNavigate()
  const { accessToken } = useAuth()

  const product = location.state as Product | undefined

  const [environments, setEnvironments] = useState<Environment[]>([])
  const [status, setStatus] = useState<ProductStatus | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const fetchStatus = useCallback(() => {
    if (!slug || !accessToken) return
    setLoading(true)
    setError(null)
    Promise.all([
      listEnvironments(accessToken, slug),
      getProductStatus(accessToken, slug),
    ])
      .then(([envs, s]) => {
        setEnvironments(envs)
        setStatus(s)
      })
      .catch((err: unknown) => {
        setError(err instanceof Error ? err.message : 'Failed to load deployment status')
      })
      .finally(() => setLoading(false))
  }, [slug, accessToken])

  useEffect(() => { fetchStatus() }, [fetchStatus])

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
            <div className="pd-section-sub">
              Image tags currently deployed per workload and environment.
            </div>
          </div>
          <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
            {status && (
              <span style={{ fontSize: 11, color: 'var(--text-muted)' }}>
                Updated {new Date(status.fetched_at).toLocaleTimeString()}
              </span>
            )}
            <button
              className="pd-btn-deploy"
              onClick={fetchStatus}
              disabled={loading}
              data-testid="refresh-btn"
            >
              <svg width="12" height="12" viewBox="0 0 12 12" fill="none" stroke="currentColor" strokeWidth="1.5">
                <path d="M10.5 6A4.5 4.5 0 113.5 2.5" strokeLinecap="round"/>
                <path d="M3.5 1v2H1.5" strokeLinecap="round" strokeLinejoin="round"/>
              </svg>
              Refresh
            </button>
          </div>
        </div>

        <div className="pd-card">
          <StatusMatrix
            status={status}
            environments={environments}
            loading={loading}
            error={error}
            onRefresh={fetchStatus}
          />
        </div>
      </div>
    </div>
  )
}
