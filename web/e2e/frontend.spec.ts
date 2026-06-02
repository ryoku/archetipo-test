import { test, expect } from '@playwright/test'

const OIDC_AUTHORITY = 'http://localhost:8080/realms/kubegate'
const SESSION_KEY = `oidc.user:${OIDC_AUTHORITY}:kubegate-frontend`
const fakeUser = {
  profile: { sub: 'test-user-1', name: 'Marco Rossi', email: 'marco@example.com' },
  session_state: null,
  access_token: 'fake-access-token',
  token_type: 'Bearer',
  scope: 'openid profile email',
  expires_at: 9999999999,
  id_token: 'fake-id-token',
}

test('root URL serves React app with 200 and HTML root element', async ({ page }) => {
  // Inject authenticated user so ProtectedRoute renders HomePage instead of redirecting.
  await page.addInitScript(({ key, value }) => sessionStorage.setItem(key, JSON.stringify(value)), {
    key: SESSION_KEY,
    value: fakeUser,
  })

  const response = await page.goto('/')
  expect(response?.status()).toBe(200)

  const contentType = response?.headers()['content-type'] ?? ''
  expect(contentType).toContain('text/html')

  await expect(page.locator('#root')).toBeVisible()
})

test('React Router /login route renders without 404', async ({ page }) => {
  // Stall OIDC discovery to keep LoginPage visible (no network errors, no navigation away).
  await page.route(`${OIDC_AUTHORITY}/.well-known/openid-configuration`, () => {})

  const response = await page.goto('/login')
  expect(response?.status()).toBe(200)

  const contentType = response?.headers()['content-type'] ?? ''
  expect(contentType).toContain('text/html')

  await expect(page.locator('#root')).toBeVisible()
  await expect(page.locator('p')).toContainText('Redirecting to login')
})
