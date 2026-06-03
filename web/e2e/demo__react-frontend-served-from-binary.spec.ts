import { test, expect } from '@playwright/test'

// Demo scenario for US-004: verifies the built React app (dist/) serves routes
// correctly via `pnpm serve` (sirv). The Go binary's embed.FS wiring and
// SPA-fallback handler are covered by unit tests in cmd/server/static_test.go.
// Video is recorded for this file only; other e2e tests use the global video:off default.
test.use({
  video: 'on',
  viewport: { width: 1280, height: 720 },
  launchOptions: { slowMo: 350 },
})

const SESSION_KEY = 'oidc.user:http://localhost:8080/realms/kubegate:kubegate-frontend'
const fakeUser = {
  profile: { sub: 'test-user-1', name: 'Marco Rossi', email: 'marco@example.com' },
  session_state: null,
  access_token: 'fake-access-token',
  token_type: 'Bearer',
  scope: 'openid profile email',
  expires_at: 9999999999,
  id_token: 'fake-id-token',
}

test('demo: built React app serves / and /login from dist/', async ({ page }) => {
  // Inject authenticated user so ProtectedRoute renders the home page
  // without triggering the OIDC redirect (which would navigate away from the app).
  await page.addInitScript(({ key, value }) => sessionStorage.setItem(key, JSON.stringify(value)), {
    key: SESSION_KEY,
    value: fakeUser,
  })

  // Start from the root — the entry point described in Demonstrates.
  const response = await page.goto('/')
  expect(response?.status()).toBe(200)

  // The React root element is present and the home page renders with the user.
  await expect(page.locator('#root')).toBeVisible()
  await expect(page.getByText('Marco Rossi')).toBeVisible()

  // Navigate to /login — SPA fallback must serve index.html, not a 404.
  // With an authenticated user, LoginPage redirects back to / (client-side).
  const loginResponse = await page.goto('/login')
  expect(loginResponse?.status()).toBe(200)
  await expect(page.locator('#root')).toBeVisible()

  // Hold the final state visible so the recording captures the outcome.
  await page.waitForTimeout(1500)
})
