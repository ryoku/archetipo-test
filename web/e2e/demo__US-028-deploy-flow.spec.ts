import { test, expect } from '@playwright/test'

// oidc-client-ts constructs the sessionStorage key from VITE_OIDC_ISSUER_URL and VITE_OIDC_CLIENT_ID.
const SESSION_KEY = 'oidc.user:http://localhost:8080/realms/kubegate:kubegate-frontend'

const fakeEditorUser = {
  profile: { sub: 'test-editor-1', name: 'Marco Andreoli', email: 'marco@example.com' },
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
  my_role: 'editor',
}

const fakeEnvironment = {
  id: 'env-1',
  product_id: 'prod-1',
  name: 'production',
  slug: 'production',
  type: 'production',
  gitops_path: 'apps/production/platform-api/platform-api-helmrelease.yaml',
  created_at: '2026-01-01T00:00:00Z',
}

const fakeWorkload = {
  name: 'api',
  image_repository: 'europe-west1-docker.pkg.dev/acme/platform/api',
}

const fakeTags = {
  tags: [
    { name: 'v1.14.2-rc.1', digest: 'sha256:abc', pushed_at: '2026-06-09T10:00:00Z' },
    { name: 'v1.14.1', digest: 'sha256:def', pushed_at: '2026-06-07T10:00:00Z' },
    { name: 'v1.14.0', digest: 'sha256:ghi', pushed_at: '2026-06-01T10:00:00Z' },
  ],
  next_page_token: '',
}

const fakeDeployResult = { deployment_id: 'gitops-commit-a1b2c3d' }

// Demo scenario — video on for this file only; global config keeps video: 'off'.
test.use({
  video: 'on',
  viewport: { width: 1280, height: 720 },
  launchOptions: { slowMo: 300 },
})

test('demo: user selects a tag, confirms deploy, sees success toast with commit SHA', async ({ page }) => {
  // Stall OIDC discovery so the library does not attempt network validation.
  await page.route('**/openid-configuration', () => {})

  // Inject authenticated editor session before React boots.
  await page.addInitScript(
    ({ key, value }) => sessionStorage.setItem(key, JSON.stringify(value)),
    { key: SESSION_KEY, value: fakeEditorUser },
  )

  // Set up API mocks for the full deploy flow.
  await page.route('**/api/v1/**', async (route) => {
    const url = route.request().url()
    const method = route.request().method()

    if (url.endsWith('/api/v1/products') && method === 'GET') {
      await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify([fakeProduct]) })
    } else if (url.includes('/api/v1/products/platform-api/environments') && !url.includes('/workloads') && method === 'GET') {
      await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify([fakeEnvironment]) })
    } else if (url.includes('/api/v1/products/platform-api/environments/env-1/workloads') && !url.includes('/tags') && method === 'GET') {
      await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify([fakeWorkload]) })
    } else if (url.includes('/workloads/api/tags') && method === 'GET') {
      await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(fakeTags) })
    } else if (url.includes('/environments/env-1/deployments') && method === 'POST') {
      await route.fulfill({ status: 202, contentType: 'application/json', body: JSON.stringify(fakeDeployResult) })
    } else {
      await route.continue()
    }
  })

  // Step 1: navigate to home — user is already authenticated via sessionStorage.
  await page.goto('/')
  await expect(page.locator('.home-p-name', { hasText: 'Platform API' })).toBeVisible()
  await page.waitForTimeout(800)

  // Step 2: click the product card → product detail page loads.
  await page.locator('.home-p-name', { hasText: 'Platform API' }).click()
  await expect(page).toHaveURL(/\/products\/platform-api/)
  await page.waitForTimeout(600)

  // Step 3: wait for the workload row with the Deploy button to appear.
  await expect(page.getByRole('button', { name: /Deploy api/i })).toBeVisible()
  await page.waitForTimeout(500)

  // Step 4: click the Deploy button → dialog opens.
  await page.getByRole('button', { name: /Deploy api/i }).click()
  await expect(page.getByRole('dialog')).toBeVisible()
  await page.waitForTimeout(600)

  // Step 5: wait for tags to load and assert the tag list is visible.
  await expect(page.getByText('v1.14.2-rc.1')).toBeVisible()
  await page.waitForTimeout(500)

  // Step 6: click on the tag v1.14.2-rc.1 to select it.
  await page.getByText('v1.14.2-rc.1').click()
  await page.waitForTimeout(400)

  // Step 7: assert the footer Deploy button is now enabled and the tag chip is shown.
  await expect(page.locator('.dd-btn-deploy')).toBeEnabled()
  await expect(page.locator('.dd-selected-tag-chip')).toContainText('v1.14.2-rc.1')
  await page.waitForTimeout(500)

  // Step 8: click Deploy — spinner appears, POST is sent, dialog closes.
  await page.locator('.dd-btn-deploy').click()

  // Step 9: the dialog closes and the success toast appears.
  await expect(page.getByRole('dialog')).not.toBeVisible()
  await expect(page.getByTestId('deploy-toast')).toBeVisible()
  await page.waitForTimeout(500)

  // Step 10: assert toast content — tag name and commit SHA.
  await expect(page.getByTestId('deploy-toast')).toContainText('v1.14.2-rc.1')
  await expect(page.getByTestId('deploy-toast')).toContainText('gitops-commit-a1b2c3d')

  // Hold the final state for the recording.
  await page.waitForTimeout(1500)
})
