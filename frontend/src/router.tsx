// View routing, URL state, and render orchestrator.
import { state, app, view, link, showError, clearError, busy, setBreadcrumb } from "./state.tsx";
import { render as renderTree, classToggle } from "./jsx/jsx-runtime.ts";
import { focusContextSelector, hydrateSelectors } from "./components/context-selector.tsx";
import { homeView } from "./views/home.tsx";
import { overviewView } from "./views/overview.tsx";
import { corpusView } from "./views/corpus.tsx";
import { relationshipsView } from "./views/relationships.tsx";
import { provenanceView } from "./views/provenance.tsx";
import { evaluationView } from "./views/evaluation.tsx";
import { advancedView } from "./views/advanced.tsx";
import { detailView, destroyActiveArticleReview } from "./views/detail.tsx";
import { destroyGraph } from "./components/graph.tsx";

/** Pushes or replaces URL state and immediately renders the resulting route. */
export function setURL(updates: Record<string, unknown>, replace: boolean): void {
  if (!navigationAllowed()) return;
  const href = link(updates);
  const destination = new URL(href, location.href).searchParams.get("view") || "home";
  if (destination !== view()) {
    if (replace) location.replace(href);
    else location.assign(href);
    return;
  }
  if (replace) history.replaceState({}, "", href);
  else history.pushState({}, "", href);
  render({ focusTitle: true, resetScroll: true });
}

/** Gives mounted editors one cancelable opportunity to protect unsaved local input. */
export function navigationAllowed(): boolean {
  return document.dispatchEvent(new CustomEvent("rw-before-navigate", { cancelable: true }));
}

/** Binds DOM behavior for focus context. */
export function bindFocusContext(): void {
  const button = document.querySelector<HTMLButtonElement>("[data-focus-context]");
  if (button) {
    button.addEventListener("click", () => {
      focusContextSelector();
    });
  }
}

/** Synchronizes primary navigation. */
function syncPrimaryNavigation(current: string): void {
  const detailViews = ["article", "author", "reference"];
  var navigationView = current;
  if (detailViews.includes(current)) navigationView = "corpus";

  const viewLinks = document.querySelectorAll<HTMLElement>("[data-view-link]");
  viewLinks.forEach((item) => {
    item.setAttribute("href", link({ view: item.dataset.viewLink }));
    const active = item.dataset.viewLink === navigationView;
    var ariaCurrent = "false";
    if (active) ariaCurrent = "page";
    classToggle(item, "active", active);
    item.setAttribute("aria-current", ariaCurrent);
  });
}

/** Synchronizes shell visibility and the page-level breadcrumb before a view renders. */
function syncShell(current: string): void {
  const isHome = current === "home" || current === "trash";
  const contextPanel = document.querySelector<HTMLElement>(".rw-context-panel");
  const navigation = document.querySelector<HTMLElement>(".rw-primary-nav");
  const mobileToggle = document.querySelector<HTMLElement>("#mobile-nav-toggle");

  if (contextPanel) contextPanel.hidden = isHome;
  if (navigation) navigation.hidden = isHome;
  if (mobileToggle) mobileToggle.hidden = isHome;
  if (isHome) {
    setBreadcrumb([{ label: "Home" }]);
    return;
  }

  const labels: Record<string, string> = {
    overview: "Overview",
    corpus: "Corpus",
    relationships: "Relationships",
    provenance: "Provenance",
    evaluation: "Evaluation",
    advanced: "Advanced",
    article: "Article",
    author: "Author",
    reference: "Reference mention",
  };

  setBreadcrumb([
    {
      label: "Home",
      href: link({
        view: "home",
        article_id: "",
        author_id: "",
        reference_id: "",
      }),
    },
    {
      label: "Deepdive",
      href: link({
        view: "overview",
        article_id: "",
        author_id: "",
        reference_id: "",
      }),
    },
    { label: labels[current] || "Overview" },
  ]);
}

/** Asynchronously renders view. */
async function renderView(): Promise<void> {
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
export async function render(options?: { focusTitle?: boolean; resetScroll?: boolean }): Promise<void> {
  const sequence = ++state.request;
  var titleToFocus: HTMLElement | null = null;
  if (state.controller) state.controller.abort();
  state.controller = new AbortController();

  syncShell(view());
  syncPrimaryNavigation(view());
  destroyGraph();
  clearError();
  busy(true);

  try {
    await destroyActiveArticleReview();
    if (sequence !== state.request) return;

    if (view() !== "home" && view() !== "trash") await hydrateSelectors();
    if (sequence !== state.request) return;

    await renderView();
    if (sequence !== state.request) return;
    var pageTitle = "Research workspace";
    const titleElement = document.querySelector<HTMLElement>("#page-title");
    if (titleElement) pageTitle = titleElement.textContent || "Research workspace";

    document.title = `${pageTitle} · Research workspace`;
    if (options?.resetScroll) window.scrollTo({ top: 0, left: 0, behavior: "auto" });
    if (options?.focusTitle && titleElement) titleToFocus = titleElement;
  } catch (error) {
    const isAbort = typeof error === "object" && error !== null && "name" in error && error.name === "AbortError";
    if (!isAbort && sequence === state.request) {
      renderTree(null, app);
      showError(error);
    }
  } finally {
    if (sequence === state.request) {
      busy(false);
      if (titleToFocus) {
        titleToFocus.tabIndex = -1;
        titleToFocus.focus({ preventScroll: true });
      }
    }
  }
}
