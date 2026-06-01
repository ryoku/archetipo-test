import { test, expect } from '@playwright/test'

test('root URL serves React app with 200 and HTML root element', async ({ page }) => {
  const response = await page.goto('/')
  expect(response?.status()).toBe(200)

  const contentType = response?.headers()['content-type'] ?? ''
  expect(contentType).toContain('text/html')

  await expect(page.locator('#root')).toBeVisible()
})

test('React Router /login route renders without 404', async ({ page }) => {
  const response = await page.goto('/login')
  expect(response?.status()).toBe(200)

  const contentType = response?.headers()['content-type'] ?? ''
  expect(contentType).toContain('text/html')

  await expect(page.locator('#root')).toBeVisible()
  await expect(page.locator('h1')).toHaveText('Login')
})
