import { test, expect } from '@playwright/test'

// Match the OIDC config baked into the SPA build.
const OIDC_AUTHORITY = 'http://localhost:8080/realms/kubegate'
const OIDC_CLIENT_ID = 'kubegate-frontend'
const SESSION_KEY = `oidc.user:${OIDC_AUTHORITY}:${OIDC_CLIENT_ID}`

const fakeUser = {
  profile: { sub: 'test-user-1', name: 'Sara Bianchi', email: 'sara@example.com' },
  session_state: null,
  access_token: 'fake-access-token',
  refresh_token: null,
  token_type: 'Bearer',
  scope: 'openid profile email',
  expires_at: 9999999999,
  id_token: 'fake-id-token',
}

const fakeProduct = {
  id: 'prod-1',
  name: 'Platform API',
  slug: 'platform-api',
  description: 'Core API service for the platform team',
  created_at: '2026-01-01T00:00:00Z',
  my_role: 'admin',
}

const fakeEnvironment = {
  id: 'env-1',
  product_id: 'prod-1',
  name: 'test-env',
  type: 'production',
  overlay_path: 'overlays/test',
  created_at: '2026-06-07T10:00:00Z',
}

// Demo scenario — video on for this file only; global config keeps video: 'off'.
test.use({
  video: 'on',
  viewport: { width: 1280, height: 720 },
  launchOptions: { slowMo: 300 },
})

test('demo: Sara navigates to Environments, adds one, then deletes it', async ({ page }) => {
  // Stall OIDC discovery so the LoginPage stays visible during initial navigation.
  await page.route(`${OIDC_AUTHORITY}/.well-known/openid-configuration`, () => {
    // Never fulfilled — keeps the redirect spinner visible on screen.
  })

  // Step 1: visit / unauthenticated → ProtectedRoute redirects to /login.
  await page.goto('/')
  await expect(page).toHaveURL(/\/login/)
  await page.waitForTimeout(600)

  // Step 2: inject OIDC session and set up API mocks.
  await page.evaluate(
    ({ key, value }) => sessionStorage.setItem(key, JSON.stringify(value)),
    { key: SESSION_KEY, value: fakeUser },
  )

  // Mock all API calls for the demo flows.
  await page.route('**/api/v1/**', async (route) => {
    const url = route.request().url()
    const method = route.request().method()

    if (/\/api\/v1\/products$/.test(url) && method === 'GET') {
      // Product list
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify([fakeProduct]),
      })
    } else if (/\/api\/v1\/products\/platform-api\/environments\/env-1$/.test(url) && method === 'DELETE') {
      // Delete environment
      await route.fulfill({ status: 204 })
    } else if (/\/api\/v1\/products\/platform-api\/environments$/.test(url) && method === 'GET') {
      // Environment list — always empty (React adds the new environment via optimistic update after POST)
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify([]),
      })
    } else if (/\/api\/v1\/products\/platform-api\/environments$/.test(url) && method === 'POST') {
      // Create environment
      await route.fulfill({
        status: 201,
        contentType: 'application/json',
        body: JSON.stringify(fakeEnvironment),
      })
    } else {
      await route.continue()
    }
  })

  // Step 3: navigate to / — AuthProvider reads user from sessionStorage, home shows product list.
  await page.goto('/')
  await expect(page.getByText('Platform API')).toBeVisible()
  await expect(page.getByText('platform-api')).toBeVisible()
  await page.waitForTimeout(800)

  // Step 4: click the product card → product detail page loads.
  await page.getByText('Platform API').click()
  await expect(page).toHaveURL(/\/products\/platform-api/)
  await expect(page.getByText('Registered Components')).toBeVisible()
  await page.waitForTimeout(700)

  // Step 5: click "Environments" in the sub-nav → EnvironmentsPage loads.
  await page.getByRole('button', { name: 'Environments' }).click()
  await expect(page).toHaveURL(/\/products\/platform-api\/environments/)
  await expect(page.getByText('Deployment Environments')).toBeVisible()
  await page.waitForTimeout(600)

  // Step 6: verify empty state is visible (no environments yet).
  await expect(page.getByTestId('empty-environments')).toBeVisible()
  await page.waitForTimeout(500)

  // Step 7: click "Add Environment" → inline form appears.
  await page.getByRole('button', { name: 'Add Environment' }).click()
  await expect(page.getByText('New Environment')).toBeVisible()
  await page.waitForTimeout(400)

  // Step 8: fill in the form fields.
  await page.getByLabel('Name *').fill('test-env')
  await page.waitForTimeout(350)
  await page.getByLabel('Type *').selectOption('production')
  await page.waitForTimeout(350)
  await page.getByLabel('Overlay Path *').fill('overlays/test')
  await page.waitForTimeout(500)

  // Step 9: submit → environment appears in the table with production badge.
  await page.getByRole('button', { name: 'Save Environment' }).click()
  await expect(page.getByText('test-env')).toBeVisible()
  await expect(page.locator('.env-type-badge', { hasText: 'production' })).toBeVisible()
  await page.waitForTimeout(900)

  // Step 10: click Delete on the environment row → confirm dialog opens.
  await page.getByRole('button', { name: 'Delete test-env' }).click()
  await expect(page.getByText('Remove Environment')).toBeVisible()
  await expect(page.getByText('This action cannot be undone.')).toBeVisible()
  await page.waitForTimeout(700)

  // Step 11: confirm deletion → environment disappears; empty state returns.
  await page.getByRole('button', { name: 'Delete Environment' }).click()
  await expect(page.getByTestId('empty-environments')).toBeVisible()

  // Hold end state so the recording captures the final outcome.
  await page.waitForTimeout(1500)
})
