// Requires ProductDetailPage.css to be imported by the page consumer.
import { useNavigate } from 'react-router-dom'

export default function ProductNotFound() {
  const navigate = useNavigate()
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
