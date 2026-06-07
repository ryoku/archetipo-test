// Requires ProductDetailPage.css to be imported by the page consumer.
import type { Product } from '../api/products'

function getInitials(name: string): string {
  return name
    .split(/\s+/)
    .map((w) => w[0])
    .join('')
    .toUpperCase()
    .slice(0, 2)
}

export default function ProductHero({ product }: { readonly product: Product }) {
  return (
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
  )
}
