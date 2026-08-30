// Shared viewer-state seeding for unit tests that render views directly.
// Seeding goes through the real boot adoption path: the state is attached to
// history and the pathname is set to the owning view path, then initViewerState
// adopts it exactly as a full page load would.
import { initViewerState, viewPage } from '../../src/state.tsx';

/** Seeds viewerState through the boot adoption path for one view state. */
export function seedViewerState(state: Record<string, string>): void {
  const path = viewPage[state.view || 'home'] || viewPage.home;
  history.replaceState(state, '', path);
  initViewerState();
}
