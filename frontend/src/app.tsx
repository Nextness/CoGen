// Entry point: imports modules, sets up event listeners, kicks off initial render.
import { bindDismissibleMessages, bindLoadingButtons, contextChange, initViewerState, isStateObject, loadState, pathView, restoreState, stateFor, view, viewerState, viewPage } from "./state.tsx";
import { navigationAllowed, render, setURL } from "./router.tsx";
import { selects } from "./components/context-selector.tsx";
import { initHealthCheck, initMobileNavToggle } from "./components/shell.tsx";
import { classAdd } from "./jsx/jsx-runtime.ts";

const internalPagePaths = new Set(Object.values(viewPage));

selects.search.addEventListener("change", (event: Event) => {
  setURL(contextChange({
    search_id: (event.target as HTMLSelectElement).value,
    search_revision_id: "",
    plan_id: "",
    run_id: "",
  }), false);
});

selects.revision.addEventListener("change", (event: Event) => {
  setURL(contextChange({
    search_revision_id: (event.target as HTMLSelectElement).value,
    plan_id: "",
    run_id: "",
  }), false);
});

selects.plan.addEventListener("change", (event: Event) => {
  setURL(contextChange({
    plan_id: (event.target as HTMLSelectElement).value,
    run_id: "",
  }), false);
});

selects.run.addEventListener("change", (event: Event) => {
  setURL(contextChange({ run_id: (event.target as HTMLSelectElement).value }), false);
});

document.addEventListener("click", (event) => {
  const anchor = (event.target as HTMLElement).closest<HTMLAnchorElement>("a[href]");
  if (!anchor) return;
  if (event.defaultPrevented || event.button !== 0 || event.metaKey || event.ctrlKey || event.shiftKey || event.altKey || anchor.target || anchor.hasAttribute("download")) return;
  const href = anchor.getAttribute("href");
  if (!href) return;
  const destination = new URL(href, location.href);
  const destinationPath = "/" + destination.pathname.replace(/^\/+/, "").replace(/\/+$/, "");
  if (destination.origin !== location.origin || !internalPagePaths.has(destinationPath)) return;
  if (!navigationAllowed()) {
    event.preventDefault();
    return;
  }
  const destinationView = pathView(destination.pathname);
  if (destinationView === view()) {
    event.preventDefault();
    var applied = viewerState;
    if (anchor.dataset.state) {
      try {
        applied = stateFor(JSON.parse(anchor.dataset.state));
      } catch (_) {
        applied = viewerState;
      }
    }
    restoreState(applied);
    history.pushState(applied, "", href);
    render({ focusTitle: true, resetScroll: true });
    return;
  }
  if (anchor.dataset.state) {
    try {
      restoreState(stateFor(JSON.parse(anchor.dataset.state)));
    } catch (_) {
      restoreState(stateFor({ view: destinationView }));
    }
  } else {
    restoreState(stateFor({ view: destinationView }));
  }
});

// Delegate dismissible messages and loading buttons
document.addEventListener("click", (event) => {
  const closeButton = (event.target as HTMLElement).closest<HTMLElement>(".ui.message > .close");
  if (closeButton) {
    const message = closeButton.closest<HTMLElement>(".ui.message");
    if (message) {
      message.style.opacity = "0";
      setTimeout(() => { message.hidden = true; }, 150);
    }
    return;
  }
  const loadingButton = (event.target as HTMLElement).closest<HTMLButtonElement>("[data-loading]");
  if (loadingButton) {
    classAdd(loadingButton, ["loading"]);
    loadingButton.disabled = true;
  }
});

window.addEventListener("popstate", (event) => {
  if (!navigationAllowed()) {
    history.forward();
    return;
  }
  const restored = isStateObject(event.state) ? event.state : loadState() || {};
  restoreState({ ...restored, view: pathView() });
  render();
});

initHealthCheck();
initMobileNavToggle();
initViewerState();
const pageMarker = document.querySelector<HTMLMetaElement>('meta[name="rw-page"]')?.content || "home";
render({ focusTitle: pageMarker !== "home" });
