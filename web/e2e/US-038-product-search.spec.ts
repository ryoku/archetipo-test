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
  await page.route(oidcDiscoveryUrl, () => {})
  await page.goto('/')
  await expect(page).toHaveURL(/\/login/)
  await page.evaluate(
    ({ key, value }) => sessionStorage.setItem(key, JSON.stringify(value)),
    { key: SESSION_KEY, value: makeSession('Marco Andreoli', ACCESS_TOKEN) },
  )
  await page.unroute(oidcDiscoveryUrl)
  await page.route(oidcDiscoveryUrl, async (route) => {
    await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(OIDC_CONFIG_STUB) })
  })
  await page.route(`${OIDC_AUTHORITY}/protocol/openid-connect/certs`, async (route) => {
    await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ keys: [] }) })
  })
  await page.route('**/api/v1/stats', async (route) => {
    await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ product_count: 3, environment_count: 5, component_count: 4, deployments_last_24h: 1 }) })
  })
  await page.route('**/api/v1/products', async (route) => {
    if (route.request().method() === 'GET') {
      await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(PRODUCTS) })
    } else {
      await route.continue()
    }
  })
  await page.goto('/')
  await expect(page.getByTestId('search-filter-bar')).toBeVisible()
}

test('empty state with descriptive message when no products match combined filter', async ({ page }) => {
  await setupPage(page)

  await page.getByTestId('chip-recently-deployed').click()
  await page.getByTestId('search-input').fill('zzz')

  await expect(page.getByTestId('filter-empty-state')).toBeVisible()
  await expect(page.getByText(/No products match your search/i)).toBeVisible()
  await expect(page.getByText('Clear filters')).toBeVisible()
})

test('Clear filters button resets view to full product list', async ({ page }) => {
  await setupPage(page)

  await page.getByTestId('chip-production').click()
  await page.getByTestId('search-input').fill('zzz')
  await expect(page.getByTestId('filter-empty-state')).toBeVisible()

  await page.getByText('Clear filters').click()

  await expect(page.getByText('Platform API')).toBeVisible()
  await expect(page.getByText('Customer App')).toBeVisible()
  await expect(page.getByText('Worker Service')).toBeVisible()
})
