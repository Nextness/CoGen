// Searchable, paged, hierarchical research-context selectors.
import { state, pickID, text, value } from "../state.tsx";
import { h, Fragment, render as renderTree, cx } from "../jsx/jsx-runtime.ts";
import { api, APIError, endpoint, errorMessage } from "../api.tsx";
import type { HierarchyAttempt, HierarchyItem, HierarchyPage, HierarchyPlan, HierarchySearch, RunContextResponse, WireRecord } from "../api/types.ts";
import { replaceState } from "../router.tsx";

/** Typed compound class names used by this module. */
const classNames = {
  uiErrorMessage: cx("ui", "error", "message"),
};

/** The four native context selects. */
export interface ContextSelects {
  search: HTMLSelectElement;
  revision: HTMLSelectElement;
  plan: HTMLSelectElement;
  run: HTMLSelectElement;
}

/** The stable native controls used as the form-value source for custom selectors. */
export const selects = {
  search: document.querySelector("#search-select") as HTMLSelectElement,
  revision: document.querySelector("#revision-select") as HTMLSelectElement,
  plan: document.querySelector("#plan-select") as HTMLSelectElement,
  run: document.querySelector("#run-select") as HTMLSelectElement,
};

/** One hierarchy item or exact selected run-context item shown in a selector. */
type SelectorItem = HierarchyItem | RunContextResponse["search"] | RunContextResponse["revision"];

/** One server-backed option query for a level in the context hierarchy. */
interface DropdownConfig {
  section: string;
  parent: Record<string, string>;
  placeholder: string;
  selectedID: string;
  label: (item: SelectorItem) => string;
}

/** One searchable dropdown presentation derived from its bounded native select. */
interface DropdownState {
  root: HTMLElement;
  trigger: HTMLButtonElement;
  menu: HTMLElement;
  query: HTMLInputElement;
  options: HTMLElement;
  nextCursor: string;
  sequence: number;
  timer: number | null;
}

const dropdowns: Record<string, DropdownState> = {};
const dropdownConfigs: Record<string, DropdownConfig> = {};
const hierarchyCache = new Map<string, HierarchyPage<HierarchyItem>>();

/** Clears validated option-page cache entries after tests or known hierarchy mutations. */
export function clearContextOptionCache(): void {
  hierarchyCache.clear();
}

/** Returns the human-readable label for one native select option. */
function optionLabel(option: HTMLOptionElement): string {
  return String(option?.textContent || "").trim();
}

/** Closes one searchable context selector and restores its unfiltered bounded option page. */
function closeDropdown(key: string): void {
  const dropdown = dropdowns[key];
  if (!dropdown) return;
  dropdown.menu.hidden = true;
  dropdown.trigger.setAttribute("aria-expanded", "false");
  dropdown.query.value = "";
  renderDropdownOptions(key);
}

/** Renders the current bounded native options and continuation action as one listbox. */
function renderDropdownOptions(key: string): void {
  const dropdown = dropdowns[key];
  const select = selects[key as keyof ContextSelects];
  if (!dropdown || !select) return;
  const nativeOptions = Array.from(select.options).filter((option) => {
    return Boolean(option.value);
  });
  const optionButtons = nativeOptions.map((option) => {
    const selected = option.value === select.value;
    const optionClass = cx("rw-search-dropdown__option", selected && "selected");
    return (
      <button
        type="button"
        className={optionClass}
        role="option"
        aria-selected={String(selected)}
        data-context-value={option.value}
      >
        {optionLabel(option)}
      </button>
    );
  });
  var empty: JSX.Element | null = null;
  if (!optionButtons.length) empty = <p className="rw-search-dropdown__empty">No matching values.</p>;
  var more: JSX.Element | null = null;
  if (dropdown.nextCursor) {
    more = (
      <button type="button" className="rw-search-dropdown__more" data-context-more>
        Load more values
      </button>
    );
  }
  const optionsMarkup = (
    <Fragment>
      {empty}
      {optionButtons}
      {more}
    </Fragment>
  );
  renderTree(optionsMarkup, dropdown.options);
}

/** Renders a concise selector loading announcement. */
function renderDropdownLoading(key: string): void {
  const dropdown = dropdowns[key];
  if (!dropdown) return;
  const loadingMarkup = <p className="rw-search-dropdown__empty" role="status">Searching available values.</p>;
  renderTree(loadingMarkup, dropdown.options);
}

/** Synchronizes the custom selector presentation with its native select source. */
function syncDropdown(key: string): void {
  const dropdown = dropdowns[key];
  const select = selects[key as keyof ContextSelects];
  if (!dropdown || !select) return;
  const selected = select.selectedOptions[0];
  var label = "";
  if (selected) label = optionLabel(selected);
  const hasValue = Boolean(select.value);
  dropdown.trigger.disabled = select.disabled;
  var triggerMarkup: JSX.Element = <span className="rw-search-dropdown__placeholder">{label || "Select a value"}</span>;
  if (hasValue) triggerMarkup = <span className="rw-search-dropdown__value">{label}</span>;
  renderTree(triggerMarkup, dropdown.trigger);
  renderDropdownOptions(key);
  if (select.disabled) closeDropdown(key);
}

/** Populates one native select from a bounded server page and synchronizes its presentation. */
function selectOptions(key: string, items: SelectorItem[], selected: string, config: DropdownConfig): void {
  const select = selects[key as keyof ContextSelects];
  const optionElements = items.map((item) => {
    return <option value={String(pickID(item) || "")}>{config.label(item)}</option>;
  });
  const optionsMarkup = (
    <Fragment>
      <option value="">{config.placeholder}</option>
      {optionElements}
    </Fragment>
  );
  renderTree(optionsMarkup, select);
  select.disabled = items.length === 0;
  select.value = selected;
  if (!select.value && selected) select.value = "";
  syncDropdown(key);
}

/** Appends a bounded continuation page without duplicating option identifiers. */
function appendOptions(key: string, items: SelectorItem[], config: DropdownConfig): void {
  const select = selects[key as keyof ContextSelects];
  const existing = new Set(Array.from(select.options).map((option) => {
    return option.value;
  }));
  items.forEach((item) => {
    const id = String(pickID(item));
    if (existing.has(id)) return;
    const option = document.createElement("option");
    option.value = id;
    option.textContent = config.label(item);
    select.append(option);
  });
  syncDropdown(key);
}

/** Fetches one validated hierarchy page and retains successful pages for later route renders. */
async function fetchOptionPage(config: DropdownConfig, query: string, cursor: string): Promise<HierarchyPage<HierarchyItem>> {
  const requestQuery = {
    section: config.section,
    ...config.parent,
    selected_id: config.selectedID,
    q: query,
    cursor: cursor,
  };
  const cacheKey = endpoint("/api/hierarchy", requestQuery);
  const cached = hierarchyCache.get(cacheKey);
  if (cached) return cached;
  const page = await api<HierarchyPage<HierarchyItem>>("/api/hierarchy", requestQuery, {
    method: "GET",
    headers: { Accept: "application/json" },
  });
  hierarchyCache.set(cacheKey, page);
  return page;
}

/** Loads a search or continuation page into one open selector without affecting sibling controls. */
async function loadDropdownPage(key: string, query: string, cursor = ""): Promise<void> {
  const dropdown = dropdowns[key];
  const config = dropdownConfigs[key];
  if (!dropdown || !config) return;
  const sequence = ++dropdown.sequence;
  renderDropdownLoading(key);
  try {
    const page = await fetchOptionPage(config, query, cursor);
    if (sequence !== dropdown.sequence) return;
    const items = page.items.slice();
    if (page.selected_item && !items.some((item) => {
      return String(pickID(item)) === String(pickID(page.selected_item));
    })) {
      items.unshift(page.selected_item);
    }
    dropdown.nextCursor = page.next_cursor || "";
    if (cursor) appendOptions(key, items, config);
    else selectOptions(key, items, selects[key as keyof ContextSelects].value, config);
    hideDropdownError(key);
  } catch (failure) {
    if (sequence !== dropdown.sequence) return;
    dropdown.nextCursor = "";
    const errorMarkup = <p className={classNames.uiErrorMessage} role="alert">{errorMessage(failure, "Unable to load context options.")}</p>;
    renderTree(errorMarkup, dropdown.options);
  }
}

/** Schedules a server search after the user pauses typing. */
function scheduleDropdownSearch(key: string): void {
  const dropdown = dropdowns[key];
  if (!dropdown) return;
  if (dropdown.timer !== null) window.clearTimeout(dropdown.timer);
  renderDropdownLoading(key);
  dropdown.timer = window.setTimeout(() => {
    dropdown.timer = null;
    void loadDropdownPage(key, dropdown.query.value.trim());
  }, 180);
}

/** Initializes one keyboard-operable searchable selector around its native select. */
function initializeDropdown(key: string): void {
  const select = selects[key as keyof ContextSelects];
  const root = document.querySelector<HTMLElement>(`[data-context-dropdown="${key}"]`);
  if (!select || !root) return;
  const dropdown: DropdownState = {
    root: root,
    trigger: root.querySelector<HTMLButtonElement>(".rw-search-dropdown__trigger")!,
    menu: root.querySelector<HTMLElement>(".rw-search-dropdown__menu")!,
    query: root.querySelector<HTMLInputElement>(".rw-search-dropdown__query")!,
    options: root.querySelector<HTMLElement>(".rw-search-dropdown__options")!,
    nextCursor: "",
    sequence: 0,
    timer: null,
  };
  dropdowns[key] = dropdown;
  const optionsID = `context-${key}-options`;
  dropdown.options.id = optionsID;
  dropdown.options.setAttribute("role", "listbox");
  dropdown.options.setAttribute("aria-live", "polite");
  dropdown.trigger.setAttribute("role", "combobox");
  dropdown.trigger.setAttribute("aria-haspopup", "listbox");
  dropdown.trigger.setAttribute("aria-controls", optionsID);
  dropdown.trigger.setAttribute("aria-expanded", "false");
  dropdown.query.setAttribute("aria-controls", optionsID);
  dropdown.query.setAttribute("autocomplete", "off");

  dropdown.trigger.addEventListener("click", () => {
    if (dropdown.trigger.disabled) return;
    const opening = dropdown.menu.hidden;
    Object.keys(dropdowns).forEach(closeDropdown);
    if (opening) {
      dropdown.menu.hidden = false;
      dropdown.trigger.setAttribute("aria-expanded", "true");
      dropdown.query.focus();
    }
  });
  dropdown.trigger.addEventListener("keydown", (event) => {
    if ((event.key === "ArrowDown" || event.key === "ArrowUp") && !dropdown.trigger.disabled) {
      event.preventDefault();
      dropdown.menu.hidden = false;
      dropdown.trigger.setAttribute("aria-expanded", "true");
      dropdown.query.focus();
    }
  });
  dropdown.query.addEventListener("input", () => {
    scheduleDropdownSearch(key);
  });
  dropdown.query.addEventListener("keydown", (event) => {
    const options = Array.from(dropdown.options.querySelectorAll<HTMLElement>("[role=option]"));
    if (event.key === "Escape") {
      event.preventDefault();
      closeDropdown(key);
      dropdown.trigger.focus();
    } else if (event.key === "ArrowDown") {
      event.preventDefault();
      options[0]?.focus();
    } else if (event.key === "ArrowUp") {
      event.preventDefault();
      options[options.length - 1]?.focus();
    } else if (event.key === "Enter") {
      event.preventDefault();
      options[0]?.click();
    }
  });
  dropdown.options.addEventListener("keydown", (event) => {
    const current = (event.target as HTMLElement).closest<HTMLElement>("[role=option]");
    if (!current) return;
    const options = Array.from(dropdown.options.querySelectorAll<HTMLElement>("[role=option]"));
    const currentIndex = options.indexOf(current);
    var destination = currentIndex;
    if (event.key === "ArrowDown") destination = Math.min(options.length - 1, currentIndex + 1);
    else if (event.key === "ArrowUp") destination = Math.max(0, currentIndex - 1);
    else if (event.key === "Home") destination = 0;
    else if (event.key === "End") destination = options.length - 1;
    else if (event.key === "Escape") {
      event.preventDefault();
      closeDropdown(key);
      dropdown.trigger.focus();
      return;
    } else if (event.key === "Enter" || event.key === " ") {
      event.preventDefault();
      current.click();
      return;
    } else {
      return;
    }
    event.preventDefault();
    if (event.key === "ArrowUp" && currentIndex === 0) dropdown.query.focus();
    else options[destination]?.focus();
  });
  dropdown.options.addEventListener("click", (event) => {
    const more = (event.target as HTMLElement).closest<HTMLElement>("[data-context-more]");
    if (more) {
      void loadDropdownPage(key, dropdown.query.value.trim(), dropdown.nextCursor);
      return;
    }
    const option = (event.target as HTMLElement).closest<HTMLElement>("[data-context-value]");
    if (!option) return;
    select.value = option.dataset.contextValue || "";
    closeDropdown(key);
    syncDropdown(key);
    select.dispatchEvent(new Event("change", { bubbles: true }));
    dropdown.trigger.focus();
  });
  root.addEventListener("focusout", () => {
    window.setTimeout(() => {
      if (!root.contains(document.activeElement)) closeDropdown(key);
    }, 0);
  });
  select.addEventListener("change", () => {
    syncDropdown(key);
  });
  syncDropdown(key);
}

/** Scrolls to and focuses the visible search selector trigger. */
export function focusContextSelector(): void {
  const dropdown = dropdowns.search;
  if (!dropdown) return;
  dropdown.root.scrollIntoView?.({ behavior: "smooth", block: "center" });
  dropdown.trigger.focus();
}

/** Shows one local selector-loading state without replacing the current page. */
function showLoading(key: string): void {
  const select = selects[key as keyof ContextSelects];
  if (!select) return;
  select.disabled = true;
  const placeholder = select.options[0];
  if (placeholder) placeholder.textContent = "Loading…";
  syncDropdown(key);
}

/** Shows an inline loading failure beside one context selector. */
function showDropdownError(key: string, message: string): void {
  const select = selects[key as keyof ContextSelects];
  const field = select?.closest<HTMLElement>(".ui.field");
  if (!field) return;
  field.querySelector(".ui.error.message")?.remove();
  const error = document.createElement("p");
  error.className = classNames.uiErrorMessage;
  error.setAttribute("role", "alert");
  error.textContent = message;
  field.append(error);
}

/** Removes an inline loading failure from one context selector. */
function hideDropdownError(key: string): void {
  const field = selects[key as keyof ContextSelects]?.closest<HTMLElement>(".ui.field");
  field?.querySelector(".ui.error.message")?.remove();
}

/** Replaces invalid or crossed hierarchy identifiers without starting a second render. */
function replaceContext(updates: Record<string, unknown>): void {
  replaceState(updates);
}

/** Reconciles a selected run to its server-owned complete ancestry. */
async function reconcileSelectedRun(): Promise<RunContextResponse | null> {
  const selectedRunID = value("run_id");
  if (!selectedRunID) return null;
  try {
    const context = await api<RunContextResponse>(`/api/runs/${encodeURIComponent(selectedRunID)}/context`, {}, {
      method: "GET",
      headers: { Accept: "application/json" },
    });
    const canonical = {
      search_id: String(context.search.id),
      search_revision_id: String(context.revision.id),
      plan_id: String(context.plan.id),
      run_id: String(context.run.id),
    };
    const crossed = Object.entries(canonical).some(([key, expected]) => {
      return value(key) !== expected;
    });
    if (crossed) replaceContext(canonical);
    return context;
  } catch (error) {
    if (error instanceof APIError && error.status === 404) {
      replaceContext({ run_id: "" });
      return null;
    }
    throw error;
  }
}

/** Adds one exact selected item to a page when it falls outside the first result window. */
function withSelectedItem(page: HierarchyPage<HierarchyItem>, canonicalItem: SelectorItem | null | undefined): SelectorItem[] {
  const items: SelectorItem[] = page.items.slice();
  const selectedItem = canonicalItem || page.selected_item;
  if (selectedItem && !items.some((item) => {
    return String(pickID(item)) === String(pickID(selectedItem));
  })) {
    items.unshift(selectedItem);
  }
  return items;
}

/** Returns the URL parameter owned by one selector level. */
function selectorParam(key: string): string {
  var name = "run_id";
  if (key === "search") name = "search_id";
  else if (key === "revision") name = "search_revision_id";
  else if (key === "plan") name = "plan_id";
  return name;
}

/** Loads and validates one hierarchy level, including sole-child selection. */
async function hydrateLevel(key: string, config: DropdownConfig, canonicalItem: SelectorItem | null | undefined, clearInvalid: Record<string, unknown>): Promise<{ items: SelectorItem[]; page: HierarchyPage<HierarchyItem> }> {
  dropdownConfigs[key] = config;
  showLoading(key);
  const page = await fetchOptionPage(config, "", "");
  const items = withSelectedItem(page, canonicalItem);
  dropdowns[key].nextCursor = page.next_cursor || "";
  const selected = value(selectorParam(key));
  const validSelected = items.some((item) => {
    return String(pickID(item)) === selected;
  });
  if (selected && !validSelected) replaceContext(clearInvalid);
  var effectiveSelected = selected;
  if (selected && !validSelected) effectiveSelected = "";
  if (!effectiveSelected && items.length === 1 && !page.has_more) {
    effectiveSelected = String(pickID(items[0]));
    replaceContext({ [selectorParam(key)]: effectiveSelected });
  }
  config.selectedID = effectiveSelected;
  selectOptions(key, items, effectiveSelected, config);
  hideDropdownError(key);
  return { items: items, page: page };
}

/** Loads the bounded context hierarchy required by the currently selected URL values. */
export async function hydrateSelectors(): Promise<void> {
  ["search", "revision", "plan", "run"].forEach(showLoading);
  Object.keys(selects).forEach(hideDropdownError);
  var selectedRunContext: RunContextResponse | null = null;
  try {
    selectedRunContext = await reconcileSelectedRun();
  } catch (error) {
    replaceContext({ run_id: "" });
    showDropdownError("run", `Failed to validate run context: ${errorMessage(error, "Unknown error")}`);
  }

  const searchConfig: DropdownConfig = {
    section: "searches",
    parent: {},
    placeholder: "Select a search",
    selectedID: value("search_id"),
    label: (item) => {
      return text(item, ["search_id", "label", "id"]);
    },
  };
  try {
    const searchPage = await hydrateLevel("search", searchConfig, selectedRunContext?.search, {
      search_id: "", search_revision_id: "", plan_id: "", run_id: "",
    });
    state.searches = searchPage.items as HierarchySearch[];
  } catch (error) {
    selectOptions("search", [], "", searchConfig);
    showDropdownError("search", `Failed to load searches: ${errorMessage(error, "Unknown error")}`);
    return;
  }

  if (!value("search_id")) {
    const revisionConfig: DropdownConfig = { section: "revisions", parent: {}, placeholder: "Select a search", selectedID: "", label: (item) => text(item, ["label", "id"]) };
    const planConfig: DropdownConfig = { section: "plans", parent: {}, placeholder: "Select a search revision", selectedID: "", label: (item) => text(item, ["execution_fingerprint", "id"]) };
    const runConfig: DropdownConfig = { section: "attempts", parent: {}, placeholder: "Select an execution plan", selectedID: "", label: (item) => text(item, ["id"]) };
    selectOptions("revision", [], "", revisionConfig);
    selectOptions("plan", [], "", planConfig);
    selectOptions("run", [], "", runConfig);
    return;
  }

  const revisionConfig: DropdownConfig = {
    section: "revisions",
    parent: { search_id: value("search_id") },
    placeholder: "Select a search revision",
    selectedID: value("search_revision_id"),
    label: (item) => {
      return text(item, ["label", "id"]);
    },
  };
  try {
    const revisionPage = await hydrateLevel("revision", revisionConfig, selectedRunContext?.revision, {
      search_revision_id: "", plan_id: "", run_id: "",
    });
    if (!revisionPage.items.length && value("search_revision_id")) replaceContext({ search_revision_id: "", plan_id: "", run_id: "" });
  } catch (error) {
    selectOptions("revision", [], "", revisionConfig);
    showDropdownError("revision", `Failed to load revisions: ${errorMessage(error, "Unknown error")}`);
    return;
  }
  if (!value("search_revision_id")) {
    const planConfig: DropdownConfig = { section: "plans", parent: {}, placeholder: "Select a search revision", selectedID: "", label: (item) => text(item, ["execution_fingerprint", "id"]) };
    const runConfig: DropdownConfig = { section: "attempts", parent: {}, placeholder: "Select an execution plan", selectedID: "", label: (item) => text(item, ["id"]) };
    selectOptions("plan", [], "", planConfig);
    selectOptions("run", [], "", runConfig);
    return;
  }

  const planConfig: DropdownConfig = {
    section: "plans",
    parent: { search_revision_id: value("search_revision_id") },
    placeholder: "Select an execution plan",
    selectedID: value("plan_id"),
    label: (item) => {
      const record = item as unknown as WireRecord;
      return `Plan ${pickID(item)} · ${String(record.execution_fingerprint || "No fingerprint").slice(0, 12)}`;
    },
  };
  try {
    const planPage = await hydrateLevel("plan", planConfig, selectedRunContext?.plan, { plan_id: "", run_id: "" });
    state.plans = planPage.items as HierarchyPlan[];
  } catch (error) {
    selectOptions("plan", [], "", planConfig);
    showDropdownError("plan", `Failed to load plans: ${errorMessage(error, "Unknown error")}`);
    return;
  }
  if (!value("plan_id")) {
    const runConfig: DropdownConfig = { section: "attempts", parent: {}, placeholder: "Select an execution plan", selectedID: "", label: (item) => text(item, ["id"]) };
    selectOptions("run", [], "", runConfig);
    return;
  }

  const runConfig: DropdownConfig = {
    section: "attempts",
    parent: { plan_id: value("plan_id") },
    placeholder: "Select a run attempt",
    selectedID: value("run_id"),
    label: (item) => {
      const record = item as unknown as WireRecord;
      var date = "";
      if (record.started_at) date = ` · ${String(record.started_at).slice(0, 10)}`;
      return `Run ${pickID(item)} · ${record.status || "recorded"}${date}`;
    },
  };
  try {
    const runPage = await hydrateLevel("run", runConfig, selectedRunContext?.run, { run_id: "" });
    state.runs = runPage.items as HierarchyAttempt[];
  } catch (error) {
    selectOptions("run", [], "", runConfig);
    showDropdownError("run", `Failed to load runs: ${errorMessage(error, "Unknown error")}`);
  }
}

Object.keys(selects).forEach(initializeDropdown);
document.addEventListener("click", (event) => {
  if (!(event.target as HTMLElement).closest("[data-context-dropdown]")) {
    Object.keys(dropdowns).forEach(closeDropdown);
  }
});
