import { test, expect } from '@playwright/test'

/**
 * US-051 Demo scenario
 *
 * Demonstrates: a developer loads any authenticated page and sees the new shell —
 * brand palette tokens resolved from CSS variables, no hardcoded hex values.
 */
test.use({
  video: 'on',
  launchOptions: { slowMo: 350 },
  viewport: { width: 1280, height: 720 },
})

function hexToRgb(hex: string): string {
  const h = hex.replace('#', '')
  const r = parseInt(h.length === 3 ? h[0] + h[0] : h.slice(0, 2), 16)
  const g = parseInt(h.length === 3 ? h[1] + h[1] : h.slice(2, 4), 16)
  const b = parseInt(h.length === 3 ? h[2] + h[2] : h.slice(4, 6), 16)
  return `rgb(${r}, ${g}, ${b})`
}

async function getTokenAsRgb(page: import('@playwright/test').Page, token: string): Promise<string> {
  return page.evaluate((t: string) => {
    const el = document.createElement('div')
    el.style.color = `var(${t})`
    document.body.appendChild(el)
    const computed = getComputedStyle(el).color
    document.body.removeChild(el)
    return computed
  }, token)
}

test('demo: shell light theme — brand palette tokens active in browser', async ({ page }) => {
  // Start from a clean state
  await page.goto('/')

  // Verify brand primary token: #CC0000 (red brand accent)
  const primaryColor = await getTokenAsRgb(page, '--primary')
  expect(primaryColor).toBe(hexToRgb('#CC0000'))

  // Verify brand black token: #0A0A0A (near-black structure)
  const brandBlack = await getTokenAsRgb(page, '--brand-black')
  expect(brandBlack).toBe(hexToRgb('#0A0A0A'))

  // Verify body background is the light page background (#FBFBFB)
  const bodyBg = await page.evaluate(() =>
    getComputedStyle(document.body).backgroundColor
  )
  expect(bodyBg).toBe('rgb(251, 251, 251)')

  // Verify no dark-mode background (#070a12 = rgb(7,10,18)) is active
  expect(bodyBg).not.toBe('rgb(7, 10, 18)')

  // Verify environment tokens for the badge system
  const envProd = await getTokenAsRgb(page, '--env-prod')
  expect(envProd).toBe(hexToRgb('#0A0A0A'))

  const envQa = await getTokenAsRgb(page, '--env-qa')
  expect(envQa).toBe(hexToRgb('#1D4ED8'))

  const envDev = await getTokenAsRgb(page, '--env-dev')
  expect(envDev).toBe(hexToRgb('#6B7280'))

  // Hold final state visible for at least 1.5s for the video recording
  await expect(page.locator('body')).toBeVisible()
  await page.waitForTimeout(1500)
})
