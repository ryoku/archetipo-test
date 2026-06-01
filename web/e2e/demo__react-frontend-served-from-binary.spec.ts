import { test, expect } from '@playwright/test'

// Demo scenario for US-004: verifies the built React app (dist/) serves routes
// correctly via `npm run serve` (sirv). The Go binary's embed.FS wiring and
// SPA-fallback handler are covered by unit tests in cmd/server/static_test.go.
// Video is recorded for this file only; other e2e tests use the global video:off default.
test.use({
  video: 'on',
  viewport: { width: 1280, height: 720 },
  launchOptions: { slowMo: 350 },
})

test('demo: built React app serves / and /login from dist/', async ({ page }) => {
  // Start from the root — the entry point described in Demonstrates.
  const response = await page.goto('/')
  expect(response?.status()).toBe(200)

  // The React root element is present and the home page heading is visible.
  await expect(page.locator('#root')).toBeVisible()
  await expect(page.locator('h1')).toHaveText('Home')

  // Navigate to /login — SPA fallback must serve index.html, not a 404.
  await page.goto('/login')
  await expect(page.locator('h1')).toHaveText('Login')

  // Hold the final state visible so the recording captures the outcome.
  await expect(page.locator('#root')).toBeVisible()
  await page.waitForTimeout(1500)
})
