import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { useAuth } from '../auth/AuthContext'
import { listProducts, type Product } from '../api/products'
import './HomePage.css'

export default function HomePage() {
  const { user, logout, accessToken } = useAuth()
  const navigate = useNavigate()
  const [products, setProducts] = useState<Product[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    if (!accessToken) return
    listProducts(accessToken)
      .then(setProducts)
      .catch((err: unknown) => {
        setError(err instanceof Error ? err.message : 'Failed to load products')
      })
      .finally(() => setLoading(false))
  }, [accessToken])

  const displayName =
    user?.profile?.name ?? user?.profile?.preferred_username ?? 'User'

  function getInitials(name: string): string {
    return name
      .split(/\s+/)
      .map((w) => w[0])
      .join('')
      .toUpperCase()
      .slice(0, 2)
  }

  return (
    <div className="home-page">
      <header className="home-header">
        <div className="home-header-brand">
          <div className="home-logo-mark">
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
          <span className="home-logo-name">KubeGate</span>
        </div>
        <div className="home-header-user">
          <div className="home-avatar">{getInitials(displayName)}</div>
          <span className="home-user-name">{displayName}</span>
          <button className="home-btn-logout" onClick={() => logout()}>
            Logout
          </button>
        </div>
      </header>

      <main className="home-main">
        <div className="home-page-top">
          <h1 className="home-page-title">Products</h1>
          <p className="home-page-desc">
            Select a product to view and manage its components.
          </p>
        </div>

        {loading && (
          <div className="home-loading" data-testid="loading-state">
            <div className="home-spinner" />
            <span>Loading products…</span>
          </div>
        )}

        {!loading && error && (
          <div className="home-error" role="alert">
            <strong>Error:</strong> {error}
          </div>
        )}

        {!loading && !error && products.length === 0 && (
          <div className="home-empty" data-testid="empty-state">
            <div className="home-empty-icon">
              <svg
                width="24"
                height="24"
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                strokeWidth="1.5"
              >
                <rect x="3" y="3" width="7" height="7" rx="1" />
                <rect x="14" y="3" width="7" height="7" rx="1" />
                <rect x="3" y="14" width="7" height="7" rx="1" />
                <rect x="14" y="14" width="7" height="7" rx="1" />
              </svg>
            </div>
            <p className="home-empty-title">No products yet</p>
            <p className="home-empty-sub">
              Products will appear here once they are created.
            </p>
          </div>
        )}

        {!loading && !error && products.length > 0 && (
          <div className="home-product-grid">
            {products.map((product) => (
              <button
                key={product.id}
                className="home-product-card"
                onClick={() =>
                  navigate(`/products/${product.slug}`, { state: product })
                }
              >
                <div className="home-p-card-top">
                  <div className="home-p-icon">{getInitials(product.name)}</div>
                  <div className="home-p-info">
                    <div className="home-p-name">{product.name}</div>
                    <div className="home-p-slug">{product.slug}</div>
                  </div>
                </div>
                {product.description && (
                  <p className="home-p-desc">{product.description}</p>
                )}
              </button>
            ))}
          </div>
        )}
      </main>
    </div>
  )
}
