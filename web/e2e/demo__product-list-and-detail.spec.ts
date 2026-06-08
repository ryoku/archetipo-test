import { test, expect } from '@playwright/test'

// In the dist build (no .env file), VITE_OIDC_ISSUER_URL and VITE_OIDC_CLIENT_ID
// are undefined, so oidc-client-ts constructs the sessionStorage key as:
// oidc.user:undefined:undefined
const SESSION_KEY = 'oidc.user:undefined:undefined'

const fakeUser = {
  profile: { sub: 'test-user-1', name: 'Marco Rossi', email: 'marco@example.com' },
  session_state: null,
  access_token: 'fake-access-token',
  refresh_token: null,
  token_type: 'Bearer',
  scope: 'openid profile email',
  expires_at: 9999999999,
  id_token: 'fake-id-token',
}

const mockProducts = [
  {
    id: 'id-platform-api',
    name: 'Platform API',
    slug: 'platform-api',
    description: 'Core platform services',
    created_at: '2026-01-15T10:00:00Z',
  },
  {
    id: 'id-auth-svc',
    name: 'Auth Service',
    slug: 'auth-svc',
    description: 'Authentication and authorization',
    created_at: '2026-01-20T10:00:00Z',
  },
]

// Demo scenario — video on for this file; global config keeps video: 'off'.
test.use({
  video: 'on',
  viewport: { width: 1280, height: 720 },
  launchOptions: { slowMo: 300 },
})

test('demo: product list and detail — home lists products, clicking a card opens detail page', async ({
  page,
}) => {
  // Stall OIDC discovery so the library does not attempt network validation.
  await page.route('**/openid-configuration', () => {})

  // Mock GET /api/v1/products to return fixture data — no backend required.
  await page.route('**/api/v1/products', (route) => {
    route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(mockProducts),
    })
  })

  // Inject authenticated user directly via addInitScript so it is available
  // before the React app boots and reads sessionStorage.
  await page.addInitScript(
    ({ key, value }) => sessionStorage.setItem(key, JSON.stringify(value)),
    { key: SESSION_KEY, value: fakeUser },
  )

  // Step 1: visit / — ProtectedRoute passes, HomePage fetches and renders product grid.
  await page.goto('/')
  await expect(page.getByTestId('product-grid')).toBeVisible()

  // Both product cards should be visible.
  const cards = page.getByTestId('product-card')
  await expect(cards).toHaveCount(2)
  await expect(cards.first()).toContainText('Platform API')
  await expect(cards.first()).toContainText('platform-api')
  await page.waitForTimeout(800)

  // Step 2: click the first card — navigates to /products/platform-api.
  await cards.first().click()
  await expect(page).toHaveURL(/\/products\/platform-api/)

  // Step 3: product detail page shows name, slug, and description.
  await expect(page.getByRole('heading', { name: 'Platform API' })).toBeVisible()
  await expect(page.locator('.slug-tag', { hasText: 'platform-api' }).first()).toBeVisible()
  await expect(page.getByText('Core platform services')).toBeVisible()

  // Placeholder sections are present.
  await expect(page.getByText('Components')).toBeVisible()
  await expect(page.getByText('Environments')).toBeVisible()
  await page.waitForTimeout(800)

  // Step 4: breadcrumb back link returns to the product list.
  await page.getByRole('link', { name: 'Products' }).click()
  await expect(page).toHaveURL('/')
  await expect(page.getByTestId('product-grid')).toBeVisible()

  // Hold end state for the recording.
  await page.waitForTimeout(1500)
})

test('empty state: user with no products sees empty-state message', async ({ page }) => {
  await page.route('**/openid-configuration', () => {})
  await page.route('**/api/v1/products', (route) => {
    route.fulfill({ status: 200, contentType: 'application/json', body: '[]' })
  })

  await page.addInitScript(
    ({ key, value }) => sessionStorage.setItem(key, JSON.stringify(value)),
    { key: SESSION_KEY, value: fakeUser },
  )

  await page.goto('/')
  await expect(page.getByTestId('empty-state')).toBeVisible()
})
