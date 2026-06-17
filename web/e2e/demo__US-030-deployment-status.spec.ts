import { test, expect } from '@playwright/test'

const SESSION_KEY = 'oidc.user:http://localhost:8080/realms/kubegate:kubegate-frontend'

const fakeViewerUser = {
  profile: { sub: 'test-viewer-1', name: 'Marco Andreoli', email: 'marco@example.com' },
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
  description: 'Core platform API service',
  created_at: '2026-01-01T00:00:00Z',
  my_role: 'viewer',
}

const fakeEnvironments = [
  {
    id: 'env-dev',
    product_id: 'prod-1',
    name: 'dev',
    slug: 'dev',
    type: 'dev',
    gitops_path: 'apps/dev/platform-api/platform-api-helmrelease.yaml',
    created_at: '2026-01-01T00:00:00Z',
  },
  {
    id: 'env-prod',
    product_id: 'prod-1',
    name: 'production',
    slug: 'production',
    type: 'production',
    gitops_path: 'apps/production/platform-api/platform-api-helmrelease.yaml',
    created_at: '2026-01-01T00:00:00Z',
  },
]

const fakeStatus = {
  workloads: {
    api: { dev: 'v2.14.3-rc.7', production: 'v2.12.0' },
    worker: { dev: 'v1.4.1', production: 'N/A' },
  },
  fetched_at: '2026-06-17T12:00:00Z',
  stale: false,
}

// Demo scenario — video on for this file only; global config keeps video: 'off'.
test.use({
  video: 'on',
  viewport: { width: 1280, height: 720 },
  launchOptions: { slowMo: 300 },
})

test('demo: user navigates to Status tab and sees deployment matrix with current tags', async ({ page }) => {
  // Stall OIDC discovery so the library does not attempt network validation.
  await page.route('**/openid-configuration', () => {})

  // Inject a valid session so the SPA treats the user as authenticated.
  await page.addInitScript(
    ({ key, value }) => sessionStorage.setItem(key, JSON.stringify(value)),
    { key: SESSION_KEY, value: fakeViewerUser },
  )

  // Mock API responses.
  await page.route('**/api/v1/**', async (route) => {
    const url = route.request().url()
    if (url.includes('/products') && !url.includes('/status') && !url.includes('/environments') && url.endsWith('/products')) {
      await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify([fakeProduct]) })
    } else if (url.includes('/environments')) {
      await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(fakeEnvironments) })
    } else if (url.includes('/status')) {
      await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(fakeStatus) })
    } else {
      await route.continue()
    }
  })

  // Step 1: navigate to home — user is already authenticated via sessionStorage.
  await page.goto('/')
  await expect(page.getByText('Platform API')).toBeVisible()

  // Step 2: navigate to the product detail page.
  await page.getByText('Platform API').click()
  await expect(page.getByText('Workloads')).toBeVisible()

  // Step 3: click the Status tab.
  await page.getByRole('button', { name: 'Status' }).click()
  await expect(page).toHaveURL(/\/products\/platform-api\/status/)

  // Step 4: verify the matrix is rendered with correct tags.
  await expect(page.getByTestId('status-matrix')).toBeVisible()
  await expect(page.getByText('api')).toBeVisible()
  await expect(page.getByText('worker')).toBeVisible()
  await expect(page.getByText('v2.14.3-rc.7')).toBeVisible()
  await expect(page.getByText('v2.12.0')).toBeVisible()

  // Step 5: N/A is visible for environments where tag is not set.
  const naElements = await page.getByText('N/A').all()
  expect(naElements.length).toBeGreaterThan(0)

  // Hold the final state visible for at least 1.5 s so the video captures the outcome.
  await page.waitForTimeout(1500)
})
