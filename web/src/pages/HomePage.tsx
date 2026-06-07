import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { useAuth } from '../auth/AuthContext'
import { listProducts, createProduct, type Product } from '../api/products'
import { isDevOpsAdmin } from '../auth/claims'
import './ProductDetailPage.css'
import './HomePage.css'

const SLUG_REGEX = /^[a-z0-9]+(-[a-z0-9]+)*$/

interface ProductFormState { name: string; slug: string; description: string }
interface ProductFormErrors { name?: string; slug?: string }

function getInitials(name: string): string {
  return name
    .split(/\s+/)
    .map((w) => w[0])
    .join('')
    .toUpperCase()
    .slice(0, 2)
}

export default function HomePage() {
  const { user, logout, accessToken } = useAuth()
  const navigate = useNavigate()
  const [products, setProducts] = useState<Product[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const canCreate = isDevOpsAdmin(accessToken)

  // Add form state
  const [showForm, setShowForm] = useState(false)
  const [formState, setFormState] = useState<ProductFormState>({ name: '', slug: '', description: '' })
  const [formErrors, setFormErrors] = useState<ProductFormErrors>({})
  const [formSubmitting, setFormSubmitting] = useState(false)
  const [formError, setFormError] = useState<string | null>(null)

  useEffect(() => {
    if (!accessToken) return
    listProducts(accessToken)
      .then(setProducts)
      .catch((err: unknown) => {
        setError(err instanceof Error ? err.message : 'Failed to load products')
      })
      .finally(() => { setLoading(false) })
  }, [accessToken])

  const displayName =
    user?.profile.name ?? user?.profile.preferred_username ?? 'User'

  function validateForm(): boolean {
    const errs: ProductFormErrors = {}
    if (!formState.name.trim()) errs.name = 'Name is required'
    if (!formState.slug.trim()) {
      errs.slug = 'Slug is required'
    } else if (!SLUG_REGEX.test(formState.slug.trim())) {
      errs.slug = 'Slug must be lowercase letters, numbers, and hyphens only'
    }
    setFormErrors(errs)
    return Object.keys(errs).length === 0
  }

  async function handleFormSubmit() {
    if (!validateForm() || !accessToken) return
    setFormSubmitting(true)
    setFormError(null)
    try {
      const newProduct = await createProduct(accessToken, {
        name: formState.name.trim(),
        slug: formState.slug.trim(),
        description: formState.description.trim(),
      })
      setProducts((prev) => [newProduct, ...prev])
      setShowForm(false)
      setFormState({ name: '', slug: '', description: '' })
      setFormErrors({})
      setFormError(null)
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Failed to create product'
      if (msg.includes('409')) {
        setFormError('A product with this slug already exists')
      } else {
        setFormError(msg)
      }
    } finally {
      setFormSubmitting(false)
    }
  }

  function handleCancelForm() {
    setShowForm(false)
    setFormState({ name: '', slug: '', description: '' })
    setFormErrors({})
    setFormError(null)
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
          <button className="home-btn-logout" onClick={() => { void logout() }}>
            Logout
          </button>
        </div>
      </header>

      <main className="home-main">
        <div className="home-page-top">
          <div className="home-page-head">
            <h1 className="home-page-title">Products</h1>
            <p className="home-page-desc">
              Select a product to view and manage its components.
            </p>
          </div>
          {canCreate && !showForm && (
            <button
              type="button"
              className="home-btn-add-product"
              onClick={() => setShowForm(true)}
            >
              <svg width="13" height="13" viewBox="0 0 13 13" fill="none" stroke="currentColor" strokeWidth="2">
                <path d="M6.5 1v11M1 6.5h11" />
              </svg>
              Add Product
            </button>
          )}
        </div>

        {showForm && (
          <div className="pd-inline-form home-inline-form">
            <div className="pd-inline-form-title">
              <svg width="13" height="13" viewBox="0 0 12 12" fill="none" stroke="currentColor" strokeWidth="2">
                <path d="M6 2v8M2 6h8" />
              </svg>
              New Product
            </div>
            {formError && (
              <div className="home-form-error" role="alert">
                {formError}
              </div>
            )}
            <div className="pd-form-row">
              <div className="pd-field">
                <label className="pd-field-label" htmlFor="prod-name">Name *</label>
                <input
                  id="prod-name"
                  type="text"
                  className="pd-input"
                  placeholder="e.g. Payments Service"
                  value={formState.name}
                  onChange={(e) => setFormState((prev) => ({ ...prev, name: e.target.value }))}
                  autoComplete="off"
                />
                {formErrors.name ? (
                  <span className="pd-field-error">{formErrors.name}</span>
                ) : (
                  <span className="pd-field-hint">display name</span>
                )}
              </div>
              <div className="pd-field">
                <label className="pd-field-label" htmlFor="prod-slug">Slug *</label>
                <input
                  id="prod-slug"
                  type="text"
                  className="pd-input pd-input-mono"
                  placeholder="payments-service"
                  value={formState.slug}
                  onChange={(e) => setFormState((prev) => ({ ...prev, slug: e.target.value }))}
                  autoComplete="off"
                />
                {formErrors.slug ? (
                  <span className="pd-field-error">{formErrors.slug}</span>
                ) : (
                  <span className="pd-field-hint">lowercase, letters, digits and hyphens</span>
                )}
              </div>
              <div className="pd-field">
                <label className="pd-field-label" htmlFor="prod-description">Description</label>
                <input
                  id="prod-description"
                  type="text"
                  className="pd-input"
                  placeholder="brief description (optional)"
                  value={formState.description}
                  onChange={(e) => setFormState((prev) => ({ ...prev, description: e.target.value }))}
                  autoComplete="off"
                />
                <span className="pd-field-hint">optional</span>
              </div>
            </div>
            <div className="pd-form-actions">
              <button
                type="button"
                className="pd-btn-ghost"
                onClick={handleCancelForm}
                disabled={formSubmitting}
              >
                Cancel
              </button>
              <button
                type="button"
                className="pd-btn-primary"
                onClick={() => { void handleFormSubmit() }}
                disabled={formSubmitting}
              >
                <svg width="11" height="11" viewBox="0 0 11 11" fill="none" stroke="currentColor" strokeWidth="2">
                  <path d="M1.5 5.5l3 3L9.5 2.5" />
                </svg>
                {formSubmitting ? 'Saving…' : 'Save Product'}
              </button>
            </div>
          </div>
        )}

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
