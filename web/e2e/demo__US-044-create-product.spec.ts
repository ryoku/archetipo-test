import { test, expect } from '@playwright/test'

// Match the OIDC config baked into the SPA build.
const OIDC_AUTHORITY = 'http://localhost:8080/realms/kubegate'
const OIDC_CLIENT_ID = 'kubegate-frontend'
const SESSION_KEY = `oidc.user:${OIDC_AUTHORITY}:${OIDC_CLIENT_ID}`

// A real JWT (header.payload.signature) whose payload carries the kubegate:devops-admin role.
// isDevOpsAdmin() base64-decodes the middle segment and checks realm_access.roles.
// The signature segment is arbitrary — the SPA never verifies it locally.
const DEVOPS_ADMIN_PAYLOAD = btoa(
  JSON.stringify({
    sub: 'admin-user-1',
    name: 'Luca Ferrari',
    email: 'luca@example.com',
    realm_access: { roles: ['kubegate:devops-admin'] },
    exp: 9999999999,
  }),
)
  .replaceAll('+', '-')
  .replaceAll('/', '_')
  .replaceAll(/=+$/g, '')

const DEVOPS_ADMIN_TOKEN = `eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.${DEVOPS_ADMIN_PAYLOAD}.fake-sig`

// A non-admin JWT: no kubegate:devops-admin role.
const VIEWER_PAYLOAD = btoa(
  JSON.stringify({
    sub: 'viewer-user-1',
    name: 'Maria Rossi',
    email: 'maria@example.com',
    realm_access: { roles: ['kubegate:viewer'] },
    exp: 9999999999,
  }),
)
  .replaceAll('+', '-')
  .replaceAll('/', '_')
  .replaceAll(/=+$/g, '')

const VIEWER_TOKEN = `eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.${VIEWER_PAYLOAD}.fake-sig`

function makeAdminSession(name: string, accessToken: string) {
  return {
    profile: { sub: 'admin-user-1', name, email: 'luca@example.com' },
    session_state: null,
    access_token: accessToken,
    refresh_token: null,
    token_type: 'Bearer',
    scope: 'openid profile email',
    expires_at: 9999999999,
    id_token: 'fake-id-token',
  }
}

function makeViewerSession(name: string, accessToken: string) {
  return {
    profile: { sub: 'viewer-user-1', name, email: 'maria@example.com' },
    session_state: null,
    access_token: accessToken,
    refresh_token: null,
    token_type: 'Bearer',
    scope: 'openid profile email',
    expires_at: 9999999999,
    id_token: 'fake-id-token',
  }
}

const newProduct = {
  id: '1',
  name: 'Platform API',
  slug: 'platform-api',
  description: 'Core platform',
  created_at: '2026-01-01T00:00:00Z',
  my_role: 'admin' as const,
}

// ─────────────────────────────────────────────────────────────────
// Demo scenario — video on; slowMo for readability
// ─────────────────────────────────────────────────────────────────
test.describe('Demo: US-044 — DevOps Admin creates a product', () => {
  test.use({
    video: 'on',
    viewport: { width: 1280, height: 720 },
    launchOptions: { slowMo: 300 },
  })

  test('demo: Luca (DevOps Admin) opens the form, fills it, and sees the new product card', async ({ page }) => {
    // Stall OIDC discovery so LoginPage stays visible briefly.
    await page.route(`${OIDC_AUTHORITY}/.well-known/openid-configuration`, () => {
      // Never fulfilled — keeps the redirect animation visible on screen.
    })

    // Step 1: visit / unauthenticated → ProtectedRoute redirects to /login.
    await page.goto('/')
    await expect(page).toHaveURL(/\/login/)
    await page.waitForTimeout(600)

    // Step 2: inject OIDC session for a DevOps Admin user.
    await page.evaluate(
      ({ key, value }) => sessionStorage.setItem(key, JSON.stringify(value)),
      { key: SESSION_KEY, value: makeAdminSession('Luca Ferrari', DEVOPS_ADMIN_TOKEN) },
    )

    // Step 3: set up API mocks — products list starts empty, POST returns the new product.
    await page.route('**/api/v1/**', async (route) => {
      const url = route.request().url()
      const method = route.request().method()

      if (url.endsWith('/api/v1/products') && method === 'GET') {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify([]),
        })
      } else if (url.endsWith('/api/v1/products') && method === 'POST') {
        await route.fulfill({
          status: 201,
          contentType: 'application/json',
          body: JSON.stringify(newProduct),
        })
      } else {
        await route.continue()
      }
    })

    // Step 4: navigate to / — AuthProvider picks up admin session from sessionStorage.
    await page.goto('/')
    await expect(page.getByTestId('empty-state')).toBeVisible()
    await page.waitForTimeout(700)

    // Step 5: verify "Add Product" button is visible for a DevOps Admin.
    await expect(page.getByRole('button', { name: 'Add Product' })).toBeVisible()
    await page.waitForTimeout(500)

    // Step 6: click "Add Product" → inline form appears.
    await page.getByRole('button', { name: 'Add Product' }).click()
    await expect(page.getByText('New Product')).toBeVisible()
    await page.waitForTimeout(400)

    // Step 7: fill in the Name field.
    await page.getByLabel('Name *').fill('Platform API')
    await page.waitForTimeout(350)

    // Step 8: fill in the Slug field.
    await page.getByLabel('Slug *').fill('platform-api')
    await page.waitForTimeout(350)

    // Step 9: fill in the Description field.
    await page.getByLabel('Description').fill('Core platform')
    await page.waitForTimeout(500)

    // Step 10: click Save → POST fires, new product card appears, form closes.
    await page.getByRole('button', { name: 'Save Product' }).click()
    await expect(page.getByText('Platform API')).toBeVisible()
    await expect(page.getByText('platform-api')).toBeVisible()
    await expect(page.getByText('New Product')).not.toBeVisible()
    await page.waitForTimeout(900)

    // Hold final state so the recording captures the outcome.
    await page.waitForTimeout(1500)
  })
})

// ─────────────────────────────────────────────────────────────────
// Non-demo scenarios — no video, normal speed
// ─────────────────────────────────────────────────────────────────
test.describe('US-044 — Additional scenarios', () => {
  // Scenario 2: validation error — submit empty form → Name error shown
  test('shows validation error when form is submitted empty', async ({ page }) => {
    // addInitScript runs before every navigation, so sessionStorage is populated
    // before the React app boots and reads it.
    await page.addInitScript(
      ({ key, value }) => sessionStorage.setItem(key, JSON.stringify(value)),
      { key: SESSION_KEY, value: makeAdminSession('Luca Ferrari', DEVOPS_ADMIN_TOKEN) },
    )

    await page.route('**/api/v1/**', async (route) => {
      const url = route.request().url()
      const method = route.request().method()

      if (url.endsWith('/api/v1/products') && method === 'GET') {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify([]),
        })
      } else {
        await route.continue()
      }
    })

    await page.goto('/')
    await expect(page.getByTestId('empty-state')).toBeVisible()

    await page.getByRole('button', { name: 'Add Product' }).click()
    await expect(page.getByText('New Product')).toBeVisible()

    // Submit without filling anything
    await page.getByRole('button', { name: 'Save Product' }).click()

    // Name error must appear
    await expect(page.getByText('Name is required')).toBeVisible()

    // Form must remain open
    await expect(page.getByLabel('Name *')).toBeVisible()
  })

  // Scenario 3: slug conflict — POST returns 409 → conflict error shown, form stays open
  test('shows conflict error and keeps form open on 409 response', async ({ page }) => {
    await page.addInitScript(
      ({ key, value }) => sessionStorage.setItem(key, JSON.stringify(value)),
      { key: SESSION_KEY, value: makeAdminSession('Luca Ferrari', DEVOPS_ADMIN_TOKEN) },
    )

    await page.route('**/api/v1/**', async (route) => {
      const url = route.request().url()
      const method = route.request().method()

      if (url.endsWith('/api/v1/products') && method === 'GET') {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify([]),
        })
      } else if (url.endsWith('/api/v1/products') && method === 'POST') {
        // Simulate a slug conflict
        await route.fulfill({
          status: 409,
          contentType: 'application/json',
          body: JSON.stringify({ error: 'slug already exists' }),
        })
      } else {
        await route.continue()
      }
    })

    await page.goto('/')
    await expect(page.getByTestId('empty-state')).toBeVisible()

    await page.getByRole('button', { name: 'Add Product' }).click()
    await expect(page.getByText('New Product')).toBeVisible()

    await page.getByLabel('Name *').fill('Platform API')
    await page.getByLabel('Slug *').fill('platform-api')

    await page.getByRole('button', { name: 'Save Product' }).click()

    // Conflict error must appear
    await expect(page.getByText('A product with this slug already exists')).toBeVisible()

    // Form must stay open
    await expect(page.getByLabel('Name *')).toBeVisible()
  })

  // Scenario 4: non-admin user does not see "Add Product" button
  test('hides Add Product button for a non-admin user', async ({ page }) => {
    await page.addInitScript(
      ({ key, value }) => sessionStorage.setItem(key, JSON.stringify(value)),
      { key: SESSION_KEY, value: makeViewerSession('Maria Rossi', VIEWER_TOKEN) },
    )

    await page.route('**/api/v1/**', async (route) => {
      const url = route.request().url()
      const method = route.request().method()

      if (url.endsWith('/api/v1/products') && method === 'GET') {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify([]),
        })
      } else {
        await route.continue()
      }
    })

    await page.goto('/')
    await expect(page.getByTestId('empty-state')).toBeVisible()

    // "Add Product" must not be visible for a viewer
    await expect(page.getByRole('button', { name: 'Add Product' })).not.toBeVisible()
  })
})
