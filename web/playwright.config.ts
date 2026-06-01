import { defineConfig, devices } from '@playwright/test'

export default defineConfig({
  testDir: './e2e',
  outputDir: '../docs/test-results/US-004',
  use: {
    baseURL: 'http://localhost:4173',
    video: 'off',
  },
  webServer: {
    command: 'npm run serve',
    port: 4173,
    reuseExistingServer: false,
  },
  projects: [
    {
      name: 'chromium',
      use: { ...devices['Desktop Chrome'] },
    },
  ],
})
