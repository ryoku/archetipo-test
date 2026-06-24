import { test, expect } from '@playwright/test'

// oidc-client-ts constructs the sessionStorage key as `oidc.user:${authority}:${clientId}`.
// When VITE_OIDC_ISSUER_URL / VITE_OIDC_CLIENT_ID are defined in .env the built bundle
// uses the real values; when they are absent (CI without .env) they become "undefined".
// We seed both keys so the test works in either environment.
const SESSION_KEYS = [
  'oidc.user:undefined:undefined',
  'oidc.user:http://localhost:8080/realms/kubegate:kubegate-frontend',
]

const fakeAdminUser = {
  profile: { sub: 'test-admin-1', name: 'Giulia Romano', email: 'giulia@example.com' },
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

const fakeDeploymentsPage1 = {
  deployments: [
    {
      id: 'dep-1',
      actor_display_name: 'Marco Andreoli',
      component_name: 'api',
      environment_name: 'production',
      tag: 'v1.14.2',
      deployed_at: '2026-06-16T09:41:00Z',
      commit_sha: 'abc1234',
      outcome: 'success',
    },
    {
      id: 'dep-2',
      actor_display_name: 'Lucia Parisi',
      component_name: 'worker',
      environment_name: 'integration',
      tag: 'v1.14.2-rc.1',
      deployed_at: '2026-06-15T17:03:00Z',
      commit_sha: 'def5678',
      outcome: 'success',
    },
    {
      id: 'dep-3',
      actor_display_name: 'Marco Andreoli',
      component_name: 'frontend',
      environment_name: 'development',
      tag: 'main-20260615-b2f',
      deployed_at: '2026-06-15T14:28:00Z',
      commit_sha: '',
      outcome: 'failure',
      error_message: 'gitops push failed: permission denied',
    },
  ],
  page: 1,
  page_size: 20,
  total: 24,
}

const fakeDeploymentsPage2 = {
  deployments: [
    {
      id: 'dep-4',
      actor_display_name: 'Fabio Trentini',
      component_name: 'api',
      environment_name: 'integration',
      tag: 'v1.14.1',
      deployed_at: '2026-06-14T11:55:00Z',
      commit_sha: 'ghi9012',
      outcome: 'success',
    },
  ],
  page: 2,
  page_size: 20,
  total: 24,
}

// Demo scenario — video on for this file only; global config keeps video: 'off'.
test.use({
  video: 'on',
  viewport: { width: 1280, height: 720 },
  launchOptions: { slowMo: 300 },
})

test('demo: user views deployment history tab with paginated records', async ({ page }) => {
  await page.route('**/openid-configuration', () => {})

  await page.addInitScript(
    ({ keys, value }) => {
      const json = JSON.stringify(value)
      for (const key of keys) sessionStorage.setItem(key, json)
    },
    { keys: SESSION_KEYS, value: fakeAdminUser },
  )

  await page.route('**/api/v1/**', async (route) => {
    const url = route.request().url()
    const method = route.request().method()

    if (url.includes('/api/v1/products/platform-api/deployments')) {
      const parsedUrl = new URL(url)
      const p = parsedUrl.searchParams.get('page') ?? '1'
      const body = p === '2' ? fakeDeploymentsPage2 : fakeDeploymentsPage1
      await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(body) })
    } else if (url.includes('/api/v1/products/platform-api/environments') && method === 'GET') {
      await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify([]) })
    } else if (url.includes('/api/v1/products/platform-api') && method === 'GET') {
      await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify([]) })
    } else if (url.endsWith('/api/v1/products') && method === 'GET') {
      await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify([fakeProduct]) })
    } else {
      await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify([]) })
    }
  })

  // Step 1: land on the home page and wait for the product list.
  await page.goto('/')
  await expect(page.locator('.home-p-name', { hasText: 'Platform API' })).toBeVisible()
  await page.waitForTimeout(600)

  // Step 2: click the product card to navigate to the product detail page.
  await page.locator('.home-p-name', { hasText: 'Platform API' }).click()
  await expect(page).toHaveURL(/\/products\/platform-api/)
  await page.waitForTimeout(600)

  // Step 3: click the History tab in the subnav.
  await page.locator('.pd-subnav-link', { hasText: 'History' }).click()
  await expect(page).toHaveURL(/\/products\/platform-api\/history/)
  await expect(page.locator('.pd-subnav-link--active', { hasText: 'History' })).toBeVisible()
  await page.waitForTimeout(700)

  // Step 4: verify the table shows all required fields.
  await expect(page.locator('.hist-actor-name', { hasText: 'Marco Andreoli' }).first()).toBeVisible()
  await expect(page.locator('.hist-comp-name', { hasText: 'api' }).first()).toBeVisible()
  await expect(page.locator('.hist-env-badge', { hasText: 'production' })).toBeVisible()
  await expect(page.locator('.hist-tag-chip').filter({ hasText: /^v1\.14\.2$/ })).toBeVisible()
  await expect(page.locator('.hist-outcome--success').first()).toBeVisible()
  await expect(page.locator('.hist-outcome--failure').first()).toBeVisible()
  await page.waitForTimeout(700)

  // Step 5: verify pagination — Previous disabled, Next enabled (24 total, page_size 20).
  await expect(page.locator('.hist-btn-page', { hasText: 'Previous' })).toBeDisabled()
  await expect(page.locator('.hist-btn-page', { hasText: 'Next' })).toBeEnabled()
  await page.waitForTimeout(500)

  // Step 6: click Next and verify page 2 loads with new records.
  await page.locator('.hist-btn-page', { hasText: 'Next' }).click()
  await expect(page.locator('.hist-tag-chip', { hasText: 'v1.14.1' })).toBeVisible()
  await expect(page.locator('.hist-page-info')).toContainText('2')
  await page.waitForTimeout(1500)
})
