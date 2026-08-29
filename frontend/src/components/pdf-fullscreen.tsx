// Fullscreen reading mode for the embedded PDF viewer with a collapsible review drawer.
import { h, cx, classAdd, classRemove, classHas } from "../jsx/jsx-runtime.ts";

/** Typed compound class names used by this module. */
const classNames = {
  drawerEdge: cx("rw-reading-workspace__drawer-edge", "ui", "basic", "button"),
};

/** One mounted fullscreen controller. */
export interface PDFFullscreenController {
  destroy(): void;
}

/** Mounts the fullscreen toggle, CSS fallback, and review drawer for one reading workspace. */
export function mountPDFFullscreen(options: { workspace: HTMLElement; reviewHost: HTMLElement }): PDFFullscreenController {
  const workspace = options.workspace;
  const reviewHost = options.reviewHost;
  const button = workspace.querySelector<HTMLButtonElement>("[data-pdf-fullscreen]");
  let drawerCollapsed = false;
  let previouslyExpanded = false;
  let destroyed = false;
  var priorOverflow = "";
  let edgeControl: HTMLButtonElement | null = null;

  /** Returns whether the workspace is expanded by either the Fullscreen API or the fallback class. */
  function isExpanded(): boolean {
    return document.fullscreenElement === workspace || classHas(workspace, "rw-reading-workspace--expanded");
  }

  /** Updates the fullscreen button label and pressed state, returning focus on exit. */
  function updateLabel(): void {
    if (!button) return;
    const expanded = isExpanded();
    button.textContent = expanded ? "Exit fullscreen" : "Fullscreen";
    button.setAttribute("aria-pressed", expanded ? "true" : "false");
    if (previouslyExpanded && !expanded) button.focus();
    previouslyExpanded = expanded;
  }

  /** Synchronizes the drawer state, edge control, and button label with the current expansion. */
  function syncState(): void {
    if (isExpanded()) {
      drawerCollapsed = false;
      classRemove(workspace, "rw-reading-workspace--drawer-collapsed");
      renderEdgeControl();
      reviewHost.scrollTop = 0;
      const pages = workspace.querySelector<HTMLElement>("[data-pdf-pages]");
      pages?.focus();
    } else {
      drawerCollapsed = false;
      classRemove(workspace, "rw-reading-workspace--drawer-collapsed");
      removeEdgeControl();
    }
    updateLabel();
  }

  /** Renders the drawer edge control as a direct workspace child so it survives drawer collapse. */
  function renderEdgeControl(): void {
    if (edgeControl || destroyed) return;
    const chevron = h("span", { "data-drawer-chevron": "", "aria-hidden": "true" }, "\u00BB");
    const edge = h("button", {
      type: "button",
      className: classNames.drawerEdge,
      "data-drawer-edge": "",
      "aria-expanded": "true",
      "aria-label": "Hide review panel",
    }, chevron);
    workspace.append(edge);
    edgeControl = edge as HTMLButtonElement;
    edgeControl.addEventListener("click", toggleDrawer);
  }

  /** Removes the drawer edge control from the workspace. */
  function removeEdgeControl(): void {
    edgeControl?.remove();
    edgeControl = null;
  }

  /** Toggles the review drawer between expanded and collapsed. */
  function toggleDrawer(): void {
    drawerCollapsed = !drawerCollapsed;
    if (drawerCollapsed) {
      classAdd(workspace, ["rw-reading-workspace--drawer-collapsed"]);
    } else {
      classRemove(workspace, "rw-reading-workspace--drawer-collapsed");
    }
    if (edgeControl) {
      edgeControl.setAttribute("aria-expanded", drawerCollapsed ? "false" : "true");
      edgeControl.setAttribute("aria-label", drawerCollapsed ? "Show review panel" : "Hide review panel");
      const chevron = edgeControl.querySelector("[data-drawer-chevron]");
      if (chevron) chevron.textContent = drawerCollapsed ? "\u00AB" : "\u00BB";
    }
  }

  /** Enters or leaves the CSS fallback expansion, mirroring the graph's fallback path. */
  function toggleFallback(): void {
    if (classHas(workspace, "rw-reading-workspace--expanded")) {
      classRemove(workspace, "rw-reading-workspace--expanded");
      document.body.style.overflow = priorOverflow;
    } else {
      priorOverflow = document.body.style.overflow;
      document.body.style.overflow = "hidden";
      classAdd(workspace, ["rw-reading-workspace--expanded"]);
    }
    syncState();
  }

  const clickHandler = async (): Promise<void> => {
    try {
      if (document.fullscreenElement === workspace && document.exitFullscreen) {
        await document.exitFullscreen();
      } else if (workspace.requestFullscreen) {
        await workspace.requestFullscreen();
        syncState();
      } else {
        toggleFallback();
      }
    } catch (_) {
      toggleFallback();
    }
  };
  button?.addEventListener("click", clickHandler);

  const fullscreenHandler = (): void => {
    syncState();
  };
  document.addEventListener("fullscreenchange", fullscreenHandler);

  const escapeHandler = (event: KeyboardEvent): void => {
    if (event.key === "Escape" && classHas(workspace, "rw-reading-workspace--expanded")) {
      event.preventDefault();
      toggleFallback();
    }
  };
  document.addEventListener("keydown", escapeHandler);

  const selectionHandler = (): void => {
    if (drawerCollapsed) toggleDrawer();
  };
  workspace.addEventListener("rw-pdf-selection", selectionHandler);

  return {
    destroy: (): void => {
      if (destroyed) return;
      destroyed = true;
      button?.removeEventListener("click", clickHandler);
      document.removeEventListener("fullscreenchange", fullscreenHandler);
      document.removeEventListener("keydown", escapeHandler);
      workspace.removeEventListener("rw-pdf-selection", selectionHandler);
      if (document.fullscreenElement === workspace && document.exitFullscreen) {
        void document.exitFullscreen();
      }
      classRemove(workspace, "rw-reading-workspace--expanded");
      classRemove(workspace, "rw-reading-workspace--drawer-collapsed");
      document.body.style.overflow = priorOverflow;
      removeEdgeControl();
      if (button) {
        button.textContent = "Fullscreen";
        button.setAttribute("aria-pressed", "false");
      }
    },
  };
}