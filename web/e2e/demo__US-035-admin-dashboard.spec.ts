import { test, expect } from '@playwright/test'

const OIDC_AUTHORITY = 'http://localhost:8080/realms/kubegate'
const OIDC_CLIENT_ID = 'kubegate-frontend'
const SESSION_KEY = `oidc.user:${OIDC_AUTHORITY}:${OIDC_CLIENT_ID}`

const DEVOPS_ADMIN_PAYLOAD = btoa(
  JSON.stringify({
    sub: 'admin-user-1',
    name: 'Sara Bianchi',
    email: 'sara@example.com',
    realm_access: { roles: ['kubegate:devops-admin'] },
    exp: 9999999999,
  }),
)
  .replaceAll('+', '-')
  .replaceAll('/', '_')
  .replaceAll(/=+$/g, '')

const DEVOPS_ADMIN_TOKEN = `eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.${DEVOPS_ADMIN_PAYLOAD}.fake-sig`

function makeAdminSession(name: string, accessToken: string) {
  return {
    profile: { sub: 'admin-user-1', name, email: 'sara@example.com' },
    session_state: null,
    access_token: accessToken,
    refresh_token: null,
    token_type: 'Bearer',
    scope: 'openid profile email',
    expires_at: 9999999999,
    id_token: 'fake-id-token',
  }
}

const ADMIN_PRODUCTS = [
  {
    id: 'p-1',
    name: 'Platform API',
    slug: 'platform-api',
    description: 'Core platform',
    created_at: '2026-01-01T00:00:00Z',
    environment_count: 3,
    last_deployed_at: '2026-06-14T10:00:00Z',
  },
  {
    id: 'p-2',
    name: 'Customer App',
    slug: 'customer-app',
    description: 'Customer-facing app',
    created_at: '2026-02-01T00:00:00Z',
    environment_count: 1,
    last_deployed_at: null,
  },
]

// ─────────────────────────────────────────────────────────────────
// Demo scenario — video on; slowMo for readability
// ─────────────────────────────────────────────────────────────────
test.use({
  video: 'on',
  viewport: { width: 1280, height: 720 },
  launchOptions: { slowMo: 300 },
})

test('demo: Sara (DevOps Admin) views the admin dashboard, sorts by environments, and navigates to a product', async ({ page }) => {
  // Step 1: stall OIDC discovery so login page is visible briefly
  await page.route(`${OIDC_AUTHORITY}/.well-known/openid-configuration`, () => {})

  // Step 2: navigate to / unauthenticated → redirect to /login
  await page.goto('/')
  await expect(page).toHaveURL(/\/login/)
  await page.waitForTimeout(600)

  // Step 3: inject admin session
  await page.evaluate(
    ({ key, value }) => sessionStorage.setItem(key, JSON.stringify(value)),
    { key: SESSION_KEY, value: makeAdminSession('Sara Bianchi', DEVOPS_ADMIN_TOKEN) },
  )

  // Step 4: mock admin products and products API
  await page.route('**/api/v1/admin/products', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(ADMIN_PRODUCTS),
    })
  })
  await page.route('**/api/v1/products', async (route) => {
    if (route.request().method() === 'GET') {
      await route.fulfill({ status: 200, contentType: 'application/json', body: '[]' })
    } else {
      await route.continue()
    }
  })

  // Step 5: navigate to /admin — table should be visible
  await page.goto('/admin')
  await expect(page.getByTestId('products-table')).toBeVisible()
  await page.waitForTimeout(700)

  // Step 6: verify table columns
  await expect(page.getByRole('columnheader', { name: /Name/i })).toBeVisible()
  await expect(page.getByRole('columnheader', { name: /Environments/i })).toBeVisible()
  await expect(page.getByRole('columnheader', { name: /Last Deployment/i })).toBeVisible()
  await page.waitForTimeout(500)

  // Step 7: verify both product rows are present (default sort: name asc)
  await expect(page.getByTestId('product-row')).toHaveCount(2)
  await expect(page.getByText('Platform API')).toBeVisible()
  await expect(page.getByText('Customer App')).toBeVisible()
  await page.waitForTimeout(500)

  // Step 8: sort by Environments ascending — Customer App (1) comes before Platform API (3)
  await page.getByRole('columnheader', { name: /Environments/i }).click()
  await page.waitForTimeout(500)
  await expect(page.getByTestId('product-row').first()).toContainText('Customer App')
  await page.waitForTimeout(500)

  // Step 9: click Platform API row → navigate to its detail page
  await page.getByTestId('product-row').nth(1).click()
  await expect(page).toHaveURL(/\/products\/platform-api/)
  await page.waitForTimeout(1500)
})
