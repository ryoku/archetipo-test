import { test, expect } from '@playwright/test'

// Match the OIDC config baked into the SPA build.
const OIDC_AUTHORITY = 'http://localhost:8080/realms/kubegate'
const OIDC_CLIENT_ID = 'kubegate-frontend'
const SESSION_KEY = `oidc.user:${OIDC_AUTHORITY}:${OIDC_CLIENT_ID}`

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

const fakeViewerUser = {
  profile: { sub: 'test-viewer-1', name: 'Marco Verdi', email: 'marco@example.com' },
  session_state: null,
  access_token: 'fake-viewer-token',
  refresh_token: null,
  token_type: 'Bearer',
  scope: 'openid profile email',
  expires_at: 9999999999,
  id_token: 'fake-id-token-viewer',
}

const fakeAdminProduct = {
  id: 'prod-checkout',
  name: 'Checkout Service',
  slug: 'checkout-service',
  description: 'Payment and order checkout pipeline',
  created_at: '2026-01-15T00:00:00Z',
  my_role: 'admin',
}

const fakeViewerProduct = {
  ...fakeAdminProduct,
  my_role: 'viewer',
}

const defaultTagConvention = {
  regex: String.raw`^v\d+\.\d+\.\d+$`,
  source: 'default',
}

const productTagConvention = {
  regex: '^release-.*$',
  source: 'product',
}

// Demo scenario — video on for this file only; global config keeps video: 'off'.
test.describe('demo: DevOps Admin sets a product-level tag convention override', () => {
  test.use({
    video: 'on',
    viewport: { width: 1280, height: 720 },
    launchOptions: { slowMo: 300 },
  })

  test('demo: admin navigates to Settings, views default regex, sets product override', async ({ page }) => {
    // Stall OIDC discovery so the LoginPage stays visible during initial navigation.
    await page.route(`${OIDC_AUTHORITY}/.well-known/openid-configuration`, () => {
      // Never fulfilled — keeps the redirect spinner visible on screen.
    })

    // Step 1: visit / unauthenticated → ProtectedRoute redirects to /login.
    await page.goto('/')
    await expect(page).toHaveURL(/\/login/)
    await page.waitForTimeout(600)

    // Step 2: inject OIDC session (admin role) and set up API mocks.
    await page.evaluate(
      ({ key, value }) => sessionStorage.setItem(key, JSON.stringify(value)),
      { key: SESSION_KEY, value: fakeAdminUser },
    )

    // Mock all API calls for the demo flow.
    await page.route('**/api/v1/**', async (route) => {
      const url = route.request().url()
      const method = route.request().method()

      if (url.endsWith('/api/v1/products') && method === 'GET') {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify([fakeAdminProduct]),
        })
      } else if (url.endsWith('/api/v1/products/checkout-service/tag-convention') && method === 'GET') {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify(defaultTagConvention),
        })
      } else if (url.endsWith('/api/v1/products/checkout-service/tag-convention') && method === 'PUT') {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify(productTagConvention),
        })
      } else {
        await route.continue()
      }
    })

    // Step 3: navigate to / — AuthProvider reads user from sessionStorage, home shows product list.
    await page.goto('/')
    await expect(page.getByText('Checkout Service')).toBeVisible()
    await expect(page.getByText('checkout-service')).toBeVisible()
    await page.waitForTimeout(800)

    // Step 4: click the product card → product detail page loads.
    await page.getByText('Checkout Service').click()
    await expect(page).toHaveURL(/\/products\/checkout-service/)
    await expect(page.getByText('Registered Components')).toBeVisible()
    await page.waitForTimeout(700)

    // Step 5: click "Settings" in the sub-nav → ProductSettingsPage loads.
    await page.getByRole('button', { name: 'Settings' }).click()
    await expect(page).toHaveURL(/\/products\/checkout-service\/settings/)
    await expect(page.getByText('Tag Convention')).toBeVisible()
    await page.waitForTimeout(600)

    // Step 6: verify the global default badge and the default regex are visible.
    await expect(page.locator('.pd-source-badge--default')).toBeVisible()
    await expect(page.locator('.pd-source-badge--default')).toContainText('global default')
    await expect(page.locator('.pd-regex-value')).toContainText(String.raw`^v\d+\.\d+\.\d+$`)
    await page.waitForTimeout(700)

    // Step 7: click Edit → edit form appears with current regex pre-filled.
    await page.getByRole('button', { name: 'Edit' }).click()
    await expect(page.locator('#tag-convention-regex')).toBeVisible()
    await expect(page.locator('#tag-convention-regex')).toHaveValue(String.raw`^v\d+\.\d+\.\d+$`)
    await page.waitForTimeout(500)

    // Step 8: clear the field and type the new product-level regex.
    // fill() selects-all and replaces, so no separate clear step is needed.
    await page.locator('#tag-convention-regex').fill('^release-.*$')
    await page.waitForTimeout(600)

    // Step 9: click Save → PUT is called, product override badge appears with new regex.
    await page.getByRole('button', { name: 'Save' }).click()
    await expect(page.locator('.pd-source-badge--product')).toBeVisible()
    await expect(page.locator('.pd-source-badge--product')).toContainText('product override')
    await expect(page.locator('.pd-regex-value')).toContainText('^release-.*$')
    await page.waitForTimeout(900)

    // Hold the final state so the recording captures the confirmed override.
    await page.waitForTimeout(1500)
  })
})

test.describe('Viewer sees read-only settings', () => {
  // No video for this non-demo scenario.

  test('viewer: no Edit button visible on Settings page', async ({ page }) => {
    // Stall OIDC discovery so the LoginPage stays visible during initial navigation.
    await page.route(`${OIDC_AUTHORITY}/.well-known/openid-configuration`, () => {
      // Never fulfilled.
    })

    // Step 1: visit / unauthenticated → redirect to /login.
    await page.goto('/')
    await expect(page).toHaveURL(/\/login/)

    // Step 2: inject OIDC session as viewer.
    await page.evaluate(
      ({ key, value }) => sessionStorage.setItem(key, JSON.stringify(value)),
      { key: SESSION_KEY, value: fakeViewerUser },
    )

    // Mock API calls.
    await page.route('**/api/v1/**', async (route) => {
      const url = route.request().url()
      const method = route.request().method()

      if (url.endsWith('/api/v1/products') && method === 'GET') {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify([fakeViewerProduct]),
        })
      } else if (url.endsWith('/api/v1/products/checkout-service/tag-convention') && method === 'GET') {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify(defaultTagConvention),
        })
      } else {
        await route.continue()
      }
    })

    // Step 3: navigate to / — product list loads.
    await page.goto('/')
    await expect(page.getByText('Checkout Service')).toBeVisible()

    // Step 4: navigate directly to the settings page, passing product state via navigation.
    await page.getByText('Checkout Service').click()
    await expect(page).toHaveURL(/\/products\/checkout-service/)
    await page.getByRole('button', { name: 'Settings' }).click()
    await expect(page).toHaveURL(/\/products\/checkout-service\/settings/)

    // Step 5: tag convention section renders with global default badge.
    await expect(page.getByText('Tag Convention')).toBeVisible()
    await expect(page.locator('.pd-source-badge--default')).toBeVisible()
    await expect(page.locator('.pd-regex-value')).toContainText(String.raw`^v\d+\.\d+\.\d+$`)

    // Step 6: Edit button must NOT be present for a viewer.
    await expect(page.getByRole('button', { name: 'Edit' })).not.toBeVisible()
  })
})
