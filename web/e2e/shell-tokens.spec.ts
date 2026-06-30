import { test, expect } from '@playwright/test'

/**
 * US-051: Verify design tokens are applied to the app shell.
 * Checks that CSS variables from tokens.css resolve correctly on the page.
 * Does not require authentication — token verification happens at the CSS level.
 *
 * Note: browsers may compress hex shorthand (#CC0000 → #c00), so comparisons use
 * a helper that converts to RGB for reliable cross-browser equality.
 */

function hexToRgb(hex: string): string {
  const h = hex.replace('#', '')
  const r = parseInt(h.length === 3 ? h[0] + h[0] : h.slice(0, 2), 16)
  const g = parseInt(h.length === 3 ? h[1] + h[1] : h.slice(2, 4), 16)
  const b = parseInt(h.length === 3 ? h[2] + h[2] : h.slice(4, 6), 16)
  return `rgb(${r}, ${g}, ${b})`
}

async function getTokenAsRgb(page: import('@playwright/test').Page, token: string): Promise<string> {
  const raw = await page.evaluate((t: string) => {
    const el = document.createElement('div')
    el.style.color = `var(${t})`
    document.body.appendChild(el)
    const computed = getComputedStyle(el).color
    document.body.removeChild(el)
    return computed
  }, token)
  return raw
}

test.describe('US-051: Shell light theme — design token verification', () => {
  test('tokens.css defines brand palette variables', async ({ page }) => {
    await page.goto('/')

    const primary = await getTokenAsRgb(page, '--primary')
    const brandBlack = await getTokenAsRgb(page, '--brand-black')
    const primaryHover = await getTokenAsRgb(page, '--primary-hover')
    const danger = await getTokenAsRgb(page, '--danger')

    expect(primary).toBe(hexToRgb('#CC0000'))
    expect(brandBlack).toBe(hexToRgb('#0A0A0A'))
    expect(primaryHover).toBe(hexToRgb('#AA0000'))
    expect(danger).toBe(hexToRgb('#8B0000'))
  })

  test('surface and background tokens are defined', async ({ page }) => {
    await page.goto('/')

    const bgPage = await getTokenAsRgb(page, '--bg-page')
    const bgCard = await getTokenAsRgb(page, '--bg-card')
    const surfaceRaised = await getTokenAsRgb(page, '--surface-raised')

    expect(bgPage).toBe(hexToRgb('#FBFBFB'))
    expect(bgCard).toBe(hexToRgb('#FFFFFF'))
    expect(surfaceRaised).toBe(hexToRgb('#F5F5F5'))
  })

  test('status and environment tokens are defined', async ({ page }) => {
    await page.goto('/')

    const statusSuccess = await getTokenAsRgb(page, '--status-success')
    const statusWarning = await getTokenAsRgb(page, '--status-warning')
    const envProd = await getTokenAsRgb(page, '--env-prod')
    const envQa = await getTokenAsRgb(page, '--env-qa')
    const envDev = await getTokenAsRgb(page, '--env-dev')

    expect(statusSuccess).toBe(hexToRgb('#16A34A'))
    expect(statusWarning).toBe(hexToRgb('#D97706'))
    expect(envProd).toBe(hexToRgb('#0A0A0A'))
    expect(envQa).toBe(hexToRgb('#1D4ED8'))
    expect(envDev).toBe(hexToRgb('#6B7280'))
  })

  test('typography and shell dimension tokens are defined', async ({ page }) => {
    await page.goto('/')

    const fontUi = await page.evaluate(() =>
      getComputedStyle(document.documentElement).getPropertyValue('--font-ui').trim()
    )
    const railW = await page.evaluate(() =>
      getComputedStyle(document.documentElement).getPropertyValue('--rail-w').trim()
    )
    const sidebarW = await page.evaluate(() =>
      getComputedStyle(document.documentElement).getPropertyValue('--sidebar-w').trim()
    )

    expect(fontUi).toContain('Archivo')
    expect(railW).toBe('6px')
    expect(sidebarW).toBe('240px')
  })

  test('body uses token-based light background', async ({ page }) => {
    await page.goto('/')

    const bodyBg = await page.evaluate(() =>
      getComputedStyle(document.body).backgroundColor
    )
    // --bg-page: #FBFBFB = rgb(251, 251, 251)
    expect(bodyBg).toBe('rgb(251, 251, 251)')
  })

  test('no hardcoded dark background colors remain active', async ({ page }) => {
    await page.goto('/')

    const bodyBg = await page.evaluate(() =>
      getComputedStyle(document.body).backgroundColor
    )
    // Old dark-theme background #070a12 = rgb(7, 10, 18)
    expect(bodyBg).not.toBe('rgb(7, 10, 18)')
  })
})
