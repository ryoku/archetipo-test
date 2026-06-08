import { BrowserRouter, Route, Routes } from 'react-router-dom'
import { AuthProvider } from './auth/AuthContext'
import { ProtectedRoute } from './auth/ProtectedRoute'
import HomePage from './pages/HomePage'
import LoginPage from './pages/LoginPage'
import OidcCallbackPage from './pages/OidcCallbackPage'
import ProductDetailPage from './pages/ProductDetailPage'
import EnvironmentsPage from './pages/EnvironmentsPage'
import ProductSettingsPage from './pages/ProductSettingsPage'

export default function App() {
  return (
    <BrowserRouter>
      <AuthProvider>
        <Routes>
          <Route
            path="/"
            element={
              <ProtectedRoute>
                <HomePage />
              </ProtectedRoute>
            }
          />
          <Route path="/login" element={<LoginPage />} />
          <Route path="/auth/callback" element={<OidcCallbackPage />} />
          <Route
            path="/products/:slug"
            element={
              <ProtectedRoute>
                <ProductDetailPage />
              </ProtectedRoute>
            }
          />
          <Route
            path="/products/:slug/environments"
            element={
              <ProtectedRoute>
                <EnvironmentsPage />
              </ProtectedRoute>
            }
          />
          <Route
            path="/products/:slug/settings"
            element={
              <ProtectedRoute>
                <ProductSettingsPage />
              </ProtectedRoute>
            }
          />
        </Routes>
      </AuthProvider>
    </BrowserRouter>
  )
}
