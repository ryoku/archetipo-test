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
}

const fakeComponent = {
  id: 'comp-1',
  product_id: 'prod-1',
  name: 'API Gateway',
  slug: 'api-gateway',
  gcr_image_path: 'europe-west1-docker.pkg.dev/acme/platform/api-gateway',
  created_at: '2026-06-05T10:00:00Z',
}

// Demo scenario — video on for this file only; global config keeps video: 'off'.
test.use({
  video: 'on',
  viewport: { width: 1280, height: 720 },
  launchOptions: { slowMo: 300 },
})

test('demo: component CRUD — add component to product detail and remove it from UI', async ({
  page,
}) => {
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
    } else if (/\/api\/v1\/products\/platform-api\/components\/api-gateway$/.test(url) && method === 'DELETE') {
      // Delete component
      await route.fulfill({ status: 204 })
    } else if (/\/api\/v1\/products\/platform-api\/components$/.test(url) && method === 'GET') {
      // Component list — always empty (React adds the new component via optimistic update after POST)
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify([]),
      })
    } else if (/\/api\/v1\/products\/platform-api\/components$/.test(url) && method === 'POST') {
      // Create component
      await route.fulfill({
        status: 201,
        contentType: 'application/json',
        body: JSON.stringify(fakeComponent),
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
  await expect(page.getByTestId('empty-components')).toBeVisible()
  await page.waitForTimeout(700)

  // Step 5: click "Add Component" → inline form appears.
  await page.getByRole('button', { name: 'Add Component' }).click()
  await expect(page.getByText('New Component')).toBeVisible()
  await page.waitForTimeout(400)

  // Step 6: fill in the form fields.
  await page.getByLabel('Name *').fill('API Gateway')
  await page.waitForTimeout(350)
  await page.getByLabel('Slug *').fill('api-gateway')
  await page.waitForTimeout(350)
  await page
    .getByLabel('GCR Image Path *')
    .fill('europe-west1-docker.pkg.dev/acme/platform/api-gateway')
  await page.waitForTimeout(500)

  // Step 7: submit → component appears immediately in the table.
  await page.getByRole('button', { name: 'Save Component' }).click()
  await expect(page.getByText('API Gateway')).toBeVisible()
  // slug appears in .pd-comp-slug-str; use CSS scoping to avoid matching the GCR path substring
  await expect(page.locator('.pd-comp-slug-str', { hasText: 'api-gateway' })).toBeVisible()
  await page.waitForTimeout(900)

  // Step 8: click Delete on the component row → confirm dialog opens.
  await page.getByRole('button', { name: 'Delete API Gateway' }).click()
  await expect(page.getByText('Remove Component')).toBeVisible()
  await expect(page.getByText('This action cannot be undone.')).toBeVisible()
  await page.waitForTimeout(700)

  // Step 9: confirm deletion → component disappears; empty state returns.
  await page.getByRole('button', { name: 'Delete Component' }).click()
  await expect(page.getByTestId('empty-components')).toBeVisible()

  // Hold end state so the recording captures the final outcome.
  await page.waitForTimeout(1500)
})
