// Context selector: search, revision, plan, run dropdowns.
// Enhanced with search filtering, loading skeletons,
// auto-select, error states, and per-dropdown clear buttons.
import { state, pickID, text, list, selectedRun, esc, link, value } from '../state.js';
import { setURL } from '../router.js';
import { api } from '../api.js';

export const selects = {
  search: document.querySelector('#search-select'),
  revision: document.querySelector('#revision-select'),
  plan: document.querySelector('#plan-select'),
  run: document.querySelector('#run-select'),
};
export const clearContext = document.querySelector('#clear-context');

// Map select key to URL parameter name
const paramMap = {
  search: 'search_id',
  revision: 'search_revision_id',
  plan: 'plan_id',
  run: 'run_id',
};

/**
 * Show a loading skeleton placeholder in a dropdown's field area.
 * @param {string} key - The select key ('search', 'revision', 'plan', 'run')
 */
function showSkeleton(key) {
  var select = selects[key];
  if (!select) return;
  var field = select.closest('.ui.field');
  if (!field) return;
  // Remove existing skeleton or error message
  var existing = field.querySelector('.ui.placeholder, .ui.error.message');
  if (existing) existing.remove();
  var skeleton = document.createElement('div');
  skeleton.className = 'ui placeholder';
  skeleton.innerHTML = '<div class="line"></div><div class="line"></div><div class="line"></div>';
  // Insert after the label
  var label = field.querySelector('label');
  if (label && label.nextSibling) {
    label.parentNode.insertBefore(skeleton, label.nextSibling);
  } else {
    field.appendChild(skeleton);
  }
}

/**
 * Remove a loading skeleton from a dropdown's field area.
 * @param {string} key - The select key
 */
function hideSkeleton(key) {
  var select = selects[key];
  if (!select) return;
  var field = select.closest('.ui.field');
  if (!field) return;
  var skeleton = field.querySelector('.ui.placeholder');
  if (skeleton) skeleton.remove();
}

/**
 * Show an inline error message below a dropdown.
 * @param {string} key - The select key
 * @param {string} message - Error message text
 */
function showDropdownError(key, message) {
  var select = selects[key];
  if (!select) return;
  var field = select.closest('.ui.field');
  if (!field) return;
  // Remove existing skeleton or error message
  var existing = field.querySelector('.ui.placeholder, .ui.error.message');
  if (existing) existing.remove();
  var error = document.createElement('div');
  error.className = 'ui error message';
  error.textContent = message;
  field.appendChild(error);
}

/**
 * Remove an error message from a dropdown's field area.
 * @param {string} key - The select key
 */
function hideDropdownError(key) {
  var select = selects[key];
  if (!select) return;
  var field = select.closest('.ui.field');
  if (!field) return;
  var error = field.querySelector('.ui.error.message');
  if (error) error.remove();
}

/**
 * Add a clear button (×) to a dropdown field.
 * Clicking it clears the select value and resets dependent URL params.
 * @param {string} key - The select key
 */
function addClearButton(key) {
  var select = selects[key];
  if (!select) return;
  var field = select.closest('.ui.field');
  if (!field) return;
  // Remove existing clear button
  var existing = field.querySelector('.ui.dropdown.clear');
  if (existing) existing.remove();
  var btn = document.createElement('button');
  btn.className = 'ui dropdown clear';
  btn.type = 'button';
  btn.setAttribute('aria-label', 'Clear ' + key + ' selection');
  btn.textContent = '\u00D7';
  btn.addEventListener('click', function(event) {
    event.stopPropagation();
    select.value = '';
    // Clear this and all dependent URL params
    var updates = { [paramMap[key]]: '' };
    var dependentKeys = { search: ['search_revision_id', 'plan_id', 'run_id'], revision: ['plan_id', 'run_id'], plan: ['run_id'], run: [] };
    (dependentKeys[key] || []).forEach(function(dep) {
      updates[dep] = '';
    });
    setURL(updates, false);
  });
  field.appendChild(btn);
}

/**
 * Remove a clear button from a dropdown field.
 * @param {string} key - The select key
 */
function removeClearButton(key) {
  var select = selects[key];
  if (!select) return;
  var field = select.closest('.ui.field');
  if (!field) return;
  var btn = field.querySelector('.ui.dropdown.clear');
  if (btn) btn.remove();
}

/**
 * Auto-select a single option if the dropdown has exactly one.
 * Updates the URL parameter directly without triggering a render cycle.
 * Returns true if auto-selected.
 * @param {string} key - The select key
 * @returns {boolean}
 */
function autoSelectSingle(key) {
  var select = selects[key];
  if (!select || select.disabled) return false;
  var options = Array.from(select.options).filter(function(opt) {
    return opt.value && !opt.disabled;
  });
  if (options.length === 1) {
    select.value = options[0].value;
    // Update URL param directly, clearing dependent params
    var param = paramMap[key];
    if (param) {
      var url = new URL(location.href);
      url.searchParams.set(param, options[0].value);
      var dependentKeys = { search: ['search_revision_id', 'plan_id', 'run_id'], revision: ['plan_id', 'run_id'], plan: ['run_id'], run: [] };
      (dependentKeys[key] || []).forEach(function(dep) {
        url.searchParams.delete(dep);
      });
      history.replaceState({}, '', url.toString());
    }
    return true;
  }
  return false;
}

/**
 * Initialize per-dropdown enhancements.
 * Adds clear buttons to each dropdown.
 */
function initDropdownEnhancements() {
  Object.keys(selects).forEach(function(key) {
    var select = selects[key];
    if (!select) return;

    // Toggle clear button on change
    select.addEventListener('change', function() {
      if (select.value) {
        addClearButton(key);
      } else {
        removeClearButton(key);
      }
    });
  });
}

/** Selects options. */
function selectOptions(select, items, selected, label, labelFn) {
  var optionsHtml = '<option value="">' + esc(label) + '</option>';
  optionsHtml = optionsHtml + items.map(function(item) {
    const val = esc(pickID(item));
    var optionText;
    if (labelFn) {
      optionText = esc(labelFn(item));
    } else {
      optionText = esc(text(item, ['label', 'search_id', 'execution_fingerprint', 'name', 'title', 'fingerprint', 'id']));
    }
    return '<option value="' + val + '">' + optionText + '</option>';
  }).join('');

  select.innerHTML = optionsHtml;
  select.value = selected;
  if (items.length === 0) {
    select.disabled = true;
  } else {
    select.disabled = false;
  }
}

/** Asynchronously implements hydrate selectors for the viewer. */
export async function hydrateSelectors() {
  // Show skeletons on dependent dropdowns while fetching
  showSkeleton('revision');
  showSkeleton('plan');
  showSkeleton('run');
  hideDropdownError('search');
  hideDropdownError('revision');
  hideDropdownError('plan');
  hideDropdownError('run');

  if (!state.searches.length) {
    try {
      state.searches = list(await api('/api/searches'), ['searches', 'items']);
    } catch (err) {
      showDropdownError('search', 'Failed to load searches: ' + (err.message || err));
      return;
    }
  }

  const currentSearch = value('search_id');
  selectOptions(selects.search, state.searches, currentSearch, 'Select a search');
  hideSkeleton('search');

  // Auto-select if only one search (no URL change, cascades within this call)
  if (!currentSearch) {
    autoSelectSingle('search');
  }

  const search = state.searches.find(function(item) {
    return String(pickID(item)) === value('search_id');
  });
  if (!search) {
    hideSkeleton('revision');
    hideSkeleton('plan');
    hideSkeleton('run');
    selectOptions(selects.revision, [], '', 'Select a search');
    selectOptions(selects.plan, [], '', 'Select a search revision');
    selectOptions(selects.run, [], '', 'Select a search revision');
    document.querySelector('#selection-summary').textContent = 'Choose a search and its revision to inspect captured workspace evidence.';
    return;
  }

  const revisions = list(search, ['revisions', 'search_revisions']);
  selectOptions(selects.revision, revisions, value('search_revision_id'), 'Select a search revision');
  hideSkeleton('revision');

  // Auto-select if only one revision
  if (!value('search_revision_id')) {
    autoSelectSingle('revision');
  }

  if (!value('search_revision_id')) {
    hideSkeleton('plan');
    hideSkeleton('run');
    selectOptions(selects.plan, [], '', 'Select a search revision');
    selectOptions(selects.run, [], '', 'Select a search revision');
    document.querySelector('#selection-summary').textContent = 'Choose a search and its revision to inspect captured workspace evidence.';
    return;
  }

  try {
    const [plans, runs] = await Promise.all([
      api('/api/plans', { search_revision_id: value('search_revision_id') }),
      api('/api/runs', {
        search_revision_id: value('search_revision_id'),
        plan_id: value('plan_id'),
        include_trashed: 'true'
      }),
    ]);

    state.plans = list(plans, ['plans', 'items']);
    state.runs = list(runs, ['runs', 'items']);

    selectOptions(selects.plan, state.plans, value('plan_id'), 'Select an execution plan');
    hideSkeleton('plan');
    hideDropdownError('plan');

    // Auto-select if only one plan
    if (!value('plan_id')) {
      autoSelectSingle('plan');
    }

    selectOptions(selects.run, state.runs, value('run_id'), 'Select a run attempt', function(run) {
      const status = run.status || 'recorded';
      var trashed = '';
      if (run.visibility_state === 'trashed') {
        trashed = ' \u00B7 trashed';
      }
      var time = '';
      if (run.started_at) {
        time = ' \u00B7 ' + String(run.started_at).slice(0, 10);
      }
      return 'Run ' + pickID(run) + ' \u00B7 ' + status + trashed + time;
    });
    hideSkeleton('run');
    hideDropdownError('run');

    // Auto-select if only one run
    if (!value('run_id')) {
      autoSelectSingle('run');
    }

    const run = selectedRun();
    var summary;
    if (run) {
      summary = 'Inspecting run attempt ' + pickID(run) + ' (' + (run.status || 'recorded') + '). Pipeline evidence is immutable; completed runs may have local review versions.';
    } else {
      summary = 'Choose a run attempt to inspect its captured metrics, corpus, and provenance.';
    }
    document.querySelector('#selection-summary').textContent = summary;
  } catch (err) {
    hideSkeleton('plan');
    hideSkeleton('run');
    showDropdownError('plan', 'Failed to load plans: ' + (err.message || err));
    showDropdownError('run', 'Failed to load runs: ' + (err.message || err));
  }
}

// Initialize dropdown enhancements on module load
initDropdownEnhancements();
