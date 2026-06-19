import { test, expect } from '@playwright/test'

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

const fakeStatus = {
  workloads: {
    api: { dev: 'v1.14.2', production: 'v1.12.0' },
    worker: { dev: 'v1.10.0', production: 'N/A' },
  },
  fetched_at: new Date().toISOString(),
  stale: false,
}

const fakeStatusStale = { ...fakeStatus, stale: true }

// Demo scenario — video on for this file only.
test.use({
  video: 'on',
  viewport: { width: 1280, height: 720 },
  launchOptions: { slowMo: 300 },
})

test('demo: product detail Status tab shows workload × environment deployment matrix', async ({ page }) => {
  await page.route('**/openid-configuration', () => {})

  await page.addInitScript(
    ({ key, value }) => sessionStorage.setItem(key, JSON.stringify(value)),
    { key: SESSION_KEY, value: fakeAdminUser },
  )

  await page.route('**/api/v1/**', async (route) => {
    const url = route.request().url()

    if (url.includes('/api/v1/products') && !url.includes('/platform-api')) {
      await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify([fakeProduct]) })
      return
    }
    if (url.includes('/platform-api/status')) {
      await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(fakeStatus) })
      return
    }
    if (url.includes('/platform-api/environments')) {
      await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify([]) })
      return
    }
    await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify([]) })
  })

  // Navigate to the product list (home page)
  await page.goto('/')
  await expect(page.getByText('Platform API')).toBeVisible()

  // Navigate to product detail by clicking the product card
  await page.getByText('Platform API').click()
  await expect(page.getByText('Status')).toBeVisible()

  // Click the Status tab
  await page.getByRole('button', { name: 'Status' }).click()
  await expect(page.getByTestId('status-matrix')).toBeVisible()

  // Verify the matrix shows correct tags
  await expect(page.getByTestId('tag-api-dev')).toHaveText('v1.14.2')
  await expect(page.getByTestId('tag-api-production')).toHaveText('v1.12.0')
  await expect(page.getByTestId('tag-worker-dev')).toHaveText('v1.10.0')
  await expect(page.getByTestId('tag-worker-production')).toHaveText('N/A')

  // Hold the final state visible
  await page.waitForTimeout(1500)
})

test('status tab shows stale badge and refreshes on click', async ({ page }) => {
  await page.route('**/openid-configuration', () => {})

  await page.addInitScript(
    ({ key, value }) => sessionStorage.setItem(key, JSON.stringify(value)),
    { key: SESSION_KEY, value: fakeAdminUser },
  )

  let callCount = 0
  await page.route('**/api/v1/**', async (route) => {
    const url = route.request().url()

    if (url.includes('/api/v1/products') && !url.includes('/platform-api')) {
      await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify([fakeProduct]) })
      return
    }
    if (url.includes('/platform-api/status')) {
      callCount++
      const body = callCount === 1 ? fakeStatusStale : fakeStatus
      await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(body) })
      return
    }
    await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify([]) })
  })

  await page.goto('/')
  await page.getByText('Platform API').click()
  await page.getByRole('button', { name: 'Status' }).click()
  await expect(page.getByTestId('stale-badge')).toBeVisible()

  // Refresh clears the stale badge
  await page.getByTestId('status-refresh').click()
  await expect(page.getByTestId('stale-badge')).not.toBeVisible()
  await expect(page.getByTestId('status-matrix')).toBeVisible()

  await page.waitForTimeout(1500)
})

test('status tab shows N/A for environments without HelmRelease', async ({ page }) => {
  await page.route('**/openid-configuration', () => {})

  await page.addInitScript(
    ({ key, value }) => sessionStorage.setItem(key, JSON.stringify(value)),
    { key: SESSION_KEY, value: fakeAdminUser },
  )

  const naStatus = {
    workloads: { api: { dev: 'v1.0.0', production: 'N/A' } },
    fetched_at: new Date().toISOString(),
    stale: false,
  }

  await page.route('**/api/v1/**', async (route) => {
    const url = route.request().url()
    if (url.includes('/api/v1/products') && !url.includes('/platform-api')) {
      await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify([fakeProduct]) })
      return
    }
    if (url.includes('/platform-api/status')) {
      await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(naStatus) })
      return
    }
    await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify([]) })
  })

  await page.goto('/')
  await page.getByText('Platform API').click()
  await page.getByRole('button', { name: 'Status' }).click()
  await expect(page.getByTestId('tag-api-production')).toHaveText('N/A')

  await page.waitForTimeout(1500)
})

test('status tab shows error message on fetch failure', async ({ page }) => {
  await page.route('**/openid-configuration', () => {})

  await page.addInitScript(
    ({ key, value }) => sessionStorage.setItem(key, JSON.stringify(value)),
    { key: SESSION_KEY, value: fakeAdminUser },
  )

  await page.route('**/api/v1/**', async (route) => {
    const url = route.request().url()
    if (url.includes('/api/v1/products') && !url.includes('/platform-api')) {
      await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify([fakeProduct]) })
      return
    }
    if (url.includes('/platform-api/status')) {
      await route.fulfill({ status: 500, contentType: 'application/json', body: JSON.stringify({ error: 'internal error' }) })
      return
    }
    await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify([]) })
  })

  await page.goto('/')
  await page.getByText('Platform API').click()
  await page.getByRole('button', { name: 'Status' }).click()
  await expect(page.getByTestId('status-error')).toBeVisible()
  await expect(page.getByTestId('status-retry')).toBeVisible()

  await page.waitForTimeout(1500)
})
