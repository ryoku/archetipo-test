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
  product_count: 5,
  environment_count: 12,
  component_count: 9,
  deployments_last_24h: 4,
}

const recentIso = new Date(Date.now() - 60 * 60 * 1000).toISOString()

const PRODUCTS = [
  {
    id: 'p-1',
    name: 'Platform API',
    slug: 'platform-api',
    description: 'Core platform backend',
    created_at: '2026-01-01T00:00:00Z',
    has_production_env: true,
    last_deployed_at: recentIso,
  },
  {
    id: 'p-2',
    name: 'Platform Gateway',
    slug: 'platform-gateway',
    description: 'API gateway service',
    created_at: '2026-01-02T00:00:00Z',
    has_production_env: false,
    last_deployed_at: null,
  },
  {
    id: 'p-3',
    name: 'Customer App',
    slug: 'customer-app',
    description: 'Customer-facing application',
    created_at: '2026-02-01T00:00:00Z',
    has_production_env: true,
    last_deployed_at: null,
  },
  {
    id: 'p-4',
    name: 'Worker Service',
    slug: 'worker-svc',
    description: 'Background job processor',
    created_at: '2026-03-01T00:00:00Z',
    has_production_env: false,
    last_deployed_at: null,
  },
  {
    id: 'p-5',
    name: 'Metrics Collector',
    slug: 'metrics-collector',
    description: 'Telemetry pipeline',
    created_at: '2026-04-01T00:00:00Z',
    has_production_env: false,
    last_deployed_at: null,
  },
]

const OIDC_CONFIG_STUB = {
  issuer: OIDC_AUTHORITY,
  authorization_endpoint: `${OIDC_AUTHORITY}/protocol/openid-connect/auth`,
  token_endpoint: `${OIDC_AUTHORITY}/protocol/openid-connect/token`,
  userinfo_endpoint: `${OIDC_AUTHORITY}/protocol/openid-connect/userinfo`,
  jwks_uri: `${OIDC_AUTHORITY}/protocol/openid-connect/certs`,
  end_session_endpoint: `${OIDC_AUTHORITY}/protocol/openid-connect/logout`,
  response_types_supported: ['code'],
  subject_types_supported: ['public'],
  id_token_signing_alg_values_supported: ['RS256'],
}

async function setupPage(page: Parameters<typeof test>[1] extends (arg: infer T) => unknown ? T : never) {
  const oidcDiscoveryUrl = `${OIDC_AUTHORITY}/.well-known/openid-configuration`

  // Step 1: stall OIDC discovery so the app redirects to the LOCAL /login
  // route (not Keycloak), giving us access to sessionStorage
  await page.route(oidcDiscoveryUrl, () => {})
  await page.goto('/')
  await expect(page).toHaveURL(/\/login/)

  // Step 2: inject session and unregister the stall
  await page.evaluate(
    ({ key, value }) => sessionStorage.setItem(key, JSON.stringify(value)),
    { key: SESSION_KEY, value: makeSession('Marco Andreoli', ACCESS_TOKEN) },
  )
  await page.unroute(oidcDiscoveryUrl)

  // Step 3: now fulfill discovery and JWKS so no background request hangs
  await page.route(oidcDiscoveryUrl, async (route) => {
    await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(OIDC_CONFIG_STUB) })
  })
  await page.route(`${OIDC_AUTHORITY}/protocol/openid-connect/certs`, async (route) => {
    await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ keys: [] }) })
  })
  await page.route(`${OIDC_AUTHORITY}/protocol/openid-connect/userinfo`, async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ sub: 'user-1', name: 'Marco Andreoli', email: 'marco@example.com' }),
    })
  })
  await page.route(`${OIDC_AUTHORITY}/protocol/openid-connect/token`, async (route) => {
    await route.fulfill({ status: 400, contentType: 'application/json', body: JSON.stringify({ error: 'invalid_request' }) })
  })

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

  await page.goto('/')
  await expect(page.getByTestId('search-filter-bar')).toBeVisible()
}

// ─────────────────────────────────────────────────────────────────
// Demo scenario — video on, slowMo for readability
// ─────────────────────────────────────────────────────────────────
test.use({
  video: 'on',
  viewport: { width: 1280, height: 720 },
  launchOptions: { slowMo: 300 },
})

test('demo: Marco searches for "platform" and then narrows to Production products', async ({ page }) => {
  test.setTimeout(90000)
  await setupPage(page)

  // Step 1: full product list visible
  await expect(page.getByText('Platform API')).toBeVisible()
  await expect(page.getByText('Customer App')).toBeVisible()
  await expect(page.getByText('Worker Service')).toBeVisible()
  await page.waitForTimeout(500)

  // Step 2: type "platform" in the search box — grid narrows in real time
  await page.getByTestId('search-input').fill('platform')
  await expect(page.getByText('Platform API')).toBeVisible()
  await expect(page.getByText('Platform Gateway')).toBeVisible()
  await expect(page.locator('.home-product-grid')).not.toContainText('Customer App')
  await expect(page.locator('.home-product-grid')).not.toContainText('Worker Service')
  await page.waitForTimeout(600)

  // Step 3: click Production chip — further narrows to products with a production env
  await page.getByTestId('chip-production').click()
  await expect(page.getByTestId('chip-production')).toHaveClass(/active/)
  await expect(page.getByText('Platform API')).toBeVisible()
  await expect(page.locator('.home-product-grid')).not.toContainText('Platform Gateway')
  await page.waitForTimeout(600)

  // Step 4: clear search — Production chip still active, shows all production products
  await page.getByTestId('search-clear').click()
  await expect(page.getByText('Platform API')).toBeVisible()
  await expect(page.getByText('Customer App')).toBeVisible()
  await expect(page.locator('.home-product-grid')).not.toContainText('Worker Service')
  await page.waitForTimeout(600)

  // Step 5: click "Recently deployed" chip — only Platform API (deployed 1h ago)
  await page.getByTestId('chip-recently-deployed').click()
  await expect(page.getByTestId('chip-recently-deployed')).toHaveClass(/active/)
  await expect(page.getByText('Platform API')).toBeVisible()
  await expect(page.locator('.home-product-grid')).not.toContainText('Customer App')
  await page.waitForTimeout(600)

  // Step 6: click All — full list restored
  await page.getByTestId('chip-all').click()
  await expect(page.getByTestId('chip-all')).toHaveClass(/active/)
  await expect(page.getByText('Customer App')).toBeVisible()
  await expect(page.getByText('Worker Service')).toBeVisible()
  await page.waitForTimeout(1200)
})

