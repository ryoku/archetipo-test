// Requires ProductDetailPage.css to be imported by the page consumer.
import { useNavigate } from 'react-router-dom'
import type { Product } from '../api/products'

interface Props {
  readonly activeTab: 'components' | 'environments' | 'settings'
  readonly product: Product
}

export default function ProductSubNav({ activeTab, product }: Props) {
  const navigate = useNavigate()
  return (
    <nav className="pd-subnav">
      <button
        type="button"
        className={`pd-subnav-link${activeTab === 'components' ? ' pd-subnav-link--active' : ''}`}
        onClick={activeTab === 'components' ? undefined : () => navigate(`/products/${product.slug}`, { state: product })}
      >
        <svg width="14" height="14" viewBox="0 0 14 14" fill="none" stroke="currentColor" strokeWidth="1.5">
          <rect x="1" y="2" width="12" height="10" rx="2"/>
          <path d="M5 2v10M9 2v10M1 6h12M1 9h12"/>
        </svg>
        Components
      </button>
      <button
        type="button"
        className={`pd-subnav-link${activeTab === 'environments' ? ' pd-subnav-link--active' : ''}`}
        onClick={activeTab === 'environments' ? undefined : () => navigate(`/products/${product.slug}/environments`, { state: product })}
      >
        <svg width="14" height="14" viewBox="0 0 14 14" fill="none" stroke="currentColor" strokeWidth="1.5">
          <path d="M2 3h10a1 1 0 011 1v6a1 1 0 01-1 1H2a1 1 0 01-1-1V4a1 1 0 011-1z"/>
          <path d="M4 3V2M10 3V2M1 7h12"/>
        </svg>
        Environments
      </button>
      <button
        type="button"
        className={`pd-subnav-link${activeTab === 'settings' ? ' pd-subnav-link--active' : ''}`}
        onClick={activeTab === 'settings' ? undefined : () => navigate(`/products/${product.slug}/settings`, { state: product })}
      >
        <svg width="14" height="14" viewBox="0 0 14 14" fill="none" stroke="currentColor" strokeWidth="1.5">
          <circle cx="7" cy="7" r="2"/>
          <path d="M7 1v1.5M7 11.5V13M1 7h1.5M11.5 7H13M2.93 2.93l1.06 1.06M10.01 10.01l1.06 1.06M2.93 11.07l1.06-1.06M10.01 3.99l1.06-1.06"/>
        </svg>
        Settings
      </button>
    </nav>
  )
}
