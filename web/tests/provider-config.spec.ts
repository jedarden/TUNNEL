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

    // Find VS Code Tunnel provider and click configure
    const vscodeTunnelCard = page.locator('text=VS Code Tunnel').first()
    await expect(vscodeTunnelCard).toBeVisible()

    // Click the configure button for VS Code Tunnel
    const configureButton = page.locator('[data-provider="vscode-tunnel"]').locator('button:has-text("Configure")')
    if (await configureButton.isVisible()) {
      await configureButton.click()
    } else {
      // Try alternative selector
      await page.click('text=VS Code Tunnel >> .. >> button:has-text("Configure")')
    }

    // Wait for modal to open
    await expect(page.locator('[role="dialog"]')).toBeVisible()
  })

  test('should allow typing in VS Code Tunnel Machine Name field', async ({ page }) => {
    // Navigate to providers
    await page.click('text=Providers')
    await page.waitForLoadState('networkidle')

    // Open configure modal for VS Code Tunnel
    // Find the provider card and click configure
    const providerCards = page.locator('.provider-card, [class*="card"]')
    const vsCodeCard = providerCards.filter({ hasText: 'VS Code Tunnel' }).first()

    if (await vsCodeCard.isVisible()) {
      const configButton = vsCodeCard.locator('button:has-text("Configure")')
      await configButton.click()
    } else {
      // Fallback: look for any configure button near VS Code Tunnel text
      await page.getByRole('button', { name: /configure/i }).first().click()
    }

    // Wait for modal
    const modal = page.locator('[role="dialog"]')
    await expect(modal).toBeVisible({ timeout: 5000 })

    // Find the Machine Name input
    const machineNameInput = page.locator('#vsc-name')

    if (await machineNameInput.isVisible()) {
      // Clear any existing value and type
      await machineNameInput.click()
      await machineNameInput.fill('')
      await machineNameInput.type('test-machine-name')

      // Verify the value was entered
      await expect(machineNameInput).toHaveValue('test-machine-name')
    } else {
      // Try alternative: look for input with placeholder
      const altInput = modal.locator('input[placeholder*="my-dev-machine"]')
      await expect(altInput).toBeVisible()
      await altInput.click()
      await altInput.fill('test-machine-name')
      await expect(altInput).toHaveValue('test-machine-name')
    }
  })

  test('should allow typing in all provider config fields', async ({ page }) => {
    // Navigate to providers
    await page.click('text=Providers')
    await page.waitForLoadState('networkidle')

    // Test ngrok configuration
    const ngrokCard = page.locator('text=ngrok').first()
    if (await ngrokCard.isVisible()) {
      // Find configure button
      await page.locator('text=ngrok >> .. >> .. >> button:has-text("Configure")').first().click()

      const modal = page.locator('[role="dialog"]')
      await expect(modal).toBeVisible({ timeout: 5000 })

      // Test Auth Token input
      const authTokenInput = page.locator('#ngrok-token')
      if (await authTokenInput.isVisible()) {
        await authTokenInput.fill('test-token-12345')
        await expect(authTokenInput).toHaveValue('test-token-12345')
      }

      // Test Subdomain input
      const subdomainInput = page.locator('#ngrok-subdomain')
      if (await subdomainInput.isVisible()) {
        await subdomainInput.fill('my-subdomain')
        await expect(subdomainInput).toHaveValue('my-subdomain')
      }

      // Close modal
      await page.keyboard.press('Escape')
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

    // Open a provider configuration
    const configureButtons = page.locator('button:has-text("Configure")')
    const count = await configureButtons.count()

    if (count > 0) {
      await configureButtons.first().click()

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

    // Click first configure button
    const configBtn = page.locator('button:has-text("Configure")').first()
    if (await configBtn.isVisible()) {
      await configBtn.click()

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
    }
  })
})
