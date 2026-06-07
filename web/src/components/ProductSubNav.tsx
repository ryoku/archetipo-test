// Requires ProductDetailPage.css to be imported by the page consumer.
import { useNavigate } from 'react-router-dom'
import type { Product } from '../api/products'

interface Props {
  readonly activeTab: 'components' | 'environments'
  readonly product: Product
}

export default function ProductSubNav({ activeTab, product }: Props) {
  const navigate = useNavigate()
  return (
    <nav className="pd-subnav">
      <button
        type="button"
        className={`pd-subnav-link${activeTab === 'components' ? ' pd-subnav-link--active' : ''}`}
        onClick={activeTab === 'environments' ? () => navigate(`/products/${product.slug}`, { state: product }) : undefined}
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
        onClick={activeTab === 'components' ? () => navigate(`/products/${product.slug}/environments`, { state: product }) : undefined}
      >
        <svg width="14" height="14" viewBox="0 0 14 14" fill="none" stroke="currentColor" strokeWidth="1.5">
          <path d="M2 3h10a1 1 0 011 1v6a1 1 0 01-1 1H2a1 1 0 01-1-1V4a1 1 0 011-1z"/>
          <path d="M4 3V2M10 3V2M1 7h12"/>
        </svg>
        Environments
      </button>
    </nav>
  )
}
