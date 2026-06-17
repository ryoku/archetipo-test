import type { Environment, ProductStatus } from '../api/products'
import './StatusMatrix.css'

interface Props {
  readonly status: ProductStatus | null
  readonly environments: Environment[]
  readonly loading: boolean
  readonly error: string | null
  readonly onRefresh: () => void
}

function envColorClass(type: Environment['type']): string {
  switch (type) {
    case 'production': return 'env-prod'
    case 'integration': return 'env-intg'
    default: return 'env-dev'
  }
}

function tagCellClass(tag: string, envType: Environment['type']): string {
  if (tag === 'N/A') return 'sm-tag-cell sm-tag-na'
  switch (envType) {
    case 'production': return 'sm-tag-cell sm-tag-prod'
    case 'integration': return 'sm-tag-cell sm-tag-intg'
    default: return 'sm-tag-cell sm-tag-dev'
  }
}

export default function StatusMatrix({ status, environments, loading, error, onRefresh }: Props) {
  if (loading) {
    return (
      <div className="sm-loading" data-testid="status-loading">
        <div className="pd-spinner" />
        <span>Loading deployment status…</span>
      </div>
    )
  }

  if (error) {
    return (
      <div className="sm-error" data-testid="status-error">
        <strong>Error:</strong> {error}
        <button className="pd-btn-deploy" onClick={onRefresh} style={{ marginLeft: 12 }}>Retry</button>
      </div>
    )
  }

  if (!status) return null

  const workloadNames = Object.keys(status.workloads).sort()

  if (workloadNames.length === 0) {
    return (
      <div className="pd-empty-state" data-testid="status-empty">
        <div className="pd-empty-title">No deployment data</div>
        <div className="pd-empty-sub">No HelmReleases found for any environment in this product.</div>
      </div>
    )
  }

  return (
    <div data-testid="status-matrix">
      {status.stale && (
        <div className="sm-stale-banner" data-testid="stale-banner">
          <svg width="14" height="14" viewBox="0 0 14 14" fill="none" stroke="currentColor" strokeWidth="1.5">
            <circle cx="7" cy="7" r="6" />
            <path d="M7 4v3.5l2 1.5" strokeLinecap="round" />
          </svg>
          <span>
            <strong>Data may be stale.</strong> The cache has expired — click Refresh to reload.
          </span>
        </div>
      )}

      <div className="sm-matrix-wrap">
        <table className="sm-matrix-table">
          <thead>
            <tr>
              <th className="sm-col-workload">Workload</th>
              {environments.map((env, i) => (
                <th key={env.id} className="sm-col-env">
                  <div className={`sm-env-col-header ${envColorClass(env.type)}`} style={i === environments.length - 1 ? { borderRight: 'none' } : undefined}>
                    <svg width="10" height="10" viewBox="0 0 10 10" fill="none">
                      <circle cx="5" cy="5" r="4" stroke="currentColor" strokeWidth="1.2" />
                    </svg>
                    <span className="sm-env-col-name">{env.name}</span>
                    <span className="sm-env-col-slug">{env.slug}</span>
                  </div>
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {workloadNames.map((workload) => (
              <tr key={workload}>
                <td className="sm-td-workload">
                  <div className="sm-wl-name">{workload}</div>
                </td>
                {environments.map((env) => {
                  const tag = status.workloads[workload]?.[env.slug] ?? 'N/A'
                  return (
                    <td key={env.id} className="sm-td-cell">
                      <span className={tagCellClass(tag, env.type)}>{tag}</span>
                    </td>
                  )
                })}
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      <div className="sm-footer-note">
        <svg width="11" height="11" viewBox="0 0 12 12" fill="none" stroke="currentColor" strokeWidth="1.4">
          <circle cx="6" cy="6" r="5" /><path d="M6 5v3M6 3.5v.5" />
        </svg>
        <span>
          <strong>N/A</strong> = HelmRelease not found or <code>image.tag</code> not set.
          Data refreshed every 60 s (configurable TTL).
        </span>
      </div>
    </div>
  )
}
