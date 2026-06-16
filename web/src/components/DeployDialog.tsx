import { useCallback, useEffect, useRef, useState } from 'react'
import { deployTag, DeployApiError, listTags, type DeployError, type Tag, type Workload } from '../api/products'
import './DeployDialog.css'

type DeployDialogProps =
  | { open: false; token: string | null; productSlug: string; environmentId: string; workload: Workload | null; onClose: () => void; onDeploySuccess?: (tag: string, commitSha: string) => void }
  | { open: true;  token: string | null; productSlug: string; environmentId: string; workload: Workload;        onClose: () => void; onDeploySuccess?: (tag: string, commitSha: string) => void }

function formatPushedAt(iso: string): string {
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return iso.slice(0, 10)
  return d.toLocaleDateString('it-IT', { day: '2-digit', month: 'short', year: 'numeric' })
}

export function DeployDialog({
  open,
  token,
  productSlug,
  environmentId,
  workload,
  onClose,
  onDeploySuccess,
}: DeployDialogProps) {
  const [tags, setTags] = useState<Tag[]>([])
  const [nextPageToken, setNextPageToken] = useState<string | undefined>(undefined)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState(false)
  const [filter, setFilter] = useState('')
  const [selectedTag, setSelectedTag] = useState<string | null>(null)
  const [manualTag, setManualTag] = useState('')
  const [deployState, setDeployState] = useState<'idle' | 'loading' | 'conflict' | 'tag_error' | 'general_error'>('idle')
  const [deployErrorData, setDeployErrorData] = useState<DeployError | null>(null)
  const debounceRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  const prevOpenRef = useRef(false)
  const filterDirtyRef = useRef(false)

  const fetchTags = useCallback(
    async (opts: { filter?: string; pageToken?: string; append?: boolean }) => {
      if (!workload || !token) return
      setLoading(true)
      try {
        const result = await listTags(token, productSlug, environmentId, workload.name, {
          filter: opts.filter || undefined,
          pageToken: opts.pageToken,
          pageSize: 20,
        })
        setTags(prev => (opts.append ? [...prev, ...result.tags] : result.tags))
        setNextPageToken(result.next_page_token)
        setError(false)
      } catch (err: unknown) {
        console.error('[DeployDialog] fetchTags failed:', err)
        setError(true)
      } finally {
        setLoading(false)
      }
    },
    [token, productSlug, environmentId, workload],
  )

  useEffect(() => {
    if (!open) {
      prevOpenRef.current = false
      return
    }
    if (prevOpenRef.current) return
    prevOpenRef.current = true
    filterDirtyRef.current = false
    setFilter('')
    setTags([])
    setNextPageToken(undefined)
    setSelectedTag(null)
    setManualTag('')
    setError(false)
    setDeployState('idle')
    setDeployErrorData(null)
    void fetchTags({})
  }, [open, fetchTags])

  useEffect(() => {
    if (!filterDirtyRef.current) return
    if (debounceRef.current !== null) clearTimeout(debounceRef.current)
    debounceRef.current = setTimeout(() => {
      setTags([])
      setNextPageToken(undefined)
      setSelectedTag(null)
      void fetchTags({ filter })
    }, 300)
    return () => {
      if (debounceRef.current !== null) clearTimeout(debounceRef.current)
    }
  }, [filter, fetchTags])

  useEffect(() => {
    if (!open) return
    const onKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose()
    }
    document.addEventListener('keydown', onKeyDown)
    return () => document.removeEventListener('keydown', onKeyDown)
  }, [open, onClose])

  const handleLoadMore = () => {
    void fetchTags({ filter, pageToken: nextPageToken, append: true })
  }

  const handleDeploy = () => {
    const tag = error ? manualTag.trim() : (selectedTag ?? '')
    if (!tag || !token || !workload) return
    setDeployState('loading')
    setDeployErrorData(null)
    deployTag(token, productSlug, environmentId, workload.name, tag)
      .then((result) => {
        setDeployState('idle')
        onDeploySuccess?.(tag, result.deployment_id)
        onClose()
      })
      .catch((err: unknown) => {
        if (err instanceof DeployApiError) {
          if (err.detail.type === 'conflict') setDeployState('conflict')
          else setDeployState('tag_error')
          setDeployErrorData(err.detail)
        } else {
          setDeployState('general_error')
        }
      })
  }

  const deployEnabled = (error ? manualTag.trim().length > 0 : selectedTag !== null) && deployState !== 'loading'

  if (!open) return null

  function renderTagListBody() {
    if (loading && tags.length === 0) {
      return (
        <div className="dd-loading">
          <div className="dd-spinner" />
          Caricamento tag…
        </div>
      )
    }
    if (error) {
      return (
        <div className="dd-empty">
          <svg width="36" height="36" viewBox="0 0 36 36" fill="none" stroke="currentColor" strokeWidth="1.2" opacity="0.28">
            <path d="M18 5L31 12V24L18 31L5 24V12L18 5Z" />
            <path d="M18 14v6" />
            <circle cx="18" cy="24" r="1" fill="currentColor" />
          </svg>
          <div className="dd-empty-title">Tag non disponibili</div>
          <div>Artifact Registry non raggiungibile. Usa il campo manuale per inserire il tag da deployare.</div>
        </div>
      )
    }
    if (tags.length === 0) {
      return (
        <div className="dd-empty">
          <svg width="32" height="32" viewBox="0 0 32 32" fill="none" stroke="currentColor" strokeWidth="1.2" opacity="0.35">
            <circle cx="14" cy="14" r="11" />
            <path d="M23 23l6 6" />
          </svg>
          <div>Nessun tag corrisponde al filtro</div>
        </div>
      )
    }
    return tags.map(tag => (
      <button
        key={tag.name}
        type="button"
        className={`dd-tag-row${selectedTag === tag.name ? ' dd-tag-row--selected' : ''}`}
        onClick={() => setSelectedTag(tag.name)}
      >
        <svg className="dd-tag-check" width="14" height="14" viewBox="0 0 14 14" fill="none" stroke="currentColor" strokeWidth="2">
          <path d="M2.5 7L5.5 10L11.5 4" />
        </svg>
        <span className="dd-tag-name">{tag.name}</span>
        <span className="dd-tag-date">{formatPushedAt(tag.pushed_at)}</span>
      </button>
    ))
  }

  return (
    <div className="dd-backdrop">
      <button
        type="button"
        className="dd-backdrop-dismiss"
        aria-label="Close dialog"
        onClick={onClose}
      />
      <dialog className="dd-modal" open aria-labelledby="dd-dialog-title">
        {/* Header */}
        <div className="dd-header">
          <div className="dd-comp-icon">
            <svg
              width="18"
              height="18"
              viewBox="0 0 18 18"
              fill="none"
              stroke="currentColor"
              strokeWidth="1.6"
            >
              <rect x="2" y="2" width="14" height="14" rx="3" />
              <path d="M2 8h14" />
              <path d="M8 2v14" />
            </svg>
          </div>
          <div className="dd-header-info">
            <div id="dd-dialog-title" className="dd-title">Seleziona tag — {workload.name}</div>
            <div className="dd-subtitle">{workload.image_repository}</div>
          </div>
          <button className="dd-close" onClick={onClose} aria-label="Chiudi" disabled={deployState === 'loading'}>
            <svg
              width="14"
              height="14"
              viewBox="0 0 14 14"
              fill="none"
              stroke="currentColor"
              strokeWidth="2"
            >
              <path d="M2 2l10 10M12 2L2 12" />
            </svg>
          </button>
        </div>

        {/* Filter */}
        <div className="dd-filter">
          <div className="dd-filter-wrap">
            <svg
              className="dd-filter-icon"
              width="13"
              height="13"
              viewBox="0 0 13 13"
              fill="none"
              stroke="currentColor"
              strokeWidth="1.6"
            >
              <circle cx="5.5" cy="5.5" r="4" />
              <path d="M9 9l2.5 2.5" />
            </svg>
            <input
              className="dd-filter-input"
              type="text"
              placeholder="Filtra per prefisso tag (es. v1.2)"
              value={filter}
              onChange={e => {
                filterDirtyRef.current = true
                setFilter(e.target.value)
              }}
              disabled={error}
            />
          </div>
        </div>

        {/* GCR error banner */}
        {error && (
          <div className="dd-warn-banner">
            <div className="dd-warn-icon">
              <svg
                width="16"
                height="16"
                viewBox="0 0 16 16"
                fill="none"
                stroke="currentColor"
                strokeWidth="1.6"
              >
                <path d="M8 2L14 13H2L8 2Z" />
                <path d="M8 7v3" />
                <circle cx="8" cy="12" r="0.5" fill="currentColor" />
              </svg>
            </div>
            <div className="dd-warn-body">
              <div className="dd-warn-msg">Impossibile caricare i tag da Artifact Registry</div>
              <div className="dd-warn-sub">
                Errore di connessione al registry. Puoi inserire il tag manualmente oppure riprovare.
              </div>
            </div>
            <button
              className="dd-warn-retry"
              onClick={() => {
                setError(false)
                setTags([])
                setNextPageToken(undefined)
                void fetchTags({ filter })
              }}
            >
              Riprova
            </button>
          </div>
        )}

        {/* 409 conflict banner */}
        {deployState === 'conflict' && deployErrorData?.type === 'conflict' && (
          <div className="dd-deploy-error-banner dd-deploy-error-banner--conflict">
            <div className="dd-deploy-error-icon">
              <svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.6">
                <path d="M8 2L14 13H2L8 2Z" />
                <path d="M8 7v3" />
                <circle cx="8" cy="12" r="0.5" fill="currentColor" />
              </svg>
            </div>
            <div className="dd-deploy-error-body">
              <div className="dd-deploy-error-msg">Deployment in corso da <span className="dd-deploy-error-detail">{deployErrorData.lock_holder}</span></div>
              <div className="dd-deploy-error-sub">
                Dal {new Date(deployErrorData.locked_since).toLocaleString('it-IT', { dateStyle: 'short', timeStyle: 'short' })}. Riprova tra qualche istante.
              </div>
            </div>
          </div>
        )}

        {/* 422 tag validation banner */}
        {deployState === 'tag_error' && deployErrorData?.type === 'tag_convention' && (
          <div className="dd-deploy-error-banner dd-deploy-error-banner--tag-error">
            <div className="dd-deploy-error-icon">
              <svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.6">
                <circle cx="8" cy="8" r="6" />
                <path d="M8 5v4" />
                <circle cx="8" cy="11.5" r="0.5" fill="currentColor" />
              </svg>
            </div>
            <div className="dd-deploy-error-body">
              <div className="dd-deploy-error-msg">Tag non valido per questo ambiente</div>
              <div className="dd-deploy-error-sub">{deployErrorData.message}</div>
              <code className="dd-deploy-error-regex">{deployErrorData.applied_regex}</code>
            </div>
          </div>
        )}

        {/* Generic error banner (network / 5xx) */}
        {deployState === 'general_error' && (
          <div className="dd-deploy-error-banner dd-deploy-error-banner--conflict">
            <div className="dd-deploy-error-icon">
              <svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.6">
                <path d="M8 2L14 13H2L8 2Z" />
                <path d="M8 7v3" />
                <circle cx="8" cy="12" r="0.5" fill="currentColor" />
              </svg>
            </div>
            <div className="dd-deploy-error-body">
              <div className="dd-deploy-error-msg">Errore durante il deploy</div>
              <div className="dd-deploy-error-sub">Impossibile completare il deploy. Riprova.</div>
            </div>
          </div>
        )}

        {/* Manual tag input (error fallback) */}
        {error && (
          <div className="dd-manual-row">
            <span className="dd-manual-label">Tag manuale:</span>
            <input
              className="dd-manual-input"
              type="text"
              placeholder="es. v1.14.1-rc.1"
              value={manualTag}
              onChange={e => setManualTag(e.target.value)}
            />
          </div>
        )}

        {/* Tag list */}
        <div className="dd-tag-list">
          {renderTagListBody()}
        </div>

        {/* Load more */}
        {!error && nextPageToken && !loading && (
          <div className="dd-load-more-row">
            <button className="dd-btn-load-more" onClick={handleLoadMore}>
              <svg
                width="13"
                height="13"
                viewBox="0 0 13 13"
                fill="none"
                stroke="currentColor"
                strokeWidth="1.6"
              >
                <path d="M6.5 2v9M3 8.5l3.5 3.5 3.5-3.5" />
              </svg>
              Carica altri tag
            </button>
          </div>
        )}

        {/* Footer */}
        <div className="dd-footer">
          <div
            className={`dd-selected-indicator${deployEnabled ? '' : ' dd-selected-indicator--empty'}`}
          >
            <svg
              width="13"
              height="13"
              viewBox="0 0 13 13"
              fill="none"
              stroke="currentColor"
              strokeWidth="1.5"
            >
              <circle cx="6.5" cy="6.5" r="5" />
              <path d="M4.5 6.5l1.5 1.5 3-3" />
            </svg>
            {deployEnabled ? (
              <>
                {error ? 'Inserito:' : 'Selezionato:'}
                <span className="dd-selected-tag-chip">
                  {error ? manualTag.trim() : selectedTag}
                </span>
              </>
            ) : (
              'Nessun tag selezionato'
            )}
          </div>
          <button className="dd-btn-deploy" disabled={!deployEnabled} onClick={handleDeploy}>
            {deployState === 'loading' ? (
              <div className="dd-btn-spinner" />
            ) : (
              <svg
                width="13"
                height="13"
                viewBox="0 0 13 13"
                fill="none"
                stroke="currentColor"
                strokeWidth="1.8"
              >
                <path d="M6.5 1L11 5.5H8.5V11H4.5V5.5H2L6.5 1Z" />
              </svg>
            )}
            {deployState === 'loading' ? 'Deploying…' : 'Deploy'}
          </button>
        </div>
      </dialog>
    </div>
  )
}
