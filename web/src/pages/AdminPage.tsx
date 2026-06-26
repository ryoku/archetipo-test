import { useEffect, useMemo, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { useAuth } from '../auth/AuthContext'
import { listAdminProducts, type AdminProduct, listAdminActivity, type ActivityEvent } from '../api/products'
import './AdminPage.css'

type SortCol = 'name' | 'environment_count' | 'last_deployed_at'
type SortDir = 'asc' | 'desc'

function getInitials(name: string): string {
  return name.split(/\s+/).map((w) => w[0]).join('').toUpperCase().slice(0, 2)
}

function formatRelativeTime(iso: string): string {
  const diffMs = Date.now() - new Date(iso).getTime()
  const minutes = Math.floor(diffMs / 60000)
  if (minutes < 1) return 'adesso'
  if (minutes < 60) return `${minutes}m fa`
  const hours = Math.floor(minutes / 60)
  if (hours < 24) return `${hours}h fa`
  return `${Math.floor(hours / 24)}g fa`
}

function formatDate(iso: string | null): string {
  if (!iso) return 'Never'
  return new Date(iso).toLocaleDateString('en-GB', { day: 'numeric', month: 'short', year: 'numeric' })
}

const ICON_COLORS = ['a', 'b', 'c', 'd'] as const

function compareSortCol(a: AdminProduct, b: AdminProduct, col: SortCol): number {
  if (col === 'name') return a.name.localeCompare(b.name)
  if (col === 'environment_count') return a.environment_count - b.environment_count
  return (a.last_deployed_at ?? '').localeCompare(b.last_deployed_at ?? '')
}

function SortArrow({ active, dir }: Readonly<{ active: boolean; dir: SortDir }>) {
  return (
    <span className={`admin-sort-arrow${active ? ' active' : ''}${active && dir === 'desc' ? ' desc' : ''}`}>
      <svg width="8" height="8" viewBox="0 0 8 8" fill="none" stroke="currentColor" strokeWidth="1.5">
        <path d="M4 1v6M1.5 4.5L4 7l2.5-2.5" />
      </svg>
    </span>
  )
}

export default function AdminPage() {
  const { user, logout, accessToken } = useAuth()
  const navigate = useNavigate()
  const [products, setProducts] = useState<AdminProduct[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [sortCol, setSortCol] = useState<SortCol>('name')
  const [sortDir, setSortDir] = useState<SortDir>('asc')
  const [activity, setActivity] = useState<ActivityEvent[]>([])
  const [activityLoading, setActivityLoading] = useState(true)

  useEffect(() => {
    if (!accessToken) return
    const token = accessToken
    let cancelled = false

    function fetchActivity() {
      listAdminActivity(token)
        .then((data) => { if (!cancelled) setActivity(data) })
        .catch((err: unknown) => { console.error('[AdminPage] listAdminActivity failed:', err) })
        .finally(() => { if (!cancelled) setActivityLoading(false) })
    }

    fetchActivity()
    // Poll every 30s for the "live" effect, but skip while the tab is hidden
    // to avoid pointless requests in a backgrounded tab.
    const timer = setInterval(() => {
      if (document.visibilityState === 'visible') fetchActivity()
    }, 30000)
    return () => { cancelled = true; clearInterval(timer) }
  }, [accessToken])

  useEffect(() => {
    if (!accessToken) return
    let cancelled = false
    listAdminProducts(accessToken)
      .then((data) => { if (!cancelled) setProducts(data) })
      .catch((err: unknown) => {
        if (!cancelled) {
          console.error('[AdminPage] listAdminProducts failed:', err)
          setError(err instanceof Error ? err.message : 'Failed to load products')
        }
      })
      .finally(() => { if (!cancelled) setLoading(false) })
    return () => { cancelled = true }
  }, [accessToken])

  const displayName = user?.profile.name ?? user?.profile.preferred_username ?? 'User'

  const totalEnvs = products.reduce((sum, p) => sum + p.environment_count, 0)
  const withDeployments = products.filter((p) => p.last_deployed_at !== null).length

  const sorted = useMemo(() => {
    return [...products].sort((a, b) => {
      const cmp = compareSortCol(a, b, sortCol)
      return sortDir === 'asc' ? cmp : -cmp
    })
  }, [products, sortCol, sortDir])

  function handleSort(col: SortCol) {
    if (col === sortCol) {
      setSortDir((d) => (d === 'asc' ? 'desc' : 'asc'))
    } else {
      setSortCol(col)
      setSortDir('asc')
    }
  }

  function thClass(col: SortCol) {
    let cls = 'admin-th-sortable'
    if (col === sortCol) cls += ' sort-active'
    if (col === sortCol && sortDir === 'desc') cls += ' sort-desc'
    return cls
  }

  return (
    <div className="admin-page">
      <header className="admin-header">
        <div className="admin-header-brand">
          <div className="admin-logo-mark">
            <svg width="16" height="16" viewBox="0 0 16 16" fill="none">
              <path
                d="M8 1L13 4V8C13 11 10.5 13.5 8 15C5.5 13.5 3 11 3 8V4L8 1Z"
                stroke="white"
                strokeWidth="1.5"
                strokeLinejoin="round"
              />
              <path
                d="M6 8L7.5 9.5L10 7"
                stroke="white"
                strokeWidth="1.5"
                strokeLinecap="round"
                strokeLinejoin="round"
              />
            </svg>
          </div>
          <span className="admin-logo-name">KubeGate</span>
        </div>
        <div className="admin-header-user">
          <div className="admin-avatar">{getInitials(displayName)}</div>
          <span className="admin-user-name">{displayName}</span>
          <button className="admin-btn-logout" onClick={() => { void logout() }}>
            Logout
          </button>
        </div>
      </header>

      <main className="admin-main">
        <div className="admin-page-head">
          <h1 className="admin-page-title">All Products — Admin</h1>
          <p className="admin-page-desc">Platform-wide product inventory with environment coverage and latest deployment status.</p>
        </div>

        <div className="admin-stats-bar" data-testid="stats-bar">
          <div className="admin-stat-card accent-left">
            <div className="admin-stat-val">{loading ? '—' : products.length}</div>
            <div className="admin-stat-key">Total products</div>
          </div>
          <div className="admin-stat-card info-left">
            <div className="admin-stat-val">{loading ? '—' : totalEnvs}</div>
            <div className="admin-stat-key">Total environments</div>
          </div>
          <div className="admin-stat-card ok-left">
            <div className="admin-stat-val">{loading ? '—' : withDeployments}</div>
            <div className="admin-stat-key">With deployments</div>
          </div>
        </div>

        <div className="admin-table-section">
          <div className="admin-table-header">
            <span className="admin-table-title">Products</span>
            <span className="admin-table-count" data-testid="table-count">
              {loading ? '—' : `${products.length} products`}
            </span>
          </div>

          {loading && (
            <div data-testid="loading-state">
              <table className="admin-data-table">
                <thead>
                  <tr>
                    <th>Name</th>
                    <th>Environments</th>
                    <th>Last Deployment</th>
                  </tr>
                </thead>
                <tbody>
                  {[0, 1, 2].map((i) => (
                    <tr key={i} className="admin-skel-row">
                      <td>
                        <div className="admin-skeleton" style={{ height: 12, width: 140, marginBottom: 4, borderRadius: 4 }} />
                        <div className="admin-skeleton" style={{ height: 9, width: 90, borderRadius: 4 }} />
                      </td>
                      <td><div className="admin-skeleton" style={{ height: 12, width: 40, borderRadius: 4 }} /></td>
                      <td><div className="admin-skeleton" style={{ height: 12, width: 110, borderRadius: 4 }} /></td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}

          {!loading && error && (
            <div className="admin-error">{error}</div>
          )}

          {!loading && !error && products.length === 0 && (
            <div className="admin-empty" data-testid="empty-state">
              <div className="admin-empty-icon">
                <svg width="22" height="22" viewBox="0 0 22 22" fill="none" stroke="currentColor" strokeWidth="1.4">
                  <rect x="3" y="3" width="7" height="7" rx="1.5" />
                  <rect x="12" y="3" width="7" height="7" rx="1.5" />
                  <rect x="3" y="12" width="7" height="7" rx="1.5" />
                  <rect x="12" y="12" width="7" height="7" rx="1.5" />
                </svg>
              </div>
              <p className="admin-empty-title">No products registered</p>
              <p className="admin-empty-sub">Products will appear here once they are created.</p>
            </div>
          )}

          {!loading && !error && products.length > 0 && (
            <table className="admin-data-table" data-testid="products-table">
              <thead>
                <tr>
                  <th className={thClass('name')} onClick={() => handleSort('name')}>
                    <span className="admin-sort-indicator">
                      Name <SortArrow active={sortCol === 'name'} dir={sortDir} />
                    </span>
                  </th>
                  <th className={thClass('environment_count')} onClick={() => handleSort('environment_count')}>
                    <span className="admin-sort-indicator">
                      Environments <SortArrow active={sortCol === 'environment_count'} dir={sortDir} />
                    </span>
                  </th>
                  <th className={thClass('last_deployed_at')} onClick={() => handleSort('last_deployed_at')}>
                    <span className="admin-sort-indicator">
                      Last Deployment <SortArrow active={sortCol === 'last_deployed_at'} dir={sortDir} />
                    </span>
                  </th>
                </tr>
              </thead>
              <tbody>
                {sorted.map((product, idx) => (
                  <tr
                    key={product.id}
                    data-testid="product-row"
                    onClick={() => navigate('/products/' + product.slug, { state: product })}
                  >
                    <td>
                      <div className="admin-product-name-cell">
                        <div className={`admin-p-icon admin-p-icon-${ICON_COLORS[idx % 4]}`}>
                          {getInitials(product.name)}
                        </div>
                        <div>
                          <div className="admin-p-name">{product.name}</div>
                          <div className="admin-p-slug">{product.slug}</div>
                        </div>
                      </div>
                    </td>
                    <td>
                      <span className="admin-env-num">{product.environment_count}</span>
                    </td>
                    <td>
                      {product.last_deployed_at === null ? (
                        <span className="admin-never-text">Never</span>
                      ) : (
                        <div className="admin-deploy-cell">
                          <div className="admin-deploy-dot" />
                          <span className="admin-deploy-date">{formatDate(product.last_deployed_at)}</span>
                        </div>
                      )}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </div>

        <div className="admin-activity-section" data-testid="activity-section">
          <div className="admin-table-header">
            <span className="admin-table-title">◈ Attività live</span>
            <span className="admin-table-count">Ultimi 10 eventi</span>
          </div>
          {activityLoading && (
            <div className="admin-activity-empty" data-testid="activity-loading">
              <span>Caricamento…</span>
            </div>
          )}
          {!activityLoading && activity.length === 0 && (
            <div className="admin-activity-empty" data-testid="activity-empty">
              <span>Nessun evento di deployment recente.</span>
            </div>
          )}
          {!activityLoading && activity.length > 0 && (
            <div className="admin-activity-list" data-testid="activity-list">
              {activity.map((event) => {
                let dotClass: string
                if (event.outcome === 'in_progress') dotClass = 'dot-pulse'
                else if (event.outcome === 'success') dotClass = 'dot-ok'
                else dotClass = 'dot-err'
                return (
                <div key={event.id} className="admin-activity-row" data-testid="activity-row">
                  <span
                    className={`admin-activity-dot ${dotClass}`}
                    data-testid={`activity-dot-${event.outcome}`}
                  />
                  <div className="admin-activity-avatar">
                    {getInitials(event.actor_display_name)}
                  </div>
                  <div className="admin-activity-body">
                    <div className="admin-activity-desc">
                      <span className="admin-activity-actor">{event.actor_display_name}</span>
                      {' ha rilasciato '}
                      <span className="admin-activity-tag">{event.tag}</span>
                      {' → '}
                      <span className="admin-activity-comp">{event.component_name}</span>
                      {' / '}
                      <span>{event.environment_name}</span>
                    </div>
                    {event.outcome === 'failure' && event.error_message && (
                      <div className="admin-activity-error" data-testid="activity-error-msg">
                        {event.error_message}
                      </div>
                    )}
                  </div>
                  <div className="admin-activity-meta">
                    <span className="admin-activity-slug">{event.product_slug}</span>
                    <span className="admin-activity-when">{formatRelativeTime(event.deployed_at)}</span>
                  </div>
                </div>
                )
              })}
            </div>
          )}
        </div>
      </main>
    </div>
  )
}
