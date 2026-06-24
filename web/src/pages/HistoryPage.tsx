import { useEffect, useState } from 'react'
import { useParams, useLocation, useNavigate } from 'react-router-dom'
import { useAuth } from '../auth/AuthContext'
import { listDeployments, type Product, type Deployment } from '../api/products'
import './ProductDetailPage.css'
import './HistoryPage.css'
import ProductHero from '../components/ProductHero'
import ProductSubNav from '../components/ProductSubNav'
import ProductNotFound from '../components/ProductNotFound'

function OutcomeBadge({ outcome }: { readonly outcome: string }) {
  if (outcome === 'success') {
    return (
      <span className="hist-outcome hist-outcome--success">
        <svg width="11" height="11" viewBox="0 0 11 11" fill="none" stroke="currentColor" strokeWidth="2">
          <path d="M2 5.5l2.5 2.5 4.5-4.5" />
        </svg>
        success
      </span>
    )
  }
  return (
    <span className="hist-outcome hist-outcome--failure">
      <svg width="11" height="11" viewBox="0 0 11 11" fill="none" stroke="currentColor" strokeWidth="2">
        <path d="M2.5 2.5l6 6M8.5 2.5l-6 6" />
      </svg>
      failure
    </span>
  )
}

function EnvBadge({ name }: { readonly name: string }) {
  const lower = name.toLowerCase()
  let cls = 'hist-env-badge hist-env-badge--dev'
  if (lower.includes('prod')) cls = 'hist-env-badge hist-env-badge--prod'
  else if (lower.includes('intg') || lower.includes('staging') || lower.includes('integration')) cls = 'hist-env-badge hist-env-badge--intg'
  return <span className={cls}>{name}</span>
}

function ActorAvatar({ name }: { readonly name: string }) {
  const initials = name
    .split(' ')
    .slice(0, 2)
    .map((w) => w.charAt(0))
    .join('')
    .toUpperCase()
  return <span className="hist-actor-avatar">{initials}</span>
}

function HistoryTable({ deployments }: { readonly deployments: Deployment[] }) {
  if (deployments.length === 0) {
    return (
      <div className="pd-empty-state">
        <div className="pd-empty-title">No deployments yet</div>
        <div className="pd-empty-sub">Deployment records will appear here after the first deploy.</div>
      </div>
    )
  }
  return (
    <div className="pd-tbl-wrap">
      <table className="pd-comp-table hist-table">
        <thead>
          <tr>
            <th>Actor</th>
            <th>Component</th>
            <th>Environment</th>
            <th>Tag</th>
            <th>Date</th>
            <th>Outcome</th>
          </tr>
        </thead>
        <tbody>
          {deployments.map((d) => (
            <tr key={d.id} className={d.outcome === 'failure' ? 'hist-row--failure' : undefined}>
              <td>
                <div className="hist-actor-cell">
                  <ActorAvatar name={d.actor_display_name} />
                  <span className="hist-actor-name">{d.actor_display_name}</span>
                </div>
              </td>
              <td><span className="hist-comp-name">{d.component_name}</span></td>
              <td><EnvBadge name={d.environment_name} /></td>
              <td><span className="hist-tag-chip">{d.tag}</span></td>
              <td className="hist-ts-cell">
                <div className="hist-ts-abs">{new Date(d.deployed_at).toLocaleString()}</div>
              </td>
              <td><OutcomeBadge outcome={d.outcome} /></td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}

export default function HistoryPage() {
  const { slug } = useParams<{ slug: string }>()
  const location = useLocation()
  const navigate = useNavigate()
  const { accessToken } = useAuth()

  const product = location.state as Product | undefined

  const [deployments, setDeployments] = useState<Deployment[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [page, setPage] = useState(1)
  const [total, setTotal] = useState(0)
  const pageSize = 20

  useEffect(() => {
    if (!slug || !accessToken) {
      // eslint-disable-next-line react-hooks/set-state-in-effect
      setLoading(false)
      setError('You must be signed in to view deployment history.')
      return
    }
    // Intentional reset: page/slug changed, clear stale data and show loading before new fetch.
    setLoading(true)
    setError(null)
    listDeployments(accessToken, slug, page)
      .then((res) => {
        setDeployments(res.deployments)
        setTotal(res.total)
      })
      .catch((err: unknown) => {
        setError(err instanceof Error ? err.message : 'Failed to load deployment history')
      })
      .finally(() => { setLoading(false) })
  }, [slug, accessToken, page])

  if (!product) {
    return <ProductNotFound />
  }

  const totalPages = Math.max(1, Math.ceil(total / pageSize))

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
        <ProductSubNav activeTab="history" product={product} />
      </div>

      <div className="pd-page-body">
        <div className="pd-section-head">
          <div>
            <div className="pd-section-label">Deployment history</div>
            {!loading && !error && (
              <div className="hist-total">{total} deployments recorded</div>
            )}
          </div>
        </div>

        {error && (
          <div className="pd-error" role="alert">
            <strong>Error:</strong> {error}
          </div>
        )}

        <div className="pd-card">
          {loading ? (
            <div className="pd-loading">
              <div className="pd-spinner" />
              <span>Loading deployment history…</span>
            </div>
          ) : (
            <HistoryTable deployments={deployments} />
          )}

          {!loading && !error && total > 0 && (
            <div className="hist-pagination">
              <span className="hist-page-info">
                Page <strong>{page}</strong> of <strong>{totalPages}</strong>
              </span>
              <button
                className="hist-btn-page"
                disabled={page <= 1}
                onClick={() => setPage((p) => p - 1)}
              >
                <svg width="12" height="12" viewBox="0 0 12 12" fill="none" stroke="currentColor" strokeWidth="1.8">
                  <path d="M7.5 2.5L4 6l3.5 3.5" />
                </svg>
                Previous
              </button>
              <button
                className="hist-btn-page"
                disabled={page >= totalPages}
                onClick={() => setPage((p) => p + 1)}
              >
                Next
                <svg width="12" height="12" viewBox="0 0 12 12" fill="none" stroke="currentColor" strokeWidth="1.8">
                  <path d="M4.5 2.5L8 6l-3.5 3.5" />
                </svg>
              </button>
            </div>
          )}
        </div>
      </div>
    </div>
  )
}
