import { useEffect, useRef, useState } from 'react'
import { useParams, useLocation, useNavigate } from 'react-router-dom'
import { useAuth } from '../auth/AuthContext'
import {
  listEnvironments,
  createEnvironment,
  deleteEnvironment,
  type Environment,
  type Product,
} from '../api/products'
import './ProductDetailPage.css'
import './EnvironmentsPage.css'
import ProductHero from '../components/ProductHero'
import ProductSubNav from '../components/ProductSubNav'
import ProductNotFound from '../components/ProductNotFound'
import ConfirmDeleteFooter from '../components/ConfirmDeleteFooter'

interface EnvFormState { name: string; type: Environment['type'] | ''; slug: string }
interface EnvFormErrors { name?: string; type?: string; slug?: string }

function toEnvSlug(name: string): string {
  return name.toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-+|-+$/g, '')
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

interface EnvironmentsContentProps {
  readonly loading: boolean
  readonly environments: Environment[]
  readonly canAdmin: boolean
  readonly onDelete: (env: Environment) => void
}

function EnvironmentsContent({ loading, environments, canAdmin, onDelete }: EnvironmentsContentProps) {
  if (loading) {
    return (
      <div className="pd-loading">
        <div className="pd-spinner" />
        <span>Loading environments…</span>
      </div>
    )
  }
  if (environments.length === 0) {
    return (
      <div className="pd-empty-state" data-testid="empty-environments">
        <div className="pd-empty-icon">
          <svg width="18" height="18" viewBox="0 0 18 18" fill="none" stroke="currentColor" strokeWidth="1.5">
            <path d="M2 4h14a1 1 0 011 1v8a1 1 0 01-1 1H2a1 1 0 01-1-1V5a1 1 0 011-1z"/>
            <path d="M4 4V3M14 4V3M1 9h16"/>
          </svg>
        </div>
        <div className="pd-empty-title">No environments configured</div>
        <div className="pd-empty-sub">
          {canAdmin
            ? 'Add the first environment using the button above.'
            : 'Contact a DevOps Admin to add environments.'}
        </div>
      </div>
    )
  }
  return (
    <div className="pd-tbl-wrap">
      <table className="pd-comp-table">
        <thead>
          <tr>
            <th>Environment</th>
            <th>Type</th>
            <th>GitOps Path</th>
            <th>Created</th>
            {canAdmin && <th></th>}
          </tr>
        </thead>
        <tbody>
          {environments.map(env => (
            <tr key={env.id}>
              <td className="pd-comp-name-cell">
                <div className="pd-comp-name-str">{env.name}</div>
              </td>
              <td>
                <span className={`env-type-badge env-type-${env.type}`}>{env.type}</span>
              </td>
              <td><span className="env-path-str">{env.gitops_path}</span></td>
              <td><span className="pd-comp-date">{formatDate(env.created_at)}</span></td>
              {canAdmin && (
                <td>
                  <button
                    className="pd-btn-danger"
                    onClick={() => onDelete(env)}
                    aria-label={`Delete ${env.name}`}
                  >
                    <svg width="12" height="12" viewBox="0 0 12 12" fill="none" stroke="currentColor" strokeWidth="1.5">
                      <path d="M1.5 3.5h9M4 3.5V2a.5.5 0 01.5-.5h3a.5.5 0 01.5.5v1.5M5 5.5v3M7 5.5v3M2.5 3.5l.5 7a.5.5 0 00.5.5h6a.5.5 0 00.5-.5l.5-7"/>
                    </svg>
                    Delete
                  </button>
                </td>
              )}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}

export default function EnvironmentsPage() {
  const { slug } = useParams<{ slug: string }>()
  const location = useLocation()
  const navigate = useNavigate()
  const { accessToken } = useAuth()

  const product = location.state as Product | undefined
  const canAdmin = product?.my_role === 'admin'

  const [environments, setEnvironments] = useState<Environment[]>([])
  const [loadingEnvironments, setLoadingEnvironments] = useState(true)
  const [error, setError] = useState<string | null>(null)

  // Add form state
  const [showForm, setShowForm] = useState(false)
  const [formState, setFormState] = useState<EnvFormState>({ name: '', type: '', slug: '' })
  const slugTouched = useRef(false)
  const [formErrors, setFormErrors] = useState<EnvFormErrors>({})
  const [formSubmitting, setFormSubmitting] = useState(false)

  // Delete confirm dialog state
  const [deleteTarget, setDeleteTarget] = useState<Environment | null>(null)
  const [deleteInProgress, setDeleteInProgress] = useState(false)

  useEffect(() => {
    if (!slug || !accessToken) return
    // Intentional reset: slug changed, clear stale list and show loading before new fetch.
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setLoadingEnvironments(true)
    setEnvironments([])
    listEnvironments(accessToken, slug)
      .then(setEnvironments)
      .catch((err: unknown) => {
        setError(err instanceof Error ? err.message : 'Failed to load environments')
      })
      .finally(() => { setLoadingEnvironments(false) })
  }, [slug, accessToken])

  // Window-level Escape handler: closes the delete dialog regardless of focus state.
  useEffect(() => {
    if (!deleteTarget) return
    const handleEscape = (e: KeyboardEvent) => {
      if (e.key === 'Escape') setDeleteTarget(null)
    }
    globalThis.addEventListener('keydown', handleEscape)
    return () => { globalThis.removeEventListener('keydown', handleEscape) }
  }, [deleteTarget])

  if (!product) {
    return <ProductNotFound />
  }

  function validateForm(): boolean {
    const errs: EnvFormErrors = {}
    if (!formState.name.trim()) errs.name = 'Name is required'
    if (!formState.type) errs.type = 'Type is required'
    if (!formState.slug.trim()) {
      errs.slug = 'Slug is required'
    } else if (!/^[a-z0-9]+(-[a-z0-9]+)*$/.test(formState.slug)) {
      errs.slug = 'Slug must be lowercase alphanumeric and hyphens only'
    }
    setFormErrors(errs)
    return Object.keys(errs).length === 0
  }

  async function handleFormSubmit() {
    if (!validateForm() || !accessToken || !slug) return
    setFormSubmitting(true)
    setError(null)
    try {
      const newEnv = await createEnvironment(accessToken, slug, {
        name: formState.name.trim(),
        type: formState.type as Environment['type'],
        slug: formState.slug.trim(),
      })
      setEnvironments((prev) => [...prev, newEnv])
      setShowForm(false)
      setFormState({ name: '', type: '', slug: '' })
      slugTouched.current = false
      setFormErrors({})
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Failed to create environment')
    } finally {
      setFormSubmitting(false)
    }
  }

  function handleCancelForm() {
    setShowForm(false)
    setFormState({ name: '', type: '', slug: '' })
    slugTouched.current = false
    setFormErrors({})
  }

  async function handleDeleteConfirm() {
    if (!deleteTarget || !accessToken || !slug) return
    setDeleteInProgress(true)
    setError(null)
    try {
      await deleteEnvironment(accessToken, slug, deleteTarget.id)
      setEnvironments((prev) => prev.filter((e) => e.id !== deleteTarget.id))
      setDeleteTarget(null)
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Failed to delete environment')
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
          <button className="pd-bc-link" onClick={() => navigate('/')}>Products</button>
          <span className="pd-bc-sep">/</span>
          <button className="pd-bc-link" onClick={() => navigate(`/products/${slug}`, { state: product })}>{slug}</button>
          <span className="pd-bc-sep">/</span>
          <span>Environments</span>
        </div>

        <ProductHero product={product} />

        {/* Sub-nav: Components | Environments */}
        <ProductSubNav activeTab="environments" product={product} />
      </div>

      {/* Page body */}
      <div className="pd-page-body">
        {error && (
          <div className="pd-error" role="alert">
            <strong>Error:</strong> {error}
          </div>
        )}

        {/* Environments section header */}
        <div className="pd-section-head">
          <div>
            <div className="pd-section-label">Deployment Environments</div>
            <div className="pd-section-sub">
              Each environment defines a deployment target. The GitOps path is derived automatically from the environment and product slugs.
            </div>
          </div>
          {!showForm && canAdmin && (
            <button
              type="button"
              className="pd-btn-primary"
              onClick={() => setShowForm(true)}
            >
              <svg width="12" height="12" viewBox="0 0 12 12" fill="none" stroke="currentColor" strokeWidth="2">
                <path d="M6 2v8M2 6h8" />
              </svg>
              Add Environment
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
              New Environment
            </div>
            <div className="pd-form-row">
              <div className="pd-field">
                <label className="pd-field-label" htmlFor="env-name">
                  Name *
                </label>
                <input
                  id="env-name"
                  type="text"
                  className="pd-input"
                  placeholder="e.g. staging"
                  value={formState.name}
                  onChange={(e) => {
                    const name = e.target.value
                    setFormState((prev) => ({
                      ...prev,
                      name,
                      slug: slugTouched.current ? prev.slug : toEnvSlug(name),
                    }))
                  }}
                  autoComplete="off"
                />
                {formErrors.name ? (
                  <span className="pd-field-error">{formErrors.name}</span>
                ) : (
                  <span className="pd-field-hint">display name</span>
                )}
              </div>
              <div className="pd-field">
                <label className="pd-field-label" htmlFor="env-type">
                  Type *
                </label>
                <select
                  id="env-type"
                  className="pd-input"
                  value={formState.type}
                  onChange={(e) => setFormState((prev) => ({ ...prev, type: e.target.value as Environment['type'] | '' }))}
                >
                  <option value="">Select type…</option>
                  <option value="dev">dev</option>
                  <option value="integration">integration</option>
                  <option value="production">production</option>
                </select>
                {formErrors.type ? (
                  <span className="pd-field-error">{formErrors.type}</span>
                ) : (
                  <span className="pd-field-hint">dev / integration / production</span>
                )}
              </div>
              <div className="pd-field">
                <label className="pd-field-label" htmlFor="env-slug">
                  Slug *
                </label>
                <input
                  id="env-slug"
                  type="text"
                  className="pd-input pd-input-mono"
                  placeholder="e.g. staging"
                  value={formState.slug}
                  onChange={(e) => {
                    slugTouched.current = true
                    setFormState((prev) => ({ ...prev, slug: e.target.value }))
                  }}
                  autoComplete="off"
                />
                {formErrors.slug ? (
                  <span className="pd-field-error">{formErrors.slug}</span>
                ) : (
                  <span className="pd-field-hint">lowercase alphanumeric and hyphens</span>
                )}
              </div>
            </div>
            {formState.slug && (
              <div className="pd-field pd-field-preview">
                <span className="pd-field-label">GitOps Path</span>
                <span className="env-path-str" data-testid="gitops-path-preview">
                  apps/{formState.slug}/{slug}/{slug}-helmrelease.yaml
                </span>
              </div>
            )}
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
                onClick={handleFormSubmit}
                disabled={formSubmitting}
              >
                {formSubmitting ? 'Saving…' : 'Save Environment'}
              </button>
            </div>
          </div>
        )}

        {/* Environments table card */}
        <div className="pd-card">
          <EnvironmentsContent
            loading={loadingEnvironments}
            environments={environments}
            canAdmin={canAdmin}
            onDelete={(env) => setDeleteTarget(env)}
          />
        </div>
      </div>

      {/* Delete confirm dialog */}
      {deleteTarget && (
        <div className="pd-backdrop">
          <button
            type="button"
            className="pd-backdrop-dismiss"
            aria-label="Close dialog"
            onClick={() => setDeleteTarget(null)}
          />
          <dialog
            className="pd-confirm-dialog"
            open
            aria-label="Remove Environment"
          >
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
                <div className="pd-confirm-title">Remove Environment</div>
                <div className="pd-confirm-sub">This action cannot be undone.</div>
              </div>
            </div>
            <div className="pd-confirm-body">
              <div className="pd-confirm-target">
                <div className="pd-confirm-target-name">{deleteTarget.name}</div>
                <div className="pd-confirm-target-sub">
                  <span className={`env-type-badge env-type-${deleteTarget.type}`}>{deleteTarget.type}</span>
                </div>
                <div className="env-path-str">{deleteTarget.gitops_path}</div>
              </div>
              <div className="pd-confirm-warn">
                You are about to permanently remove this environment.
              </div>
            </div>
            <ConfirmDeleteFooter
              label="Delete Environment"
              inProgress={deleteInProgress}
              onCancel={() => setDeleteTarget(null)}
              onConfirm={handleDeleteConfirm}
            />
          </dialog>
        </div>
      )}
    </div>
  )
}
