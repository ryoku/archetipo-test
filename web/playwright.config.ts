import { defineConfig, devices } from '@playwright/test'

export default defineConfig({
  testDir: './e2e',
  outputDir: '../docs/test-results/US-017',
  use: {
    baseURL: 'http://localhost:4173',
    video: 'off',
  },
  webServer: {
    command: 'pnpm serve',
    port: 4173,
    reuseExistingServer: false,
  },
  projects: [
    {
      name: 'chromium',
      use: {
        ...devices['Desktop Chrome'],
        // Use the system-available headless shell when Playwright's bundled binary is absent.
        ...(process.env.PW_CHROMIUM_PATH
          ? { executablePath: process.env.PW_CHROMIUM_PATH }
          : {}),
      },
    },
  ],
})
