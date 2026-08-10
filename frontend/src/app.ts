// Entry point: imports modules, sets up event listeners, kicks off initial render.
import { bindDismissibleMessages, bindLoadingButtons } from "./state.ts";
import { render, setURL } from "./router.ts";
import { selects } from "./components/context-selector.ts";
import { initHealthCheck, initMobileNavToggle } from "./components/shell.ts";

selects.search.addEventListener("change", (event: Event) => {
  setURL({ search_id: (event.target as HTMLSelectElement).value, search_revision_id: "", plan_id: "", run_id: "" }, false);
});

selects.revision.addEventListener("change", (event: Event) => {
  setURL({ search_revision_id: (event.target as HTMLSelectElement).value, plan_id: "", run_id: "" }, false);
});

selects.plan.addEventListener("change", (event: Event) => {
  setURL({ plan_id: (event.target as HTMLSelectElement).value, run_id: "" }, false);
});

selects.run.addEventListener("change", (event: Event) => {
  setURL({ run_id: (event.target as HTMLSelectElement).value }, false);
});

document.addEventListener("click", function(event) {
  var anchor = (event.target as HTMLElement).closest<HTMLAnchorElement>(`a[href^="?"]`);
  if (!anchor) {
    return;
  }
  if (event.defaultPrevented || event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) {
    return;
  }
  event.preventDefault();
  history.pushState({}, "", anchor.getAttribute("href"));
  render();
});

// Delegate dismissible messages and loading buttons
document.addEventListener("click", function(event) {
  var closeButton = (event.target as HTMLElement).closest<HTMLElement>(".ui.message > .close");
  if (closeButton) {
    const message = closeButton.closest<HTMLElement>(".ui.message");
    if (message) {
      message.style.opacity = "0";
      setTimeout(function() { message.hidden = true; }, 150);
    }
    return;
  }
  var loadingButton = (event.target as HTMLElement).closest<HTMLButtonElement>("[data-loading]");
  if (loadingButton) {
    loadingButton.classList.add("loading");
    loadingButton.disabled = true;
  }
});

window.addEventListener("popstate", function() {
  render();
});

initHealthCheck();
initMobileNavToggle();
render();