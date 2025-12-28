import { defineConfig } from '@playwright/test'

const baseURL = process.env.E2E_BASE_URL ?? 'http://localhost'

export default defineConfig({
  testDir: './tests',
  fullyParallel: true,
  reporter: [['list']],
  use: {
    baseURL,
    headless: true,
    viewport: { width: 1440, height: 900 },
  },
})
