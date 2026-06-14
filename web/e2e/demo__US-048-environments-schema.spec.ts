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
  name: 'staging',
  slug: 'staging',
  type: 'integration',
  gitops_path: 'apps/staging/platform-api/platform-api-helmrelease.yaml',
  created_at: '2026-06-13T10:00:00Z',
}

// Demo scenario — video on for this file only; global config keeps video: 'off'.
test.use({
  video: 'on',
  viewport: { width: 1280, height: 720 },
  launchOptions: { slowMo: 300 },
})

test('demo: environment form has slug input with live GitOps path preview, no overlay path field', async ({ page }) => {
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

  await page.route('**/api/v1/**', async (route) => {
    const url = route.request().url()
    const method = route.request().method()

    if (url.endsWith('/api/v1/products') && method === 'GET') {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify([fakeProduct]),
      })
    } else if (url.endsWith('/api/v1/products/platform-api/environments') && method === 'GET') {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify([]),
      })
    } else if (url.endsWith('/api/v1/products/platform-api/environments') && method === 'POST') {
      // Verify the request body does not contain overlay_path.
      const body = route.request().postDataJSON() as Record<string, unknown>
      if ('overlay_path' in body) {
        await route.fulfill({ status: 400, contentType: 'application/json', body: JSON.stringify({ error: 'unknown field' }) })
        return
      }
      await route.fulfill({
        status: 201,
        contentType: 'application/json',
        body: JSON.stringify(fakeEnvironment),
      })
    } else {
      await route.continue()
    }
  })

  // Step 3: navigate to / — AuthProvider reads user from sessionStorage.
  await page.goto('/')
  await expect(page.getByText('Platform API')).toBeVisible()
  await page.waitForTimeout(800)

  // Step 4: click the product card → product detail page.
  await page.getByText('Platform API').click()
  await expect(page).toHaveURL(/\/products\/platform-api/)
  await page.waitForTimeout(700)

  // Step 5: navigate to Environments tab.
  await page.getByRole('button', { name: 'Environments' }).click()
  await expect(page).toHaveURL(/\/products\/platform-api\/environments/)
  await expect(page.getByText('Deployment Environments')).toBeVisible()
  await page.waitForTimeout(600)

  // Step 6: open the Add Environment form.
  await page.getByRole('button', { name: 'Add Environment' }).click()
  await expect(page.getByText('New Environment')).toBeVisible()
  await page.waitForTimeout(400)

  // Step 7: confirm overlay path input is absent and slug input is present.
  await expect(page.getByLabel('Slug *')).toBeVisible()
  await expect(page.getByLabel('Overlay Path *')).not.toBeVisible()
  await page.waitForTimeout(500)

  // Step 8: type a name and observe slug auto-derive and live path preview.
  await page.getByLabel('Name *').fill('My Staging')
  await page.waitForTimeout(600)
  const slugInput = page.getByLabel('Slug *')
  await expect(slugInput).toHaveValue('my-staging')
  await expect(page.getByTestId('gitops-path-preview')).toBeVisible()
  await expect(page.getByTestId('gitops-path-preview')).toContainText('apps/my-staging/platform-api/platform-api-helmrelease.yaml')
  await page.waitForTimeout(700)

  // Step 9: clear slug, type a custom slug, and observe path preview update.
  await slugInput.fill('staging')
  await page.waitForTimeout(600)
  await expect(page.getByTestId('gitops-path-preview')).toContainText('apps/staging/platform-api/platform-api-helmrelease.yaml')
  await page.waitForTimeout(500)

  // Step 10: select type and submit.
  await page.getByLabel('Type *').selectOption('integration')
  await page.getByLabel('Name *').fill('staging')
  await page.waitForTimeout(400)
  await page.getByRole('button', { name: 'Save Environment' }).click()

  // Step 11: new environment row shows GitOps Path column (not overlay path).
  await expect(page.locator('.pd-comp-name-str', { hasText: 'staging' })).toBeVisible()
  await expect(page.getByText('apps/staging/platform-api/platform-api-helmrelease.yaml')).toBeVisible()
  await expect(page.locator('.env-type-badge', { hasText: 'integration' })).toBeVisible()

  // Hold end state so the recording captures the final outcome.
  await page.waitForTimeout(1500)
})
