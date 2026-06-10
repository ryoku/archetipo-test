import { test, expect } from '@playwright/test'

// In the dist build (no .env file), VITE_OIDC_ISSUER_URL and VITE_OIDC_CLIENT_ID
// are undefined, so oidc-client-ts constructs the sessionStorage key as:
// oidc.user:undefined:undefined
const SESSION_KEY = 'oidc.user:undefined:undefined'

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

const fakeComponent = {
  id: 'comp-1',
  name: 'api',
  slug: 'api',
  gcr_image_path: 'europe-west1-docker.pkg.dev/acme/platform/api',
  product_id: 'prod-1',
  created_at: '2026-01-01T00:00:00Z',
}

const allTags = {
  tags: [
    { name: 'v1.14.2', digest: 'sha256:abc', pushed_at: '2026-06-09T10:00:00Z' },
    { name: 'v1.14.1', digest: 'sha256:def', pushed_at: '2026-06-07T10:00:00Z' },
    { name: 'main-20260501', digest: 'sha256:ghi', pushed_at: '2026-05-01T10:00:00Z' },
  ],
  next_page_token: '',
}

const filteredTags = {
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

test('demo: user opens deploy dialog, filters tags, selects a tag', async ({ page }) => {
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

    if (url.includes('/api/v1/products/platform-api/components/api/tags')) {
      // Tags endpoint — respond based on filter query param.
      const parsedUrl = new URL(url)
      const filter = parsedUrl.searchParams.get('filter')
      if (filter) {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify(filteredTags),
        })
      } else {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify(allTags),
        })
      }
    } else if (url.includes('/api/v1/products/platform-api/components') && method === 'GET') {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify([fakeComponent]),
      })
    } else if (url.includes('/api/v1/products/platform-api/tag-convention') && method === 'GET') {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ regex: '^v.*$', source: 'default' }),
      })
    } else if (url.includes('/api/v1/products/platform-api/environments') && method === 'GET') {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify([]),
      })
    } else if (/\/api\/v1\/products$/.test(url) && method === 'GET') {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify([fakeProduct]),
      })
    } else {
      await route.continue()
    }
  })

  // Step 1: visit / unauthenticated → ProtectedRoute redirects to /login.
  // The addInitScript above seeds sessionStorage before React boots.
  // On the initial goto('/'), the user is already in sessionStorage so ProtectedRoute
  // renders the HomePage immediately.
  await page.goto('/')
  // Wait for the loading state to resolve and product list to appear.
  // Use CSS class to disambiguate the product name from the description.
  await expect(page.locator('.home-p-name', { hasText: 'Platform API' })).toBeVisible()
  await expect(page.locator('.home-p-slug', { hasText: 'platform-api' })).toBeVisible()
  await page.waitForTimeout(800)

  // Step 2: click the product card → product detail page loads with components.
  await page.locator('.home-p-name', { hasText: 'Platform API' }).click()
  await expect(page).toHaveURL(/\/products\/platform-api/)
  await expect(page.getByText('Registered Components')).toBeVisible()
  await page.waitForTimeout(700)

  // Step 3: wait for the component row with the Deploy button to appear.
  await expect(page.getByRole('button', { name: /Deploy api/i })).toBeVisible()
  await page.waitForTimeout(500)

  // Step 4: click the Deploy button for the 'api' component → Deploy dialog opens.
  await page.getByRole('button', { name: /Deploy api/i }).click()
  await expect(page.getByRole('dialog')).toBeVisible()
  await page.waitForTimeout(600)

  // Step 5: assert all tags are visible in the dialog.
  await expect(page.getByText('v1.14.2')).toBeVisible()
  await expect(page.getByText('v1.14.1')).toBeVisible()
  await expect(page.getByText('main-20260501')).toBeVisible()
  await page.waitForTimeout(500)

  // Step 6: type 'v1' in the filter input → wait for debounce.
  await page.locator('.dd-filter-input').fill('v1')
  await page.waitForTimeout(400)

  // Step 7: assert that 'main-20260501' is no longer visible and 'v1.14.2' is still visible.
  await expect(page.getByText('main-20260501')).not.toBeVisible()
  await expect(page.getByText('v1.14.2')).toBeVisible()
  await page.waitForTimeout(500)

  // Step 8: click on v1.14.1 tag row → tag is selected.
  await page.getByText('v1.14.1').click()
  await page.waitForTimeout(400)

  // Step 9: assert the Deploy button in the footer is enabled (not disabled).
  await expect(page.locator('.dd-btn-deploy')).toBeEnabled()
  // Assert the selected tag chip is shown in the footer.
  await expect(page.locator('.dd-selected-tag-chip')).toContainText('v1.14.1')
  await page.waitForTimeout(600)

  // Hold the final state so the recording captures the confirmed selection.
  await page.waitForTimeout(1500)
})
