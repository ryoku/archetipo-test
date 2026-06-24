// Requires ProductDetailPage.css to be imported by the page consumer.
import { useNavigate } from 'react-router-dom'
import type { Product } from '../api/products'

type Tab = 'workloads' | 'environments' | 'status' | 'history' | 'settings'

interface Props {
  readonly activeTab: Tab
  readonly product: Product
}

interface TabDef {
  readonly id: Tab
  readonly label: string
  readonly path: (slug: string) => string
  readonly icon: React.ReactNode
}

const TABS: TabDef[] = [
  {
    id: 'workloads',
    label: 'Workloads',
    path: (slug) => `/products/${slug}`,
    icon: (
      <svg width="14" height="14" viewBox="0 0 14 14" fill="none" stroke="currentColor" strokeWidth="1.5">
        <rect x="1" y="2" width="12" height="10" rx="2"/>
        <path d="M5 2v10M9 2v10M1 6h12M1 9h12"/>
      </svg>
    ),
  },
  {
    id: 'environments',
    label: 'Environments',
    path: (slug) => `/products/${slug}/environments`,
    icon: (
      <svg width="14" height="14" viewBox="0 0 14 14" fill="none" stroke="currentColor" strokeWidth="1.5">
        <path d="M2 3h10a1 1 0 011 1v6a1 1 0 01-1 1H2a1 1 0 01-1-1V4a1 1 0 011-1z"/>
        <path d="M4 3V2M10 3V2M1 7h12"/>
      </svg>
    ),
  },
  {
    id: 'status',
    label: 'Status',
    path: (slug) => `/products/${slug}/status`,
    icon: (
      <svg width="14" height="14" viewBox="0 0 14 14" fill="none" stroke="currentColor" strokeWidth="1.5">
        <circle cx="7" cy="7" r="5"/>
        <path d="M7 4v3l2 2"/>
      </svg>
    ),
  },
  {
    id: 'history',
    label: 'History',
    path: (slug) => `/products/${slug}/history`,
    icon: (
      <svg width="14" height="14" viewBox="0 0 14 14" fill="none" stroke="currentColor" strokeWidth="1.5">
        <circle cx="7" cy="7" r="5.5"/>
        <path d="M7 4v3.5l2.5 1.5"/>
      </svg>
    ),
  },
  {
    id: 'settings',
    label: 'Settings',
    path: (slug) => `/products/${slug}/settings`,
    icon: (
      <svg width="14" height="14" viewBox="0 0 14 14" fill="none" stroke="currentColor" strokeWidth="1.5">
        <circle cx="7" cy="7" r="2"/>
        <path d="M7 1v1.5M7 11.5V13M1 7h1.5M11.5 7H13M2.93 2.93l1.06 1.06M10.01 10.01l1.06 1.06M2.93 11.07l1.06-1.06M10.01 3.99l1.06-1.06"/>
      </svg>
    ),
  },
]

export default function ProductSubNav({ activeTab, product }: Props) {
  const navigate = useNavigate()
  return (
    <nav className="pd-subnav">
      {TABS.map((tab) => (
        <button
          key={tab.id}
          type="button"
          className={`pd-subnav-link${activeTab === tab.id ? ' pd-subnav-link--active' : ''}`}
          onClick={activeTab === tab.id ? undefined : () => navigate(tab.path(product.slug), { state: product })}
        >
          {tab.icon}
          {tab.label}
        </button>
      ))}
    </nav>
  )
}
