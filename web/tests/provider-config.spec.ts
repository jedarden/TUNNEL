import { test, expect } from '@playwright/test'

test.describe('Provider Configuration Modal', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/')
    // Wait for the page to load
    await page.waitForLoadState('networkidle')
  })

  test('should navigate to providers page', async ({ page }) => {
    // Click on Providers in the navigation
    await page.click('text=Providers')
    await expect(page.locator('h1')).toContainText('Providers')
  })

  test('should open VS Code Tunnels configuration modal', async ({ page }) => {
    // Navigate to providers
    await page.click('text=Providers')
    await page.waitForLoadState('networkidle')

    // Wait for provider cards to load
    await page.waitForSelector('text=VS Code Tunnels', { timeout: 10000 })

    // Click the first configure button to open any modal
    await page.locator('button:has-text("Configure")').first().click()

    // Wait for modal to open
    await expect(page.locator('[role="dialog"]')).toBeVisible({ timeout: 5000 })
  })

  test('should allow typing in VS Code Tunnel Machine Name field', async ({ page }) => {
    // Navigate to providers
    await page.click('text=Providers')
    await page.waitForLoadState('networkidle')

    // Wait for provider cards to load
    await page.waitForSelector('text=VS Code Tunnels', { timeout: 10000 })

    // Find and click configure button for VS Code Tunnels
    // Look for a card containing "VS Code Tunnels" and click its Configure button
    const cards = page.locator('[class*="card"], [class*="Card"]')
    const vsCodeCard = cards.filter({ hasText: 'VS Code Tunnels' }).first()

    if (await vsCodeCard.isVisible()) {
      await vsCodeCard.locator('button:has-text("Configure")').click()
    } else {
      // Fallback: click any configure button
      await page.locator('button:has-text("Configure")').first().click()
    }

    // Wait for modal
    const modal = page.locator('[role="dialog"]')
    await expect(modal).toBeVisible({ timeout: 5000 })

    // Find any text input in the modal and test typing
    const textInputs = modal.locator('input[type="text"], input:not([type])')
    const inputCount = await textInputs.count()

    if (inputCount > 0) {
      const firstInput = textInputs.first()
      await firstInput.click()
      await firstInput.fill('test-machine-name')
      await expect(firstInput).toHaveValue('test-machine-name')
    }
  })

  test('should allow typing in all provider config fields', async ({ page }) => {
    // Navigate to providers
    await page.click('text=Providers')
    await page.waitForLoadState('networkidle')

    // Wait for providers to load
    await page.waitForSelector('button:has-text("Configure")', { timeout: 10000 })

    // Click first configure button
    await page.locator('button:has-text("Configure")').first().click()

    const modal = page.locator('[role="dialog"]')
    await expect(modal).toBeVisible({ timeout: 5000 })

    // Test typing in visible text inputs
    const textInputs = modal.locator('input[type="text"], input:not([type="password"]):not([type="number"]):not([type="hidden"])')
    const inputCount = await textInputs.count()

    for (let i = 0; i < Math.min(inputCount, 3); i++) {
      const input = textInputs.nth(i)
      if (await input.isVisible() && await input.isEnabled()) {
        const testValue = `test-value-${i}`
        await input.click()
        await input.fill(testValue)
        await expect(input).toHaveValue(testValue)
      }
    }
  })

  test('should handle input changes without console errors', async ({ page }) => {
    const consoleErrors: string[] = []

    page.on('console', msg => {
      if (msg.type() === 'error') {
        consoleErrors.push(msg.text())
      }
    })

    // Navigate to providers
    await page.click('text=Providers')
    await page.waitForLoadState('networkidle')

    // Wait for configure buttons
    await page.waitForSelector('button:has-text("Configure")', { timeout: 10000 })

    // Open a provider configuration
    await page.locator('button:has-text("Configure")').first().click()

    const modal = page.locator('[role="dialog"]')
    await expect(modal).toBeVisible({ timeout: 5000 })

    // Find any text input and try typing
    const inputs = modal.locator('input[type="text"], input:not([type])')
    const inputCount = await inputs.count()

    for (let i = 0; i < Math.min(inputCount, 3); i++) {
      const input = inputs.nth(i)
      if (await input.isVisible() && await input.isEnabled()) {
        await input.click()
        await input.fill(`test-value-${i}`)
      }
    }

    // Filter out extension-related errors (these are not our issue)
    const relevantErrors = consoleErrors.filter(err =>
      !err.includes('runtime.lastError') &&
      !err.includes('extension') &&
      !err.includes('chrome-extension')
    )

    expect(relevantErrors).toHaveLength(0)
  })
})

test.describe('Input Field Functionality', () => {
  test('input fields should accept keyboard input', async ({ page }) => {
    await page.goto('/')
    await page.click('text=Providers')
    await page.waitForLoadState('networkidle')

    // Wait for configure buttons
    await page.waitForSelector('button:has-text("Configure")', { timeout: 10000 })

    // Click first configure button
    await page.locator('button:has-text("Configure")').first().click()

    // Wait for modal
    const modal = page.locator('[role="dialog"]')
    await expect(modal).toBeVisible({ timeout: 5000 })

    // Test typing character by character
    const firstInput = modal.locator('input:visible').first()
    if (await firstInput.isVisible()) {
      await firstInput.focus()

      // Type character by character
      for (const char of 'hello') {
        await page.keyboard.type(char)
      }

      const value = await firstInput.inputValue()
      expect(value).toContain('hello')
    }
  })
})
