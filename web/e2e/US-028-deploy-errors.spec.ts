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

const fakeViewerUser = {
  profile: { sub: 'test-viewer-1', name: 'Luca Conti', email: 'luca@example.com' },
  session_state: null,
  access_token: 'fake-viewer-token',
  refresh_token: null,
  token_type: 'Bearer',
  scope: 'openid profile email',
  expires_at: 9999999999,
  id_token: 'fake-id-token',
}

const fakeProductEditor = {
  id: 'prod-1', name: 'Platform API', slug: 'platform-api',
  description: 'Core platform API service', created_at: '2026-01-01T00:00:00Z', my_role: 'editor',
}

const fakeProductViewer = { ...fakeProductEditor, my_role: 'viewer' }

const fakeEnvironment = {
  id: 'env-1', product_id: 'prod-1', name: 'production', slug: 'production',
  type: 'production', gitops_path: 'apps/production/platform-api/platform-api-helmrelease.yaml',
  created_at: '2026-01-01T00:00:00Z',
}

const fakeWorkload = {
  name: 'api',
  image_repository: 'europe-west1-docker.pkg.dev/acme/platform/api',
}

const fakeTags = {
  tags: [{ name: 'v1.14.2-rc.1', digest: 'sha256:abc', pushed_at: '2026-06-09T10:00:00Z' }],
  next_page_token: '',
}

async function setupPage(
  page: import('@playwright/test').Page,
  user: object,
  product: object,
  deployResponse: { status: number; body: object },
) {
  await page.route('**/openid-configuration', () => {})
  await page.addInitScript(
    ({ key, value }) => sessionStorage.setItem(key, JSON.stringify(value)),
    { key: SESSION_KEY, value: user },
  )
  await page.route('**/api/v1/**', async (route) => {
    const url = route.request().url()
    const method = route.request().method()
    if (url.endsWith('/api/v1/products') && method === 'GET') {
      await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify([product]) })
    } else if (url.includes('/environments') && !url.includes('/workloads') && method === 'GET') {
      await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify([fakeEnvironment]) })
    } else if (url.includes('/workloads') && !url.includes('/tags') && method === 'GET') {
      await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify([fakeWorkload]) })
    } else if (url.includes('/workloads/api/tags') && method === 'GET') {
      await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(fakeTags) })
    } else if (url.includes('/deployments') && method === 'POST') {
      await route.fulfill({ status: deployResponse.status, contentType: 'application/json', body: JSON.stringify(deployResponse.body) })
    } else {
      await route.continue()
    }
  })
}

async function openDeployDialog(page: import('@playwright/test').Page) {
  await page.goto('/')
  await expect(page.locator('.home-p-name', { hasText: 'Platform API' })).toBeVisible()
  await page.locator('.home-p-name', { hasText: 'Platform API' }).click()
  await expect(page.getByRole('button', { name: /Deploy api/i })).toBeVisible()
  await page.getByRole('button', { name: /Deploy api/i }).click()
  await expect(page.getByRole('dialog')).toBeVisible()
  await expect(page.getByText('v1.14.2-rc.1')).toBeVisible()
  await page.getByText('v1.14.2-rc.1').click()
  await expect(page.locator('.dd-btn-deploy')).toBeEnabled()
}

test('deploy-409: modal stays open and shows conflict banner when lock is held', async ({ page }) => {
  await setupPage(page, fakeEditorUser, fakeProductEditor, {
    status: 409,
    body: { lock_holder: 'alice', locked_since: '2026-06-16T10:00:00Z' },
  })

  await openDeployDialog(page)
  await page.locator('.dd-btn-deploy').click()

  // Dialog stays open
  await expect(page.getByRole('dialog')).toBeVisible()

  // Conflict banner visible with lock holder name
  await expect(page.locator('.dd-deploy-error-banner--conflict')).toBeVisible()
  await expect(page.locator('.dd-deploy-error-banner--conflict')).toContainText('alice')
})

test('deploy-422: modal stays open and shows tag convention banner', async ({ page }) => {
  await setupPage(page, fakeEditorUser, fakeProductEditor, {
    status: 422,
    body: {
      rejected_tag: 'v1.14.2-rc.1',
      applied_regex: String.raw`^v\d+\.\d+\.\d+$`,
      message: 'Tag non conforme alla tag convention',
    },
  })

  await openDeployDialog(page)
  await page.locator('.dd-btn-deploy').click()

  // Dialog stays open
  await expect(page.getByRole('dialog')).toBeVisible()

  // Tag convention banner visible with rejection message
  await expect(page.locator('.dd-deploy-error-banner--tag-error')).toBeVisible()
  await expect(page.locator('.dd-deploy-error-banner--tag-error')).toContainText('Tag non conforme alla tag convention')
})

test('viewer-no-deploy: viewer role sees no Deploy button in the workloads table', async ({ page }) => {
  await setupPage(page, fakeViewerUser, fakeProductViewer, {
    status: 200,
    body: {},
  })

  await page.goto('/')
  await expect(page.locator('.home-p-name', { hasText: 'Platform API' })).toBeVisible()
  await page.locator('.home-p-name', { hasText: 'Platform API' }).click()

  // Workload row appears
  await expect(page.locator('.pd-comp-name-str', { hasText: 'api' })).toBeVisible()

  // No deploy button for viewer
  await expect(page.getByRole('button', { name: /Deploy api/i })).not.toBeVisible()
})
