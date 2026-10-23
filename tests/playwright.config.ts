import { defineConfig, devices } from '@playwright/test';

/**
 * Playwright configuration for the research workspace viewer.
 * Makefile targets start an isolated fixture-backed Go server and provide its URL.
 * @see https://playwright.dev/docs/test-configuration
 */
export default defineConfig({
  testDir: ".",
  testMatch: process.env.PLAYWRIGHT_SUITE === "mutation" ? /(?:review|e2e|api-shapes)\.spec\.ts$/ : /\.spec\.ts$/,
  testIgnore: [
    process.env.PLAYWRIGHT_SUITE === "read" ? /(?:review|e2e|api-shapes)\.spec\.ts$/ : undefined,
    process.env.E2E_SPEC === "1" ? undefined : /e2e\/offline\//,
    /\/(?:unit|functional)\//,
  ].filter((entry): entry is RegExp => entry !== undefined),
  fullyParallel: process.env.PLAYWRIGHT_SUITE !== 'mutation',
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 2 : 0,
  workers: process.env.PLAYWRIGHT_SUITE === 'mutation' || process.env.CI ? 1 : undefined,
  reporter: [
    ['html', { outputFolder: process.env.PLAYWRIGHT_HTML_OUTPUT_DIR || "../build/playwright/report" }],
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
  ],
  outputDir: process.env.PLAYWRIGHT_TEST_RESULTS_DIR || "../build/playwright/results",
});
