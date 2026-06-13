import { test, expect } from '@playwright/test'

// The dist build picks up VITE_OIDC_* from the repo-root .env via Vite's envDir:'..'
// setting. oidc-client-ts constructs the sessionStorage key as:
// oidc.user:{authority}:{client_id}
const SESSION_KEY = 'oidc.user:http://localhost:8080/realms/kubegate:kubegate-frontend'

const fakeAdminUser = {
  profile: { sub: 'test-admin-1', name: 'Sara Ferrario', email: 'sara@example.com' },
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
  my_role: 'admin',
}

const fakeEnvironments = [
  {
    id: 'env-1',
    product_id: 'prod-1',
    name: 'staging',
    type: 'integration',
    overlay_path: 'overlays/staging',
    slug: 'staging',
    created_at: '2026-01-15T00:00:00Z',
  },
]

const fakeWorkloads = [
  { name: 'main', image_repository: 'europe-west4-docker.pkg.dev/acme/platform/api' },
  { name: 'cron', image_repository: 'europe-west4-docker.pkg.dev/acme/platform/cron' },
]

const fakeTags = {
  tags: [
    { name: 'v1.14.2', digest: 'sha256:abc', pushed_at: '2026-06-09T10:00:00Z' },
    { name: 'v1.14.1', digest: 'sha256:def', pushed_at: '2026-06-07T10:00:00Z' },
  ],
  next_page_token: '',
}

// Demo scenario — video on for this file only; global config keeps video: 'off'.
// test.use must be top-level (not inside describe) when using video or launchOptions.
test.use({
  video: 'on',
  viewport: { width: 1280, height: 720 },
  launchOptions: { slowMo: 300 },
})

test('demo: product detail page shows Workloads tab, lists discovered workloads, opens deploy dialog', async ({ page }) => {
  // Stall OIDC discovery so the library does not attempt network validation.
  await page.route('**/openid-configuration', () => {})

  // Inject authenticated admin session via addInitScript so it is available
  // before the React app boots and reads sessionStorage.
  await page.addInitScript(
    ({ key, value }) => sessionStorage.setItem(key, JSON.stringify(value)),
    { key: SESSION_KEY, value: fakeAdminUser },
  )

  // Set up API mocks for the full demo flow.
  await page.route('**/api/v1/**', async (route) => {
    const url = route.request().url()
    const method = route.request().method()

    if (url.includes('/workloads/main/tags') && method === 'GET') {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(fakeTags),
      })
    } else if (url.endsWith('/environments/env-1/workloads') && method === 'GET') {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(fakeWorkloads),
      })
    } else if (url.endsWith('/platform-api/environments') && method === 'GET') {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(fakeEnvironments),
      })
    } else if (url.endsWith('/tag-convention') && method === 'GET') {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ regex: '^v.*$', source: 'default' }),
      })
    } else if (url.endsWith('/api/v1/products') && method === 'GET') {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify([fakeProduct]),
      })
    } else {
      await route.continue()
    }
  })

  // Navigate to the home page (product list).
  await page.goto('/')

  // Click on the product card to navigate to the product detail page.
  await expect(page.locator('.home-p-name', { hasText: 'Platform API' })).toBeVisible()
  await page.locator('.home-p-name', { hasText: 'Platform API' }).click()

  // The product detail page should show the "Workloads" tab, not "Components".
  await expect(page.getByRole('button', { name: 'Workloads' })).toBeVisible()
  expect(await page.getByRole('button', { name: 'Components' }).count()).toBe(0)

  // The workloads table should render with workload names and image repositories.
  await expect(page.getByText('main', { exact: true }).first()).toBeVisible()
  await expect(page.getByText('cron', { exact: true }).first()).toBeVisible()
  await expect(page.getByText('europe-west4-docker.pkg.dev/acme/platform/api')).toBeVisible()

  // No "Add Component" button should be present.
  expect(await page.getByRole('button', { name: 'Add Component' }).count()).toBe(0)

  // Click the Deploy button on the first workload (main).
  const deployButtons = page.getByRole('button', { name: /Deploy main/i })
  await expect(deployButtons.first()).toBeVisible()
  await deployButtons.first().click()

  // The deploy dialog should open showing the workload name.
  await expect(page.getByText(/Seleziona tag — main/i)).toBeVisible()

  // Hold the end state visible before the test ends.
  await page.waitForTimeout(1500)
})
