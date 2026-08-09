// Entry point: imports modules, sets up event listeners, kicks off initial render.
import { bindDismissibleMessages, bindLoadingButtons } from './state.js';
import { render, setURL } from './router.js';
import { selects } from './components/context-selector.js';
import { initHealthCheck, initMobileNavToggle } from './components/shell.js';

selects.search.addEventListener('change', function(event) {
  setURL({ search_id: event.target.value, search_revision_id: '', plan_id: '', run_id: '' }, false);
});

selects.revision.addEventListener('change', function(event) {
  setURL({ search_revision_id: event.target.value, plan_id: '', run_id: '' }, false);
});

selects.plan.addEventListener('change', function(event) {
  setURL({ plan_id: event.target.value, run_id: '' }, false);
});

selects.run.addEventListener('change', function(event) {
  setURL({ run_id: event.target.value }, false);
});

document.addEventListener('click', function(event) {
  var anchor = event.target.closest('a[href^="?"]');
  if (!anchor) {
    return;
  }
  if (event.defaultPrevented || event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) {
    return;
  }
  event.preventDefault();
  history.pushState({}, '', anchor.getAttribute('href'));
  render();
});

// Delegate dismissible messages and loading buttons
document.addEventListener('click', function(event) {
  var closeButton = event.target.closest('.ui.message > .close');
  if (closeButton) {
    var message = closeButton.closest('.ui.message');
    if (message) {
      message.style.opacity = '0';
      setTimeout(function() { message.hidden = true; }, 150);
    }
    return;
  }
  var loadingButton = event.target.closest('[data-loading]');
  if (loadingButton) {
    loadingButton.classList.add('loading');
    loadingButton.disabled = true;
  }
});

window.addEventListener('popstate', function() {
  render();
});

initHealthCheck();
initMobileNavToggle();
render();
