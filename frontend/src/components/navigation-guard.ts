// Shared navigation-protection guard for unsaved local drafts. It registers
// beforeunload and rw-before-navigate handlers that block leaving while a
// draft is dirty, and removes itself once the owning host disconnects.

/** Installs a navigation guard that blocks leaving while the draft is dirty. */
export function installNavigationGuard(host: HTMLElement, isDirty: () => boolean, confirmMessage: string): void {
  /** Blocks leaving while the draft is dirty, removing itself once the host disconnects. */
  function guard(event: Event): void {
    if (!host.isConnected) {
      document.removeEventListener("rw-before-navigate", guard);
      window.removeEventListener("beforeunload", guard);
      return;
    }
    if (!isDirty()) return;
    if (event.type === "beforeunload") {
      event.preventDefault();
      (event as BeforeUnloadEvent).returnValue = "";
      return;
    }
    if (!window.confirm(confirmMessage)) event.preventDefault();
  }
  document.addEventListener("rw-before-navigate", guard);
  window.addEventListener("beforeunload", guard);
}
