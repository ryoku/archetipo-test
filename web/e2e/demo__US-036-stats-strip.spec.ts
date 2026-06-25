import { test, expect } from '@playwright/test'

const OIDC_AUTHORITY = 'http://localhost:8080/realms/kubegate'
const OIDC_CLIENT_ID = 'kubegate-frontend'
const SESSION_KEY = `oidc.user:${OIDC_AUTHORITY}:${OIDC_CLIENT_ID}`

const USER_PAYLOAD = btoa(
  JSON.stringify({
    sub: 'user-1',
    name: 'Marco Andreoli',
    email: 'marco@example.com',
    realm_access: { roles: ['kubegate:devops-admin'] },
    exp: 9999999999,
  }),
)
  .replaceAll('+', '-')
  .replaceAll('/', '_')
  .replaceAll(/=+$/g, '')

const ACCESS_TOKEN = `eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.${USER_PAYLOAD}.fake-sig`

function makeSession(name: string, accessToken: string) {
  return {
    profile: { sub: 'user-1', name, email: 'marco@example.com' },
    session_state: null,
    access_token: accessToken,
    refresh_token: null,
    token_type: 'Bearer',
    scope: 'openid profile email',
    expires_at: 9999999999,
    id_token: 'fake-id-token',
  }
}

const STATS = {
  product_count: 4,
  environment_count: 11,
  component_count: 8,
  deployments_last_24h: 3,
}

const PRODUCTS = [
  {
    id: 'p-1',
    name: 'Platform API',
    slug: 'platform-api',
    description: 'Core platform',
    created_at: '2026-01-01T00:00:00Z',
  },
  {
    id: 'p-2',
    name: 'Customer App',
    slug: 'customer-app',
    description: 'Customer-facing app',
    created_at: '2026-02-01T00:00:00Z',
  },
]

// ─────────────────────────────────────────────────────────────────
// Demo scenario — video on; slowMo for readability
// ─────────────────────────────────────────────────────────────────
test.use({
  video: 'on',
  viewport: { width: 1280, height: 720 },
  launchOptions: { slowMo: 300 },
})

test('demo: Marco lands on the home page and sees 4 stat tiles above the product grid', async ({ page }) => {
  // Step 1: stall OIDC discovery so the login page is briefly visible
  await page.route(`${OIDC_AUTHORITY}/.well-known/openid-configuration`, () => {})

  // Step 2: navigate unauthenticated — should redirect to /login
  await page.goto('/')
  await expect(page).toHaveURL(/\/login/)
  await page.waitForTimeout(600)

  // Step 3: inject a valid session
  await page.evaluate(
    ({ key, value }) => sessionStorage.setItem(key, JSON.stringify(value)),
    { key: SESSION_KEY, value: makeSession('Marco Andreoli', ACCESS_TOKEN) },
  )

  // Step 4: mock API endpoints
  await page.route('**/api/v1/stats', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(STATS),
    })
  })
  await page.route('**/api/v1/products', async (route) => {
    if (route.request().method() === 'GET') {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(PRODUCTS),
      })
    } else {
      await route.continue()
    }
  })

  // Step 5: navigate to home — stats strip and product grid should load
  await page.goto('/')
  await expect(page.getByTestId('stats-strip')).toBeVisible()
  await page.waitForTimeout(800)

  // Step 6: assert all 4 stat tiles are visible with the mocked numbers
  const tiles = page.getByTestId('stat-tile')
  await expect(tiles).toHaveCount(4)
  await expect(page.getByText('4')).toBeVisible()
  await expect(page.getByText('11')).toBeVisible()
  await expect(page.getByText('8')).toBeVisible()
  await expect(page.getByText('3')).toBeVisible()
  await page.waitForTimeout(700)

  // Step 7: assert stats strip appears above the product grid
  const strip = page.getByTestId('stats-strip')
  const grid = page.locator('.home-product-grid')
  await expect(strip).toBeVisible()
  await expect(grid).toBeVisible()

  const stripBox = await strip.boundingBox()
  const gridBox = await grid.boundingBox()
  expect(stripBox!.y + stripBox!.height).toBeLessThan(gridBox!.y)
  await page.waitForTimeout(1500)
})

// ─────────────────────────────────────────────────────────────────
// Fallback scenario (no video)
// ─────────────────────────────────────────────────────────────────
test.describe('Stats fallback', () => {
  test.use({ video: 'off' })

  test('shows "--" in all tiles when stats API fails, product grid still loads', async ({ page }) => {
    await page.route(`${OIDC_AUTHORITY}/.well-known/openid-configuration`, () => {})
    await page.goto('/')
    await expect(page).toHaveURL(/\/login/)

    await page.evaluate(
      ({ key, value }) => sessionStorage.setItem(key, JSON.stringify(value)),
      { key: SESSION_KEY, value: makeSession('Marco Andreoli', ACCESS_TOKEN) },
    )

    // Stats API returns 500
    await page.route('**/api/v1/stats', async (route) => {
      await route.fulfill({ status: 500, contentType: 'application/json', body: '{"error":"internal error"}' })
    })
    await page.route('**/api/v1/products', async (route) => {
      if (route.request().method() === 'GET') {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify(PRODUCTS),
        })
      } else {
        await route.continue()
      }
    })

    await page.goto('/')
    await expect(page.getByTestId('stats-strip')).toBeVisible()

    // All 4 tiles show "--"
    const tiles = page.getByTestId('stat-tile')
    await expect(tiles).toHaveCount(4)
    for (const tile of await tiles.all()) {
      await expect(tile).toContainText('--')
    }

    // Product grid still loads normally
    await expect(page.getByText('Platform API')).toBeVisible()
    await expect(page.getByText('Customer App')).toBeVisible()
  })
})
