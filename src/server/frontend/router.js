// View routing, URL state, and render orchestrator.
import { state, app, view, link, value, showError, clearError, busy, setBreadcrumb } from './state.js';
import { selects, hydrateSelectors } from './components/context-selector.js';
import { homeView } from './views/home.js';
import { overviewView } from './views/overview.js';
import { corpusView } from './views/corpus.js';
import { relationshipsView } from './views/relationships.js';
import { provenanceView } from './views/provenance.js';
import { evaluationView } from './views/evaluation.js';
import { advancedView } from './views/advanced.js';
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
    const active = item.dataset.viewLink === navigationView;
    const ariaCurrent = active ? 'page' : 'false';
    item.classList.toggle('active', active);
    item.setAttribute('aria-current', ariaCurrent);
  });
}

/** Synchronizes shell visibility and the page-level breadcrumb before a view renders. */
function syncShell(current) {
  const isHome = current === 'home' || current === 'trash';
  const contextPanel = document.querySelector('.context-panel');
  const navigation = document.querySelector('.primary-nav');
  const mobileToggle = document.querySelector('#mobile-nav-toggle');
  if (contextPanel) contextPanel.hidden = isHome;
  if (navigation) navigation.hidden = isHome;
  if (mobileToggle) mobileToggle.hidden = isHome;
  if (isHome) {
    setBreadcrumb([{ label: 'Home' }]);
    return;
  }
  const labels = {
    overview: 'Overview', corpus: 'Corpus', relationships: 'Relationships', provenance: 'Provenance',
    evaluation: 'Evaluation', advanced: 'Advanced', article: 'Article', author: 'Author', reference: 'Reference mention'
  };
  setBreadcrumb([
    { label: 'Home', href: link({ view: 'home', article_id: '', author_id: '', reference_id: '' }) },
    { label: 'Deepdive', href: link({ view: 'overview', article_id: '', author_id: '', reference_id: '' }) },
    { label: labels[current] || 'Overview' }
  ]);
}

/** Asynchronously renders view. */
async function renderView() {
  const current = view();

  if (current === 'home' || current === 'trash') {
    return homeView();
  }

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
  if (current === 'article' || current === 'author' || current === 'reference') {
    return detailView(current);
  }
  return overviewView();
}

/** Asynchronously renders the associated state. */
export async function render() {
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
    if (view() !== 'home' && view() !== 'trash') {
      await hydrateSelectors();
    }
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
