// Searchable, hierarchical research-context selectors.
import { state, pickID, text, list, esc, value } from '../state.ts';
import { api } from '../api.ts';

/** The four native context selects. */
export interface ContextSelects {
  search: HTMLSelectElement;
  revision: HTMLSelectElement;
  plan: HTMLSelectElement;
  run: HTMLSelectElement;
}

export const selects = {
  search: document.querySelector('#search-select') as HTMLSelectElement,
  revision: document.querySelector('#revision-select') as HTMLSelectElement,
  plan: document.querySelector('#plan-select') as HTMLSelectElement,
  run: document.querySelector('#run-select') as HTMLSelectElement,
};

/** One searchable dropdown presentation derived from its native select. */
interface DropdownState {
  root: HTMLElement;
  trigger: HTMLButtonElement;
  menu: HTMLElement;
  query: HTMLInputElement;
  options: HTMLElement;
}

const dropdowns: Record<string, DropdownState> = {};

/** Closes one searchable context selector and restores its full option list. */
function closeDropdown(key: string): void {
  const dropdown = dropdowns[key];
  if (!dropdown) return;
  dropdown.menu.hidden = true;
  dropdown.trigger.setAttribute('aria-expanded', 'false');
  dropdown.query.value = '';
  renderDropdownOptions(key, '');
}

/** Returns the human-readable label for one native select option. */
function optionLabel(option: HTMLOptionElement): string {
  return String(option?.textContent || '').trim();
}

/** Renders the filtered listbox for one searchable context selector. */
function renderDropdownOptions(key: string, query: string): void {
  const dropdown = dropdowns[key];
  const select = selects[key as keyof ContextSelects];
  if (!dropdown || !select) return;
  const normalized = String(query || '').trim().toLocaleLowerCase();
  const options = Array.from(select.options).filter(function(option) {
    return option.value && (!normalized || optionLabel(option).toLocaleLowerCase().includes(normalized));
  });
  if (!options.length) {
    dropdown.options.innerHTML = '<p class="rw-search-dropdown__empty">No matching values.</p>';
    return;
  }
  dropdown.options.innerHTML = options.map(function(option) {
    const selected = option.value === select.value;
    return '<button type="button" class="rw-search-dropdown__option' + (selected ? ' selected' : '')
      + '" role="option" aria-selected="' + String(selected) + '" data-context-value="' + esc(option.value) + '">'
      + esc(optionLabel(option)) + '</button>';
  }).join('');
}

/** Synchronizes the custom selector presentation with its native select source. */
function syncDropdown(key: string): void {
  const dropdown = dropdowns[key];
  const select = selects[key as keyof ContextSelects];
  if (!dropdown || !select) return;
  const selected = select.selectedOptions[0];
  const label = selected ? optionLabel(selected) : '';
  const hasValue = Boolean(select.value);
  dropdown.trigger.disabled = select.disabled;
  dropdown.trigger.innerHTML = hasValue
    ? '<span class="rw-search-dropdown__value">' + esc(label) + '</span>'
    : '<span class="rw-search-dropdown__placeholder">' + esc(label || 'Select a value') + '</span>';
  renderDropdownOptions(key, dropdown.query.value);
  if (select.disabled) closeDropdown(key);
}

/** Initializes one keyboard-operable searchable selector around its native select. */
function initializeDropdown(key: string): void {
  const select = selects[key as keyof ContextSelects];
  const root = document.querySelector('[data-context-dropdown="' + key + '"]') as HTMLElement | null;
  if (!select || !root) return;
  const dropdown: DropdownState = {
    root: root,
    trigger: root.querySelector('.rw-search-dropdown__trigger') as HTMLButtonElement,
    menu: root.querySelector('.rw-search-dropdown__menu') as HTMLElement,
    query: root.querySelector('.rw-search-dropdown__query') as HTMLInputElement,
    options: root.querySelector('.rw-search-dropdown__options') as HTMLElement,
  };
  dropdowns[key] = dropdown;

  dropdown.trigger.addEventListener('click', function() {
    if (dropdown.trigger.disabled) return;
    const opening = dropdown.menu.hidden;
    Object.keys(dropdowns).forEach(closeDropdown);
    if (opening) {
      dropdown.menu.hidden = false;
      dropdown.trigger.setAttribute('aria-expanded', 'true');
      dropdown.query.focus();
    }
  });
  dropdown.trigger.addEventListener('keydown', function(event) {
    if (event.key === 'ArrowDown' && !dropdown.trigger.disabled) {
      event.preventDefault();
      dropdown.menu.hidden = false;
      dropdown.trigger.setAttribute('aria-expanded', 'true');
      dropdown.query.focus();
    }
  });
  dropdown.query.addEventListener('input', function() {
    renderDropdownOptions(key, dropdown.query.value);
  });
  dropdown.query.addEventListener('keydown', function(event) {
    if (event.key === 'Escape') {
      event.preventDefault();
      closeDropdown(key);
      dropdown.trigger.focus();
    } else if (event.key === 'ArrowDown') {
      event.preventDefault();
      dropdown.options.querySelector<HTMLElement>('[role="option"]')?.focus();
    } else if (event.key === 'Enter') {
      const first = dropdown.options.querySelector<HTMLElement>('[role="option"]');
      if (first) first.click();
    }
  });
  dropdown.options.addEventListener('keydown', function(event) {
    const current = (event.target as HTMLElement).closest<HTMLElement>('[role="option"]');
    if (!current) return;
    const options = Array.from(dropdown.options.querySelectorAll<HTMLElement>('[role="option"]'));
    const currentIndex = options.indexOf(current);
    if (event.key === 'ArrowDown') {
      event.preventDefault();
      options[Math.min(options.length - 1, currentIndex + 1)]?.focus();
    } else if (event.key === 'ArrowUp') {
      event.preventDefault();
      if (currentIndex === 0) dropdown.query.focus();
      else options[currentIndex - 1]?.focus();
    } else if (event.key === 'Escape') {
      event.preventDefault();
      closeDropdown(key);
      dropdown.trigger.focus();
    }
  });
  dropdown.options.addEventListener('click', function(event) {
    const option = (event.target as HTMLElement).closest<HTMLElement>('[data-context-value]');
    if (!option) return;
    select.value = (option.dataset.contextValue as string);
    closeDropdown(key);
    syncDropdown(key);
    select.dispatchEvent(new Event('change', { bubbles: true }));
    dropdown.trigger.focus();
  });
  select.addEventListener('change', function() { syncDropdown(key); });
  syncDropdown(key);
}

/** Shows one local selector-loading state without replacing the current page. */
function showLoading(key: string): void {
  const select = selects[key as keyof ContextSelects];
  if (!select) return;
  select.disabled = true;
  const placeholder = select.options[0];
  if (placeholder) placeholder.textContent = 'Loading…';
  syncDropdown(key);
}

/** Shows an inline loading failure beside one context selector. */
function showDropdownError(key: string, message: string): void {
  const select = selects[key as keyof ContextSelects];
  const field = select?.closest<HTMLElement>('.ui.field');
  if (!field) return;
  field.querySelector('.ui.error.message')?.remove();
  const error = document.createElement('p');
  error.className = 'ui error message';
  error.textContent = message;
  field.appendChild(error);
}

/** Removes an inline loading failure from one context selector. */
function hideDropdownError(key: string): void {
  selects[key as keyof ContextSelects]?.closest<HTMLElement>('.ui.field')?.querySelector('.ui.error.message')?.remove();
}

/** Populates one native select and synchronizes its searchable presentation. */
function selectOptions(select: HTMLSelectElement, items: any[], selected: string, label: string, labelFn?: (item: any) => string): void {
  const key = Object.keys(selects).find(function(name) { return selects[name as keyof ContextSelects] === select; });
  select.innerHTML = '<option value="">' + esc(label) + '</option>' + items.map(function(item) {
    const itemLabel = labelFn
      ? labelFn(item)
      : text(item, ['label', 'search_id', 'execution_fingerprint', 'name', 'title', 'fingerprint', 'id']);
    return '<option value="' + esc(pickID(item)) + '">' + esc(itemLabel) + '</option>';
  }).join('');
  select.disabled = items.length === 0;
  select.value = selected;
  if (!select.value && selected) select.value = '';
  if (key) syncDropdown(key);
}

/** Loads the context hierarchy required by the currently selected URL values. */
export async function hydrateSelectors(): Promise<void> {
  ['revision', 'plan', 'run'].forEach(showLoading);
  Object.keys(selects).forEach(hideDropdownError);
  if (!state.searches.length) {
    showLoading('search');
    try {
      state.searches = list(await api('/api/searches', {}, { method: 'GET', headers: { Accept: 'application/json' } }), ['searches', 'items']);
    } catch (error: any) {
      selectOptions(selects.search, [], '', 'Searches unavailable');
      showDropdownError('search', 'Failed to load searches: ' + (error.message || error));
      return;
    }
  }

  const currentSearch = value('search_id');
  selectOptions(selects.search, state.searches, currentSearch, 'Select a search');
  const search = state.searches.find(function(item) {
    return String(pickID(item)) === currentSearch;
  });
  if (!search) {
    selectOptions(selects.revision, [], '', 'Select a search');
    selectOptions(selects.plan, [], '', 'Select a search revision');
    selectOptions(selects.run, [], '', 'Select a search revision');
    return;
  }

  const revisions = list(search, ['revisions', 'search_revisions']);
  selectOptions(selects.revision, revisions, value('search_revision_id'), 'Select a search revision');
  if (!value('search_revision_id')) {
    selectOptions(selects.plan, [], '', 'Select a search revision');
    selectOptions(selects.run, [], '', 'Select a search revision');
    return;
  }

  try {
    const [plans, runs] = await Promise.all([
      api('/api/plans', { search_revision_id: value('search_revision_id') }, { method: 'GET', headers: { Accept: 'application/json' } }),
      api('/api/runs', {
        search_revision_id: value('search_revision_id'),
        plan_id: value('plan_id'),
        include_trashed: 'true'
      }, { method: 'GET', headers: { Accept: 'application/json' } }),
    ]);
    state.plans = list(plans, ['plans', 'items']);
    state.runs = list(runs, ['runs', 'items']);
    selectOptions(selects.plan, state.plans, value('plan_id'), 'Select an execution plan', function(plan) {
      return 'Plan ' + pickID(plan) + ' · ' + String(plan.execution_fingerprint || 'No fingerprint').slice(0, 12);
    });
    selectOptions(selects.run, state.runs, value('run_id'), 'Select a run attempt', function(run) {
      const trashed = run.visibility_state === 'trashed' ? ' · trashed' : '';
      const date = run.started_at ? ' · ' + String(run.started_at).slice(0, 10) : '';
      return 'Run ' + pickID(run) + ' · ' + (run.status || 'recorded') + trashed + date;
    });
  } catch (error: any) {
    selectOptions(selects.plan, [], '', 'Plans unavailable');
    selectOptions(selects.run, [], '', 'Runs unavailable');
    showDropdownError('plan', 'Failed to load plans: ' + (error.message || error));
    showDropdownError('run', 'Failed to load runs: ' + (error.message || error));
  }
}

Object.keys(selects).forEach(initializeDropdown);
document.addEventListener('click', function(event) {
  if (!(event.target as HTMLElement).closest('[data-context-dropdown]')) Object.keys(dropdowns).forEach(closeDropdown);
});