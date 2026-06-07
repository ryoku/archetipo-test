import { useEffect, useState } from 'react'
import { useParams, useLocation, useNavigate } from 'react-router-dom'
import { useAuth } from '../auth/AuthContext'
import {
  getTagConvention,
  setTagConvention,
  type TagConvention,
  type Product,
} from '../api/products'
import './ProductDetailPage.css'
import './ProductSettingsPage.css'
import ProductHero from '../components/ProductHero'
import ProductSubNav from '../components/ProductSubNav'
import ProductNotFound from '../components/ProductNotFound'

export default function ProductSettingsPage() {
  const { slug } = useParams<{ slug: string }>()
  const location = useLocation()
  const navigate = useNavigate()
  const { accessToken } = useAuth()

  const product = location.state as Product | undefined
  const canWrite = product?.my_role === 'editor' || product?.my_role === 'admin'

  const [tagConvention, setTagConventionState] = useState<TagConvention | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  // Edit mode state
  const [editMode, setEditMode] = useState(false)
  const [editValue, setEditValue] = useState('')
  const [editError, setEditError] = useState<string | null>(null)
  const [saving, setSaving] = useState(false)

  useEffect(() => {
    if (!slug || !accessToken) return
    setLoading(true)
    setError(null)
    getTagConvention(accessToken, slug)
      .then((tc) => {
        setTagConventionState(tc)
      })
      .catch((err: unknown) => {
        setError(err instanceof Error ? err.message : 'Failed to load tag convention')
      })
      .finally(() => { setLoading(false) })
  }, [slug, accessToken])

  if (!product) {
    return <ProductNotFound />
  }

  function handleEditClick() {
    setEditValue(tagConvention?.regex ?? '')
    setEditError(null)
    setEditMode(true)
  }

  function handleCancelEdit() {
    setEditMode(false)
    setEditValue('')
    setEditError(null)
  }

  async function handleSave() {
    if (!accessToken || !slug) return
    const trimmed = editValue.trim()
    if (!trimmed) {
      setEditError('Regex is required')
      return
    }
    setSaving(true)
    setEditError(null)
    try {
      const updated = await setTagConvention(accessToken, slug, trimmed)
      setTagConventionState(updated)
      setEditMode(false)
      setEditValue('')
    } catch (err: unknown) {
      setEditError(err instanceof Error ? err.message : 'Failed to save tag convention')
    } finally {
      setSaving(false)
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
          <span>Settings</span>
        </div>

        <ProductHero product={product} />

        {/* Sub-nav: Components | Environments | Settings */}
        <ProductSubNav activeTab="settings" product={product} />
      </div>

      {/* Page body */}
      <div className="pd-page-body">
        {error && (
          <div className="pd-error" role="alert">
            <strong>Error:</strong> {error}
          </div>
        )}

        {/* Tag Convention section */}
        <div className="pd-settings-card">
          <div className="pd-settings-card-head">
            <div className="pd-settings-card-icon">
              <svg width="15" height="15" viewBox="0 0 15 15" fill="none" stroke="currentColor" strokeWidth="1.5">
                <path d="M1 3h13M1 7h13M1 11h13" />
              </svg>
            </div>
            <div className="pd-settings-card-title-group">
              <div className="pd-settings-card-title">
                Tag Convention
                {tagConvention && (
                  <span
                    className={
                      tagConvention.source === 'product'
                        ? 'pd-source-badge pd-source-badge--product'
                        : 'pd-source-badge pd-source-badge--default'
                    }
                  >
                    {tagConvention.source === 'product' ? (
                      <>
                        <svg width="10" height="10" viewBox="0 0 10 10" fill="none" stroke="currentColor" strokeWidth="1.5">
                          <path d="M2 5.5L4 7.5L8 3" />
                        </svg>
                        product override
                      </>
                    ) : (
                      <>
                        <svg width="10" height="10" viewBox="0 0 10 10" fill="none" stroke="currentColor" strokeWidth="1.5">
                          <circle cx="5" cy="5" r="3.5" />
                        </svg>
                        global default
                      </>
                    )}
                  </span>
                )}
              </div>
              <div className="pd-settings-card-sub">
                Regex pattern used to validate image tags before production deployments.
              </div>
            </div>
          </div>

          <div className="pd-settings-card-body">
            {loading ? (
              <div className="pd-loading">
                <div className="pd-spinner" />
                <span>Loading tag convention…</span>
              </div>
            ) : tagConvention && !editMode ? (
              /* View mode */
              <div className="pd-regex-display">
                <span className="pd-regex-value">{tagConvention.regex}</span>
                {canWrite && (
                  <button
                    type="button"
                    className="pd-btn-ghost pd-btn-sm"
                    onClick={handleEditClick}
                  >
                    <svg width="12" height="12" viewBox="0 0 12 12" fill="none" stroke="currentColor" strokeWidth="1.5">
                      <path d="M8.5 1.5L10.5 3.5L4 10H2v-2L8.5 1.5Z" />
                    </svg>
                    Edit
                  </button>
                )}
              </div>
            ) : editMode ? (
              /* Edit mode */
              <div className="pd-regex-edit">
                <div className="pd-field">
                  <label className="pd-field-label" htmlFor="tag-convention-regex">
                    Regex pattern
                  </label>
                  <input
                    id="tag-convention-regex"
                    type="text"
                    className="pd-input pd-input-mono"
                    value={editValue}
                    onChange={(e) => setEditValue(e.target.value)}
                    autoComplete="off"
                    autoFocus
                    disabled={saving}
                  />
                  {editError ? (
                    <span className="pd-field-error">{editError}</span>
                  ) : (
                    <span className="pd-field-hint">e.g. ^v\d+\.\d+\.\d+$</span>
                  )}
                </div>
                <div className="pd-form-actions">
                  <button
                    type="button"
                    className="pd-btn-ghost"
                    onClick={handleCancelEdit}
                    disabled={saving}
                  >
                    Cancel
                  </button>
                  <button
                    type="button"
                    className="pd-btn-primary"
                    onClick={handleSave}
                    disabled={saving}
                  >
                    {saving ? 'Saving…' : 'Save'}
                  </button>
                </div>
              </div>
            ) : null}
          </div>
        </div>
      </div>
    </div>
  )
}
