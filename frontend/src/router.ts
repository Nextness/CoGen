// View routing, URL state, and render orchestrator.
import { state, app, view, link, value, showError, clearError, busy, setBreadcrumb } from "./state.ts";
import { selects, hydrateSelectors } from "./components/context-selector.ts";
import { homeView } from "./views/home.ts";
import { overviewView } from "./views/overview.ts";
import { corpusView } from "./views/corpus.ts";
import { relationshipsView } from "./views/relationships.ts";
import { provenanceView } from "./views/provenance.ts";
import { evaluationView } from "./views/evaluation.ts";
import { advancedView } from "./views/advanced.ts";
import { detailView, destroyActiveArticleReview } from "./views/detail.ts";
import { destroyGraph } from "./components/graph.ts";

/** Replaces the current URL state without triggering a navigation reload. */
export function setURL(updates: Record<string, any>, replace: boolean): void {
  const href = link(updates);
  if (replace) {
    history.replaceState({}, "", href);
  } else {
    history.pushState({}, "", href);
  }
  render();
}

/** Binds DOM behavior for focus context. */
export function bindFocusContext(): void {
  const button = document.querySelector<HTMLButtonElement>("[data-focus-context]");
  if (button) {
    button.addEventListener("click", function() {
      selects.search.focus();
    });
  }
}

/** Synchronizes primary navigation. */
function syncPrimaryNavigation(current: string): void {
  var navigationView: string;
  if (["article", "author", "reference"].includes(current)) {
    navigationView = "corpus";
  } else {
    navigationView = current;
  }

  document.querySelectorAll<HTMLElement>("[data-view-link]").forEach(function(item) {
    item.setAttribute("href", link({ view: item.dataset.viewLink }));
    const active = item.dataset.viewLink === navigationView;
    const ariaCurrent = active ? "page" : "false";
    item.classList.toggle("active", active);
    item.setAttribute("aria-current", ariaCurrent);
  });
}

/** Synchronizes shell visibility and the page-level breadcrumb before a view renders. */
function syncShell(current: string): void {
  const isHome = current === "home" || current === "trash";
  const contextPanel = document.querySelector<HTMLElement>(".context-panel");
  const navigation = document.querySelector<HTMLElement>(".primary-nav");
  const mobileToggle = document.querySelector<HTMLElement>("#mobile-nav-toggle");

  if (contextPanel) contextPanel.hidden = isHome;
  if (navigation) navigation.hidden = isHome;
  if (mobileToggle) mobileToggle.hidden = isHome;
  if (isHome) {
    setBreadcrumb([{ label: "Home" }]);
    return;
  }

  const labels: Record<string, string> = {
    overview: "Overview", corpus: "Corpus", relationships: "Relationships", provenance: "Provenance",
    evaluation: "Evaluation", advanced: "Advanced", article: "Article", author: "Author", reference: "Reference mention"
  };

  setBreadcrumb([
    { label: "Home", href: link({ view: "home", article_id: "", author_id: "", reference_id: "" }) },
    { label: "Deepdive", href: link({ view: "overview", article_id: "", author_id: "", reference_id: "" }) },
    { label: labels[current] || "Overview" }
  ]);
}

/** Asynchronously renders view. */
async function renderView(): Promise<any> {
  const current = view();

  if (current === "home" || current === "trash") return homeView();
  if (current === "corpus") return corpusView();
  if (current === "relationships") return relationshipsView();
  if (current === "provenance") return provenanceView();
  if (current === "evaluation") return evaluationView();
  if (current === "advanced") return advancedView();
  if (current === "article" || current === "author" || current === "reference") return detailView(current);

  return overviewView();
}

/** Asynchronously renders the associated state. */
export async function render(): Promise<void> {
  const sequence = ++state.request;
  syncShell(view());
  syncPrimaryNavigation(view());
  destroyGraph();
  await destroyActiveArticleReview();

  if (state.controller) {
    state.controller.abort();
  }
  state.controller = new AbortController();

  clearError();
  busy(true);

  try {
    if (view() !== "home" && view() !== "trash") await hydrateSelectors();
    if (sequence !== state.request) return;

    await renderView();
    var pageTitle = "Research workspace";
    const titleElement = document.querySelector<HTMLElement>("#page-title");
    if (titleElement) pageTitle = titleElement.textContent || "Research workspace";

    document.title = pageTitle + " · Research workspace";
  } catch (error) {
    if ((error as any)?.name !== "AbortError" && sequence === state.request) {
      app.innerHTML = "";
      showError(error);
    }
  } finally {
    if (sequence === state.request) busy(false);
  }
}
