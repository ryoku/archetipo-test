import { test, expect } from '@playwright/test'

// Demo scenario for US-004: shows the built server binary serving the React app.
// Video is recorded for this file only; other e2e tests use the global video:off default.
test.use({
  video: 'on',
  viewport: { width: 1280, height: 720 },
  launchOptions: { slowMo: 350 },
})

test('demo: make build produces a binary that serves the React app', async ({ page }) => {
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
