import { expect, test } from '@playwright/test'

test('redirects unauthenticated users to login', async ({ page }) => {
  await page.goto('/settings')

  await expect(page).toHaveURL(/\/login$/)
  await expect(
    page.getByRole('heading', { name: 'Connect your account' }),
  ).toBeVisible()
  await expect(page.getByRole('link', { name: 'Continue with GitHub' })).toHaveAttribute(
    'href',
    /\/auth\/login$/,
  )
})

const storageState = process.env.E2E_STORAGE_STATE

test.describe('authenticated session', () => {
  test.skip(!storageState, 'E2E_STORAGE_STATE not set')
  test.use({ storageState })

  test('loads the dashboard when authenticated', async ({ page }) => {
    await page.goto('/')

    await expect(page).toHaveURL(/\/$/)
    await expect(page.getByRole('heading', { name: 'Deployment runway' })).toBeVisible()
  })
})
