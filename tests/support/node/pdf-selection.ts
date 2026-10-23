import type { Page } from '@playwright/test';

/** Selects the fixture methods text and hands it to the review selection flow. */
export async function selectFixtureText(page: Page): Promise<void> {
  await page.evaluate(() => {
    const layer = document.querySelector('.rw-pdf-page .textLayer');
    if (!layer) throw new Error('PDF text layer is unavailable');
    const text = Array.from(layer.querySelectorAll('span')).find((span) => span.textContent?.includes('Selectable fixture methods'));
    if (!text) throw new Error('Selectable fixture methods text is unavailable');
    const range = document.createRange();
    range.selectNodeContents(text);
    const selection = window.getSelection();
    if (!selection) throw new Error('Document selection is unavailable');
    selection.removeAllRanges();
    selection.addRange(range);
    const viewer = document.querySelector('.rw-pdf-viewer');
    if (!viewer) throw new Error('PDF viewer is unavailable');
    viewer.dispatchEvent(new MouseEvent('mouseup', { bubbles: true }));
  });
}
