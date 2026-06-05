import { useEffect, useState } from 'react'
import { useParams, useLocation, useNavigate } from 'react-router-dom'
import { useAuth } from '../auth/AuthContext'
import {
  listComponents,
  createComponent,
  deleteComponent,
  type Product,
  type Component,
} from '../api/products'
import './ProductDetailPage.css'

interface FormState {
  name: string
  slug: string
  gcr_image_path: string
}

interface FormErrors {
  name?: string
  slug?: string
  gcr_image_path?: string
}

function getInitials(name: string): string {
  return name
    .split(/\s+/)
    .map((w) => w[0])
    .join('')
    .toUpperCase()
    .slice(0, 2)
}

function formatDate(iso: string): string {
  try {
    return new Date(iso).toLocaleDateString('en-GB', {
      year: 'numeric',
      month: 'short',
      day: 'numeric',
    })
  } catch {
    return iso.slice(0, 10)
  }
}

export default function ProductDetailPage() {
  const { slug } = useParams<{ slug: string }>()
  const location = useLocation()
  const navigate = useNavigate()
  const { accessToken } = useAuth()

  const product = location.state as Product | undefined
  const canWrite = product?.my_role === 'editor' || product?.my_role === 'admin'

  const [components, setComponents] = useState<Component[]>([])
  const [loadingComponents, setLoadingComponents] = useState(true)
  const [error, setError] = useState<string | null>(null)

  // Add form state
  const [showForm, setShowForm] = useState(false)
  const [formState, setFormState] = useState<FormState>({
    name: '',
    slug: '',
    gcr_image_path: '',
  })
  const [formErrors, setFormErrors] = useState<FormErrors>({})
  const [formSubmitting, setFormSubmitting] = useState(false)

  // Delete confirm dialog state
  const [deleteTarget, setDeleteTarget] = useState<Component | null>(null)
  const [deleteInProgress, setDeleteInProgress] = useState(false)

  useEffect(() => {
    if (!slug || !accessToken) return
    listComponents(accessToken, slug)
      .then(setComponents)
      .catch((err: unknown) => {
        setError(err instanceof Error ? err.message : 'Failed to load components')
      })
      .finally(() => { setLoadingComponents(false) })
  }, [slug, accessToken])

  if (!product) {
    return (
      <div className="pd-page">
        <div className="pd-not-found">
          <p>Product not found. Please go back and select a product.</p>
          <button className="pd-btn-back" onClick={() => navigate('/')}>
            ← Back to Products
          </button>
        </div>
      </div>
    )
  }

  function handleNameChange(value: string) {
    const autoSlug = value
      .toLowerCase()
      .replace(/[^a-z0-9]+/g, '-')
      .replace(/^-|-$/g, '')
    setFormState((prev) => ({ ...prev, name: value, slug: autoSlug }))
  }

  function validateForm(): boolean {
    const errs: FormErrors = {}
    if (!formState.name.trim()) errs.name = 'Name is required'
    if (!formState.slug.trim()) {
      errs.slug = 'Slug is required'
    } else if (!/^[a-z0-9]+(?:-[a-z0-9]+)*$/.test(formState.slug)) {
      errs.slug = 'Only lowercase letters, digits and hyphens'
    }
    if (!formState.gcr_image_path.trim()) errs.gcr_image_path = 'GCR image path is required'
    setFormErrors(errs)
    return Object.keys(errs).length === 0
  }

  async function handleFormSubmit() {
    if (!validateForm() || !accessToken || !slug) return
    setFormSubmitting(true)
    setError(null)
    try {
      const newComp = await createComponent(accessToken, slug, {
        name: formState.name.trim(),
        slug: formState.slug.trim(),
        gcr_image_path: formState.gcr_image_path.trim(),
      })
      setComponents((prev) => [...prev, newComp])
      setShowForm(false)
      setFormState({ name: '', slug: '', gcr_image_path: '' })
      setFormErrors({})
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Failed to create component')
    } finally {
      setFormSubmitting(false)
    }
  }

  function handleCancelForm() {
    setShowForm(false)
    setFormState({ name: '', slug: '', gcr_image_path: '' })
    setFormErrors({})
  }

  async function handleDeleteConfirm() {
    if (!deleteTarget || !accessToken || !slug) return
    setDeleteInProgress(true)
    setError(null)
    try {
      await deleteComponent(accessToken, slug, deleteTarget.slug)
      setComponents((prev) => prev.filter((c) => c.id !== deleteTarget.id))
      setDeleteTarget(null)
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Failed to delete component')
      setDeleteTarget(null)
    } finally {
      setDeleteInProgress(false)
    }
  }

  return (
    <div className="pd-page">
      {/* Page top / hero */}
      <div className="pd-page-top">
        <div className="pd-breadcrumb">
          <button className="pd-bc-link" onClick={() => navigate('/')}>
            Products
          </button>
          <span className="pd-bc-sep">/</span>
          <span>{product.slug}</span>
        </div>

        <div className="pd-product-hero">
          <div className="pd-hero-icon">{getInitials(product.name)}</div>
          <div>
            <div className="pd-hero-name">{product.name}</div>
            <div className="pd-hero-meta">
              <span className="pd-tag-chip">{product.slug}</span>
            </div>
            {product.description && (
              <p className="pd-hero-desc">{product.description}</p>
            )}
          </div>
        </div>
      </div>

      {/* Page body */}
      <div className="pd-page-body">
        {error && (
          <div className="pd-error" role="alert">
            <strong>Error:</strong> {error}
          </div>
        )}

        {/* Components section */}
        <div className="pd-section-head">
          <div>
            <div className="pd-section-label">Registered Components</div>
            <div className="pd-section-sub">
              Each component maps to a Google Artifact Registry image repository.
            </div>
          </div>
          {!showForm && canWrite && (
            <button
              className="pd-btn-primary"
              onClick={() => setShowForm(true)}
            >
              <svg width="12" height="12" viewBox="0 0 12 12" fill="none" stroke="currentColor" strokeWidth="2">
                <path d="M6 2v8M2 6h8" />
              </svg>
              Add Component
            </button>
          )}
        </div>

        {/* Inline add form */}
        {showForm && (
          <div className="pd-inline-form">
            <div className="pd-inline-form-title">
              <svg width="13" height="13" viewBox="0 0 12 12" fill="none" stroke="currentColor" strokeWidth="2">
                <path d="M6 2v8M2 6h8" />
              </svg>
              New Component
            </div>
            <div className="pd-form-row">
              <div className="pd-field">
                <label className="pd-field-label" htmlFor="comp-name">
                  Name *
                </label>
                <input
                  id="comp-name"
                  type="text"
                  className="pd-input"
                  placeholder="e.g. api-gateway"
                  value={formState.name}
                  onChange={(e) => handleNameChange(e.target.value)}
                  autoComplete="off"
                />
                {formErrors.name ? (
                  <span className="pd-field-error">{formErrors.name}</span>
                ) : (
                  <span className="pd-field-hint">display name</span>
                )}
              </div>
              <div className="pd-field">
                <label className="pd-field-label" htmlFor="comp-slug">
                  Slug *
                </label>
                <input
                  id="comp-slug"
                  type="text"
                  className="pd-input pd-input-mono"
                  placeholder="e.g. api-gateway"
                  value={formState.slug}
                  onChange={(e) =>
                    setFormState((prev) => ({ ...prev, slug: e.target.value }))
                  }
                  autoComplete="off"
                />
                {formErrors.slug ? (
                  <span className="pd-field-error">{formErrors.slug}</span>
                ) : (
                  <span className="pd-field-hint">URL-safe identifier</span>
                )}
              </div>
              <div className="pd-field">
                <label className="pd-field-label" htmlFor="comp-gcr">
                  GCR Image Path *
                </label>
                <input
                  id="comp-gcr"
                  type="text"
                  className="pd-input pd-input-mono"
                  placeholder="europe-west1-docker.pkg.dev/project/repo/image"
                  value={formState.gcr_image_path}
                  onChange={(e) =>
                    setFormState((prev) => ({
                      ...prev,
                      gcr_image_path: e.target.value,
                    }))
                  }
                  autoComplete="off"
                />
                {formErrors.gcr_image_path ? (
                  <span className="pd-field-error">{formErrors.gcr_image_path}</span>
                ) : (
                  <span className="pd-field-hint">full Artifact Registry path</span>
                )}
              </div>
            </div>
            <div className="pd-form-actions">
              <button
                className="pd-btn-ghost"
                onClick={handleCancelForm}
                disabled={formSubmitting}
              >
                Cancel
              </button>
              <button
                className="pd-btn-primary"
                onClick={handleFormSubmit}
                disabled={formSubmitting}
              >
                {formSubmitting ? 'Saving…' : 'Save Component'}
              </button>
            </div>
          </div>
        )}

        {/* Components table */}
        <div className="pd-card">
          {loadingComponents ? (
            <div className="pd-loading">
              <div className="pd-spinner" />
              <span>Loading components…</span>
            </div>
          ) : components.length === 0 ? (
            <div className="pd-empty-state" data-testid="empty-components">
              <div className="pd-empty-icon">
                <svg
                  width="18"
                  height="18"
                  viewBox="0 0 18 18"
                  fill="none"
                  stroke="currentColor"
                  strokeWidth="1.5"
                >
                  <rect x="2" y="3" width="14" height="12" rx="2" />
                  <path d="M6 3v12M12 3v12M2 7h14M2 11h14" />
                </svg>
              </div>
              <div className="pd-empty-title">No components registered</div>
              <div className="pd-empty-sub">
                Add the first component using the button above.
              </div>
            </div>
          ) : (
            <div className="pd-tbl-wrap">
              <table className="pd-comp-table">
                <thead>
                  <tr>
                    <th>Component</th>
                    <th>GCR Image Path</th>
                    <th>Added</th>
                    <th></th>
                  </tr>
                </thead>
                <tbody>
                  {components.map((comp) => (
                    <tr key={comp.id}>
                      <td className="pd-comp-name-cell">
                        <div className="pd-comp-name-str">{comp.name}</div>
                        <div className="pd-comp-slug-str">{comp.slug}</div>
                      </td>
                      <td>
                        <span className="pd-comp-path-str">{comp.gcr_image_path}</span>
                      </td>
                      <td>
                        <span className="pd-comp-date">{formatDate(comp.created_at)}</span>
                      </td>
                      <td>
                        {canWrite && (
                          <button
                            className="pd-btn-danger"
                            onClick={() => setDeleteTarget(comp)}
                            aria-label={`Delete ${comp.name}`}
                          >
                            <svg
                              width="12"
                              height="12"
                              viewBox="0 0 12 12"
                              fill="none"
                              stroke="currentColor"
                              strokeWidth="1.5"
                            >
                              <path d="M1.5 3.5h9M4 3.5V2a.5.5 0 01.5-.5h3a.5.5 0 01.5.5v1.5M5 5.5v3M7 5.5v3M2.5 3.5l.5 7a.5.5 0 00.5.5h5a.5.5 0 00.5-.5l.5-7" />
                            </svg>
                            Delete
                          </button>
                        )}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </div>
      </div>

      {/* Delete confirm dialog */}
      {deleteTarget && (
        <div
          className="pd-backdrop"
          onClick={(e) => {
            if (e.target === e.currentTarget) setDeleteTarget(null)
          }}
        >
          <div className="pd-confirm-dialog" onClick={(e) => e.stopPropagation()}>
            <div className="pd-confirm-head">
              <div className="pd-confirm-icon-wrap">
                <svg
                  width="15"
                  height="15"
                  viewBox="0 0 15 15"
                  fill="none"
                  stroke="#e05555"
                  strokeWidth="1.8"
                >
                  <path d="M2 4h11M5 4V2.5a.5.5 0 01.5-.5h4a.5.5 0 01.5.5V4M6 7v4M9 7v4M3.5 4l.5 8.5a1 1 0 001 .5h5a1 1 0 001-.5L12 4" />
                </svg>
              </div>
              <div>
                <div className="pd-confirm-title">Remove Component</div>
                <div className="pd-confirm-sub">This action cannot be undone.</div>
              </div>
            </div>
            <div className="pd-confirm-body">
              <div className="pd-confirm-target">
                <div className="pd-confirm-target-name">{deleteTarget.name}</div>
                <div className="pd-confirm-target-sub">{deleteTarget.slug}</div>
                <div className="pd-confirm-target-gcr">{deleteTarget.gcr_image_path}</div>
              </div>
              <div className="pd-confirm-warn">
                You are about to permanently remove this component. Once removed,
                it will no longer be available for deployments.
              </div>
            </div>
            <div className="pd-confirm-foot">
              <button
                className="pd-btn-ghost"
                onClick={() => setDeleteTarget(null)}
                disabled={deleteInProgress}
              >
                Cancel
              </button>
              <button
                className="pd-btn-confirm-delete"
                onClick={handleDeleteConfirm}
                disabled={deleteInProgress}
              >
                <svg
                  width="12"
                  height="12"
                  viewBox="0 0 12 12"
                  fill="none"
                  stroke="currentColor"
                  strokeWidth="1.8"
                >
                  <path d="M1.5 3.5h9M4 3.5V2a.5.5 0 01.5-.5h3a.5.5 0 01.5.5v1.5M5 5.5v3M7 5.5v3M2.5 3.5l.5 7a.5.5 0 00.5.5h6a.5.5 0 00.5-.5l.5-7" />
                </svg>
                {deleteInProgress ? 'Deleting…' : 'Delete Component'}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
