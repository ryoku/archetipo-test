import { test, expect } from '@playwright/test'

const OIDC_AUTHORITY = 'http://localhost:8080/realms/kubegate'
const OIDC_CLIENT_ID = 'kubegate-frontend'
const SESSION_KEY = `oidc.user:${OIDC_AUTHORITY}:${OIDC_CLIENT_ID}`

const DEVOPS_ADMIN_PAYLOAD = btoa(
  JSON.stringify({
    sub: 'admin-user-1',
    name: 'Sara Bianchi',
    email: 'sara@example.com',
    realm_access: { roles: ['kubegate:devops-admin'] },
    exp: 9999999999,
  }),
)
  .replaceAll('+', '-')
  .replaceAll('/', '_')
  .replaceAll(/=+$/g, '')

const DEVOPS_ADMIN_TOKEN = `eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.${DEVOPS_ADMIN_PAYLOAD}.fake-sig`

const VIEWER_PAYLOAD = btoa(
  JSON.stringify({
    sub: 'viewer-user-1',
    name: 'Marco Rossi',
    email: 'marco@example.com',
    realm_access: { roles: ['kubegate:viewer'] },
    exp: 9999999999,
  }),
)
  .replaceAll('+', '-')
  .replaceAll('/', '_')
  .replaceAll(/=+$/g, '')

const VIEWER_TOKEN = `eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.${VIEWER_PAYLOAD}.fake-sig`

function makeAdminSession(name: string, accessToken: string) {
  return {
    profile: { sub: 'admin-user-1', name, email: 'sara@example.com' },
    session_state: null,
    access_token: accessToken,
    refresh_token: null,
    token_type: 'Bearer',
    scope: 'openid profile email',
    expires_at: 9999999999,
    id_token: 'fake-id-token',
  }
}

function makeViewerSession(name: string, accessToken: string) {
  return {
    profile: { sub: 'viewer-user-1', name, email: 'marco@example.com' },
    session_state: null,
    access_token: accessToken,
    refresh_token: null,
    token_type: 'Bearer',
    scope: 'openid profile email',
    expires_at: 9999999999,
    id_token: 'fake-id-token',
  }
}

test.describe('US-035 — Admin dashboard', () => {
  test('non-admin user navigating to /admin is redirected to /', async ({ page }) => {
    await page.addInitScript(
      ({ key, value }) => sessionStorage.setItem(key, JSON.stringify(value)),
      { key: SESSION_KEY, value: makeViewerSession('Marco Rossi', VIEWER_TOKEN) },
    )

    await page.route('**/api/v1/products', async (route) => {
      if (route.request().method() === 'GET') {
        await route.fulfill({ status: 200, contentType: 'application/json', body: '[]' })
      } else {
        await route.continue()
      }
    })

    await page.goto('/admin')
    await expect(page).not.toHaveURL(/\/admin/)
    await expect(page.getByTestId('empty-state')).toBeVisible()
  })

  test('shows empty state when no products exist', async ({ page }) => {
    await page.addInitScript(
      ({ key, value }) => sessionStorage.setItem(key, JSON.stringify(value)),
      { key: SESSION_KEY, value: makeAdminSession('Sara Bianchi', DEVOPS_ADMIN_TOKEN) },
    )

    await page.route('**/api/v1/admin/products', async (route) => {
      await route.fulfill({ status: 200, contentType: 'application/json', body: '[]' })
    })

    await page.goto('/admin')
    await expect(page.getByTestId('empty-state')).toBeVisible()
  })
})
