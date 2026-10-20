import type { Page } from '@playwright/test';

/**
 * Seeds viewer state through sessionStorage and navigates to a clean route.
 *
 * The seeding semantics are load-bearing and shared by every spec:
 * - `window.name` carries the seed across the initial about:blank document.
 * - `sessionStorage` is cleared first so a prior visit cannot leak state.
 * - When the current pathname already equals the target, the history entry is
 *   replaced so browser back/forward cannot restore a stale adopted state.
 * - The init script writes the seed only when sessionStorage is still empty,
 *   preserving the precedence of any state the app itself persisted.
 */
export async function visit(page: Page, state: Record<string, string>, path?: string): Promise<void> {
  const targetPath = path || (state.view === 'home' ? '/' : `/${state.view}`);
  await page.evaluate(({ seed, seedPath }) => {
    window.name = JSON.stringify(seed);
    try {
      sessionStorage.removeItem('rw-viewer-state');
      if (location.pathname === seedPath) history.replaceState(null, '', location.pathname);
    } catch (_) {
      // The initial about:blank document may deny sessionStorage access.
    }
  }, { seed: state, seedPath: targetPath });
  await page.addInitScript(() => {
    const seed = window.name ? JSON.parse(window.name) : null;
    if (seed && !sessionStorage.getItem('rw-viewer-state')) sessionStorage.setItem('rw-viewer-state', JSON.stringify(seed));
  });
  await page.goto(targetPath);
  await page.waitForLoadState('networkidle');
}

/** Reads the current viewer state from sessionStorage. */
export async function viewerState(page: Page): Promise<Record<string, string>> {
  return page.evaluate(() => JSON.parse(sessionStorage.getItem('rw-viewer-state') || '{}'));
}
