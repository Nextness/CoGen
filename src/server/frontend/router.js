// View routing, URL state, and render orchestrator.
import { state, app, view, link, value, showError, clearError, busy } from './state.js';
import { selects, clearContext, hydrateSelectors } from './components/context-selector.js';
import { overviewView } from './views/overview.js';
import { corpusView } from './views/corpus.js';
import { relationshipsView } from './views/relationships.js';
import { provenanceView } from './views/provenance.js';
import { evaluationView } from './views/evaluation.js';
import { advancedView } from './views/advanced.js';
import { trashView } from './views/trash.js';
import { detailView, destroyActiveArticleReview } from './views/detail.js';
import { destroyGraph } from './components/graph.js';

/** Replaces the current URL state without triggering a navigation reload. */
export function setURL(updates, replace) {
  if (!replace) {
    replace = false;
  }
  const href = link(updates);
  if (replace) {
    history.replaceState({}, '', href);
  } else {
    history.pushState({}, '', href);
  }
  render();
}

/** Binds DOM behavior for focus context. */
export function bindFocusContext() {
  const button = document.querySelector('[data-focus-context]');
  if (button) {
    button.addEventListener('click', function() {
      selects.search.focus();
    });
  }
}

/** Synchronizes primary navigation. */
function syncPrimaryNavigation(current) {
  var navigationView;
  if (['article', 'author', 'reference'].includes(current)) {
    navigationView = 'corpus';
  } else {
    navigationView = current;
  }

  document.querySelectorAll('[data-view-link]').forEach(function(item) {
    item.setAttribute('href', link({ view: item.dataset.viewLink }));
    var ariaCurrent;
    if (item.dataset.viewLink === navigationView) {
      ariaCurrent = 'page';
    } else {
      ariaCurrent = 'false';
    }
    item.setAttribute('aria-current', ariaCurrent);
  });
}

/** Asynchronously renders view. */
async function renderView() {
  const current = view();

  if (current === 'corpus') {
    return corpusView();
  }
  if (current === 'relationships') {
    return relationshipsView();
  }
  if (current === 'provenance') {
    return provenanceView();
  }
  if (current === 'evaluation') {
    return evaluationView();
  }
  if (current === 'advanced') {
    return advancedView();
  }
  if (current === 'trash') {
    return trashView();
  }
  if (current === 'article' || current === 'author' || current === 'reference') {
    return detailView(current);
  }
  return overviewView();
}

/** Asynchronously renders the associated state. */
export async function render() {
  const sequence = ++state.request;
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
    await hydrateSelectors();
    if (sequence !== state.request) {
      return;
    }
    await renderView();
    var pageTitle = 'Research workspace';
    const titleElement = document.querySelector('#page-title');
    if (titleElement) {
      pageTitle = titleElement.textContent || 'Research workspace';
    }
    document.title = pageTitle + ' · Research workspace';
  } catch (error) {
    if (error?.name !== 'AbortError' && sequence === state.request) {
      app.innerHTML = '';
      showError(error);
    }
  } finally {
    if (sequence === state.request) {
      busy(false);
    }
  }
}
