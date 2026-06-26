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
]

const ACTIVITY_EVENTS = [
  {
    id: 'evt-1',
    actor_display_name: 'Sara Bianchi',
    tag: 'v1.15.0',
    component_name: 'api-gateway',
    product_slug: 'platform-api',
    environment_name: 'production',
    deployed_at: new Date().toISOString(),
    outcome: 'in_progress',
    error_message: null,
  },
  {
    id: 'evt-2',
    actor_display_name: 'Marco Rossi',
    tag: 'v3.2.1',
    component_name: 'frontend',
    product_slug: 'customer-app',
    environment_name: 'production',
    deployed_at: new Date(Date.now() - 2 * 60000).toISOString(),
    outcome: 'success',
    error_message: null,
  },
  {
    id: 'evt-3',
    actor_display_name: 'Laura Conti',
    tag: 'v0.9.4',
    component_name: 'worker',
    product_slug: 'data-sync',
    environment_name: 'staging',
    deployed_at: new Date(Date.now() - 18 * 60000).toISOString(),
    outcome: 'failure',
    error_message: 'ErrImagePull: manifest for registry.example.com/worker:v0.9.4 not found',
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

test('demo: Sara (DevOps Admin) views the live activity feed with in-progress, success, and failure deployments', async ({ page }) => {
  // Step 1: stall OIDC discovery so login page is briefly visible
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

  // Step 4: mock backend APIs
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
  await page.route('**/api/v1/admin/activity', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(ACTIVITY_EVENTS),
    })
  })

  // Step 5: navigate to /admin
  await page.goto('/admin')
  await expect(page.getByTestId('products-table')).toBeVisible()
  await page.waitForTimeout(700)

  // Step 6: scroll down to activity panel and verify it is visible
  await page.getByTestId('activity-section').scrollIntoViewIfNeeded()
  await expect(page.getByTestId('activity-section')).toBeVisible()
  await page.waitForTimeout(600)

  // Step 7: verify 3 activity rows are rendered
  const activityList = page.getByTestId('activity-list')
  await expect(page.getByTestId('activity-row')).toHaveCount(3)
  await page.waitForTimeout(500)

  // Step 8: verify in_progress row — pulsing dot visible
  await expect(page.getByTestId('activity-dot-in_progress')).toBeVisible()
  await expect(activityList.getByText('Sara Bianchi')).toBeVisible()
  await expect(activityList.getByText('v1.15.0')).toBeVisible()
  await page.waitForTimeout(600)

  // Step 9: verify success row — green solid dot
  await expect(page.getByTestId('activity-dot-success')).toBeVisible()
  await expect(activityList.getByText('Marco Rossi')).toBeVisible()
  await expect(activityList.getByText('v3.2.1')).toBeVisible()
  await page.waitForTimeout(600)

  // Step 10: verify failure row — red dot and error message
  await expect(page.getByTestId('activity-dot-failure')).toBeVisible()
  await expect(activityList.getByText('Laura Conti')).toBeVisible()
  await expect(page.getByTestId('activity-error-msg')).toBeVisible()
  await page.waitForTimeout(1500)
})
