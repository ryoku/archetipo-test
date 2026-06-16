import { useEffect, useRef, useState } from 'react'
import { useParams, useLocation, useNavigate } from 'react-router-dom'
import { useAuth } from '../auth/AuthContext'
import {
  listEnvironments,
  listWorkloads,
  type Environment,
  type Workload,
  type Product,
} from '../api/products'
import './ProductDetailPage.css'
import ProductHero from '../components/ProductHero'
import ProductSubNav from '../components/ProductSubNav'
import ProductNotFound from '../components/ProductNotFound'
import { DeployDialog } from '../components/DeployDialog'

interface WorkloadsContentProps {
  readonly loading: boolean
  readonly workloads: Workload[]
  readonly notFound: boolean
  readonly noEnvironments: boolean
  readonly canWrite: boolean
  readonly onDeploy: (workload: Workload) => void
}

function WorkloadsContent({ loading, workloads, notFound, noEnvironments, canWrite, onDeploy }: WorkloadsContentProps) {
  if (loading) {
    return (
      <div className="pd-loading">
        <div className="pd-spinner" />
        <span>Loading workloads…</span>
      </div>
    )
  }
  if (noEnvironments) {
    return (
      <div className="pd-empty-state">
        <div className="pd-empty-title">No environments configured</div>
        <div className="pd-empty-sub">Add an environment first to discover workloads.</div>
      </div>
    )
  }
  if (notFound) {
    return (
      <div className="pd-empty-state" data-testid="workloads-not-found">
        <div className="pd-empty-title">HelmRelease not configured</div>
        <div className="pd-empty-sub">No HelmRelease found for this environment in the gitops repo.</div>
      </div>
    )
  }
  if (workloads.length === 0) {
    return (
      <div className="pd-empty-state" data-testid="empty-workloads">
        <div className="pd-empty-title">No workloads discovered</div>
        <div className="pd-empty-sub">No workloads with an image.repository field were found in the HelmRelease.</div>
      </div>
    )
  }
  return (
    <div className="pd-tbl-wrap">
      <table className="pd-comp-table">
        <thead>
          <tr>
            <th>Workload</th>
            <th>Image Repository</th>
            <th></th>
          </tr>
        </thead>
        <tbody>
          {workloads.map((wl) => (
            <tr key={wl.name}>
              <td className="pd-comp-name-cell">
                <div className="pd-comp-name-str">{wl.name}</div>
              </td>
              <td><span className="pd-comp-path-str">{wl.image_repository}</span></td>
              <td>
                <div className="pd-row-actions">
                  {canWrite && (
                    <button
                      className="pd-btn-deploy"
                      onClick={() => onDeploy(wl)}
                      aria-label={`Deploy ${wl.name}`}
                    >
                      <svg width="12" height="12" viewBox="0 0 12 12" fill="none" stroke="currentColor" strokeWidth="1.8">
                        <path d="M6 1L10 5H7.5V10H4.5V5H2L6 1Z" />
                      </svg>
                      Deploy
                    </button>
                  )}
                </div>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}

export default function ProductDetailPage() {
  const { slug } = useParams<{ slug: string }>()
  const location = useLocation()
  const navigate = useNavigate()
  const { accessToken } = useAuth()

  const product = location.state as Product | undefined
  const canWrite = product?.my_role === 'editor' || product?.my_role === 'admin'

  const [environments, setEnvironments] = useState<Environment[]>([])
  const [loadingEnvironments, setLoadingEnvironments] = useState(true)
  const [selectedEnvId, setSelectedEnvId] = useState<string | null>(null)
  const [workloads, setWorkloads] = useState<Workload[]>([])
  const [loadingWorkloads, setLoadingWorkloads] = useState(false)
  const [workloadsError, setWorkloadsError] = useState<string | null>(null)
  const [workloadsNotFound, setWorkloadsNotFound] = useState(false)
  const [selectedWorkload, setSelectedWorkload] = useState<Workload | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [deploySuccess, setDeploySuccess] = useState<{ tag: string; sha: string } | null>(null)
  const toastTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)

  useEffect(() => () => { if (toastTimerRef.current !== null) clearTimeout(toastTimerRef.current) }, [])

  useEffect(() => {
    if (!slug || !accessToken) return
    listEnvironments(accessToken, slug)
      .then((envs) => {
        const firstId = envs[0]?.id ?? null
        setEnvironments(envs)
        setSelectedEnvId(firstId)
        if (firstId) setLoadingWorkloads(true)
      })
      .catch((err: unknown) => {
        console.error('[ProductDetailPage] listEnvironments failed for slug=%s:', slug, err)
        setError(err instanceof Error ? err.message : 'Failed to load environments')
      })
      .finally(() => { setLoadingEnvironments(false) })
  }, [slug, accessToken])

  useEffect(() => {
    if (!selectedEnvId || !slug || !accessToken) return
    listWorkloads(accessToken, slug, selectedEnvId)
      .then((wls) => {
        setWorkloads(wls)
        setWorkloadsNotFound(false)
        setWorkloadsError(null)
      })
      .catch((err: unknown) => {
        console.error('[ProductDetailPage] listWorkloads failed for slug=%s env=%s:', slug, selectedEnvId, err)
        const msg = err instanceof Error ? err.message : 'Failed to load workloads'
        if (msg === 'listWorkloads: 404') {
          setWorkloadsNotFound(true)
          setWorkloads([])
        } else {
          setWorkloadsError(msg)
        }
      })
      .finally(() => { setLoadingWorkloads(false) })
  }, [selectedEnvId, slug, accessToken])

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

        <ProductSubNav activeTab="workloads" product={product} />
      </div>

      <div className="pd-page-body">
        {(error ?? workloadsError) && (
          <div className="pd-error" role="alert">
            <strong>Error:</strong> {error ?? workloadsError}
          </div>
        )}

        {environments.length > 0 && (
          <div className="pd-section-head">
            <div>
              <div className="pd-section-label">Workloads</div>
              <div className="pd-section-sub">Discovered from the HelmRelease in the gitops repo.</div>
            </div>
            <select
              value={selectedEnvId ?? ''}
              onChange={(e) => {
                setSelectedEnvId(e.target.value)
                setLoadingWorkloads(true)
                setWorkloads([])
                setWorkloadsNotFound(false)
                setWorkloadsError(null)
                setError(null)
              }}
              className="pd-input"
            >
              {environments.map((env) => (
                <option key={env.id} value={env.id}>{env.name}</option>
              ))}
            </select>
          </div>
        )}

        {!error && (
          <div className="pd-card">
            <WorkloadsContent
              loading={loadingWorkloads || loadingEnvironments}
              workloads={workloads}
              notFound={workloadsNotFound}
              noEnvironments={environments.length === 0 && !loadingEnvironments}
              canWrite={canWrite}
              onDeploy={(workload) => { setSelectedWorkload(workload) }}
            />
          </div>
        )}
      </div>

      {selectedWorkload && selectedEnvId && (
        <DeployDialog
          open={true}
          token={accessToken}
          productSlug={slug ?? ''}
          workload={selectedWorkload}
          environmentId={selectedEnvId}
          onClose={() => setSelectedWorkload(null)}
          onDeploySuccess={(tag, sha) => {
            if (toastTimerRef.current !== null) clearTimeout(toastTimerRef.current)
            setDeploySuccess({ tag, sha })
            toastTimerRef.current = setTimeout(() => setDeploySuccess(null), 6000)
          }}
        />
      )}

      {deploySuccess && (
        <div className="pd-toast" role="status" aria-live="polite" data-testid="deploy-toast">
          <svg width="14" height="14" viewBox="0 0 14 14" fill="none" stroke="currentColor" strokeWidth="2">
            <path d="M2.5 7L5.5 10L11.5 4" />
          </svg>
          <div className="pd-toast-body">
            <div className="pd-toast-title">Deployed <span className="pd-toast-tag">{deploySuccess.tag}</span></div>
            <div className="pd-toast-sub">commit {deploySuccess.sha}</div>
          </div>
          <button
            className="pd-toast-close"
            aria-label="Chiudi notifica"
            onClick={() => setDeploySuccess(null)}
          >
            <svg width="10" height="10" viewBox="0 0 10 10" fill="none" stroke="currentColor" strokeWidth="2">
              <path d="M1 1l8 8M9 1L1 9" />
            </svg>
          </button>
        </div>
      )}
    </div>
  )
}
