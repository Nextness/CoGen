import { defineConfig, devices } from '@playwright/test';

/**
 * Playwright configuration for the research workspace viewer.
 * Makefile targets start an isolated fixture-backed Go server and provide its URL.
 * @see https://playwright.dev/docs/test-configuration
 */
export default defineConfig({
  testDir: './tests',
  testMatch: process.env.PLAYWRIGHT_SUITE === 'mutation' ? /(?:review|e2e)\.spec\.ts$/ : undefined,
  testIgnore: [
    process.env.PLAYWRIGHT_SUITE === 'read' ? /(?:review|e2e)\.spec\.ts$/ : undefined,
    /tests\/unit\//,
  ].filter((entry): entry is RegExp => entry !== undefined),
  fullyParallel: process.env.PLAYWRIGHT_SUITE !== 'mutation',
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 2 : 0,
  workers: process.env.PLAYWRIGHT_SUITE === 'mutation' || process.env.CI ? 1 : undefined,
  reporter: [
    ['html', { outputFolder: process.env.PLAYWRIGHT_HTML_OUTPUT_DIR || 'playwright-report' }],
    ['list'],
  ],
  use: {
    baseURL: process.env.PLAYWRIGHT_BASE_URL || 'http://127.0.0.1:8080',
    locale: 'en-US',
    timezoneId: 'UTC',
    trace: 'on-first-retry',
    screenshot: 'only-on-failure',
  },
  projects: [
    {
      name: 'chromium',
      use: { ...devices['Desktop Chrome'] },
    },
    {
      name: 'firefox',
      use: { ...devices['Desktop Firefox'] },
    },
    {
      name: 'webkit',
      use: { ...devices['Desktop Safari'] },
    },
  ],
  outputDir: process.env.PLAYWRIGHT_TEST_RESULTS_DIR || 'test-results',
});
