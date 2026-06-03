import { test, expect } from '@playwright/test'

// These constants must match VITE_OIDC_ISSUER_URL and VITE_OIDC_CLIENT_ID baked into the build.
const OIDC_AUTHORITY = 'http://localhost:8080/realms/kubegate'
const OIDC_CLIENT_ID = 'kubegate-frontend'
const SESSION_KEY = `oidc.user:${OIDC_AUTHORITY}:${OIDC_CLIENT_ID}`

const fakeUser = {
  profile: { sub: 'test-user-1', name: 'Marco Rossi', email: 'marco@example.com' },
  session_state: null,
  access_token: 'fake-access-token',
  refresh_token: null,
  token_type: 'Bearer',
  scope: 'openid profile email',
  expires_at: 9999999999,
  id_token: 'fake-id-token',
}

// Demo scenario — video on for this file only; global config keeps video: 'off'.
test.use({
  video: 'on',
  viewport: { width: 1280, height: 720 },
  launchOptions: { slowMo: 300 },
})

test('demo: OIDC login flow — unauthenticated redirect, post-login home page, persistent session', async ({
  page,
}) => {
  // Stall the OIDC discovery fetch so signinRedirect() hangs and the
  // LoginPage keeps showing "Redirecting to login…" for the video.
  await page.route(`${OIDC_AUTHORITY}/.well-known/openid-configuration`, () => {
    // Never fulfil — lets the redirect state stay visible on screen.
  })

  // Step 1: visit / unauthenticated — ProtectedRoute redirects to /login.
  await page.goto('/')
  await expect(page).toHaveURL(/\/login/)

  // LoginPage shows "Redirecting to login…" while signinRedirect waits for discovery.
  await expect(page.locator('p')).toContainText('Redirecting to login')
  await page.waitForTimeout(1200)

  // Step 2: simulate successful login by injecting the user into sessionStorage.
  await page.evaluate(
    ({ key, value }) => sessionStorage.setItem(key, JSON.stringify(value)),
    { key: SESSION_KEY, value: fakeUser },
  )

  // Navigate to / — AuthProvider reads user from sessionStorage, ProtectedRoute passes.
  await page.goto('/')
  await expect(page.getByText('Marco Rossi')).toBeVisible()
  await expect(page.getByRole('button', { name: 'Logout' })).toBeVisible()
  await page.waitForTimeout(900)

  // Step 3: refresh — sessionStorage persists, user stays logged in.
  await page.reload()
  await expect(page.getByText('Marco Rossi')).toBeVisible()
  await expect(page.getByRole('button', { name: 'Logout' })).toBeVisible()

  // Hold end state so the recording captures the final outcome.
  await page.waitForTimeout(1500)
})
