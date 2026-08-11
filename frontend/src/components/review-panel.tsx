// Run-scoped review context, complete status versions, notes, and PDF anchors.
import { api, mutate, APIError } from "../api.tsx";
import { esc, formatTime, humanLabel, link } from "../state.tsx";
import { mountNoteEditor } from "./note-editor.tsx";
import { mountPDFViewer } from "./pdf-viewer.tsx";

const statuses = ["not_evaluated", "in_progress", "approved", "not_approved", "removed"];
const substatuses = ["redacted", "unrelated", "out_of_scope", "duplicate", "retracted", "withdrawn", "superseded", "predatory_low_quality", "copyright_licensing", "not_peer_reviewed"];

/** One PDF text selection captured by the reader. */
export interface PDFSelection {
  page: number;
  selectedText: string;
  rectangles: any[];
}

/** One anchor head returned by the review anchors API. */
export interface AnchorHead {
  id: string;
  version: {
    id: any;
    page: number;
    selected_text?: string;
    pdf_content_hash?: string;
    state?: string;
    reviewer_display?: string;
    created_at?: any;
  };
  inherited_from_context_id?: any;
}

/** One proposed parent review context returned by the server. */
export interface ProposedParent {
  context_id: number;
  pipeline_run_id: number;
  search_id: any;
  search_revision: any;
  inherited_work_count: number;
}

/** Mounts all editable review controls for one immutable run article revision. */
export async function mountArticleReview(host: HTMLElement, pdfHost: HTMLElement | null, record: any, detailData: any, onAuditChange?: () => Promise<void>): Promise<{ destroy: () => any }> {
  const runID = Number(record.pipeline_run_id);
  const revisionID = Number(record.id);
  const workID = Number(record.work_id);
  const health = await api("/api/health", {}, { method: "GET", headers: { Accept: "application/json" } });
  const context = await api(`/api/runs/${runID}/review-context`, {}, { method: "GET", headers: { Accept: "application/json" } });
  let pdfController: any = null;
  let pendingSelection: PDFSelection | null = null;
  let reviewEditable = false;
  let setReviewSection: (name: string) => void = function() {};

  if (detailData.pdf_status?.status === "available" && pdfHost) {
    pdfController = await mountPDFViewer(pdfHost, {
      url: `/api/pdf/${workID}`,
      page: Number(new URLSearchParams(location.search).get("pdf_page") || 1),
      onPageChange: function(page: number) { history.replaceState({}, "", link({ pdf_page: page })); },
      onSelection: function(selection: PDFSelection) {
        pendingSelection = selection;
        setReviewSection("anchors");
        renderAnchorCandidate();
      },
    }).catch(function(error: any) {
      pdfHost!.innerHTML = `<p class="ui negative message">The embedded PDF could not be rendered. The original PDF remains available through the download endpoint: ` + esc(error.message) + "</p>";
      return null;
    });
  }

  if (!context.context_initialized) {
    renderStartReview(context.proposed_parent);
    return { destroy: function() { return pdfController?.destroy(); } };
  }
  await renderReview();
  return { destroy: function() { return pdfController?.destroy(); } };

  /** Renders explicit context initialization with safe parent confirmation. */
  function renderStartReview(proposed: ProposedParent | null): void {
    const proposedSummary = proposed
      ? "Run " + proposed.pipeline_run_id + " from " + esc(proposed.search_id) + " / " + esc(proposed.search_revision) + " contains " + proposed.inherited_work_count + " matching work" + (proposed.inherited_work_count === 1 ? "" : "s") + "."
      : "No earlier compatible review context was proposed. You can start this run with an empty review context.";
    const proposedOption = proposed ? `<option selected value="` + proposed.context_id + `">Recommended: run ` + proposed.pipeline_run_id + " · " + esc(proposed.search_id) + " / " + esc(proposed.search_revision) + " · " + proposed.inherited_work_count + " matching</option>" : "";
    host.innerHTML = `<section class="ui segment rw-review-panel rw-review-panel--empty"><div class="ui top attached header"><div><h3>Article review</h3><p>Record a run-scoped decision, notes, and PDF anchors without changing pipeline evidence.</p></div><span class="ui label">Not started</span></div>`
      + `<div class="content"><div class="rw-review-onboarding"><div><h4>Start a review context for this run</h4><p>Starting review freezes any inherited article decisions, notes, and anchors so later changes remain independent.</p></div><button type="button" class="ui primary button" data-start-review>Start review</button></div>`
      + `<p class="ui info message"><span class="header">Suggested lineage</span>` + proposedSummary + "</p></div>"
      + `<dialog class="rw-review-dialog" data-review-dialog aria-labelledby="review-dialog-title" aria-describedby="review-dialog-description"><form class="ui form rw-review-dialog__form" data-review-context-form>`
      + `<div class="rw-review-dialog__header"><div><p class="rw-review-dialog__eyebrow">Review lineage</p><h3 id="review-dialog-title">Start article review</h3><p id="review-dialog-description">Choose which earlier review context to inherit, or start empty. This choice cannot be changed after initialization.</p></div><button type="button" class="ui icon basic button rw-review-dialog__close" data-review-close aria-label="Close review setup">×</button></div>`
      + `<div class="rw-review-dialog__body"><div class="ui info message"><span class="header">Recommended starting point</span>` + proposedSummary + " Inheritance is frozen when review starts.</div>"
      + `<div class="ui field"><label for="review-parent-context">Parent review context</label><div class="ui selection dropdown"><select id="review-parent-context" data-review-parent><option value="">Start empty with no inherited review evidence</option>` + proposedOption + "</select></div><p class=\"rw-field-help\">Only earlier review contexts are eligible. Matching work heads are copied by immutable version reference.</p></div>"
      + `<section class="rw-review-candidate-panel" aria-labelledby="review-candidate-heading"><div><h4 id="review-candidate-heading">Available context scope</h4><p>Same-search contexts load automatically. Expand only when you intentionally need lineage from another search.</p></div><button type="button" class="ui basic button" data-all-review-candidates>Include all earlier searches</button></section>`
      + `<div class="ui info message rw-review-candidate-status" data-review-candidates aria-live="polite"><span class="header">Same-search contexts</span>Open this dialog to load eligible alternatives.</div></div>`
      + `<div class="rw-review-dialog__actions"><button type="button" class="ui basic button" data-review-cancel>Cancel</button><button type="submit" class="ui primary button" data-confirm-review>Initialize review</button></div></form></dialog></section>`;
    const dialog = host.querySelector("[data-review-dialog]") as HTMLDialogElement;
    let sameSearchLoaded = false;
    let allSearchesLoaded = false;
    /** Closes the setup dialog in browsers and test DOMs with partial dialog support. */
    function closeDialog(): void {
      if (typeof dialog.close === "function") dialog.close();
      else dialog.removeAttribute("open");
    }
    /** Adds bounded eligible parents from same-search or explicitly expanded scope. */
    async function appendCandidates(scope: string): Promise<void> {
      const status = host.querySelector("[data-review-candidates]") as HTMLElement;
      const expandButton = host.querySelector("[data-all-review-candidates]") as HTMLButtonElement;
      status.className = "ui info message rw-review-candidate-status";
      status.innerHTML = `<span class="header">Searching review history</span>Loading eligible ` + (scope === "all" ? "cross-search" : "same-search") + " contexts.";
      if (scope === "all") {
        expandButton.disabled = true;
        expandButton.classList.add("loading");
      }
      try {
        const candidates = await api(`/api/runs/${runID}/review-context-candidates`, { scope: scope, limit: 100 }, { method: "GET", headers: { Accept: "application/json" } });
        const select = host.querySelector("[data-review-parent]") as HTMLSelectElement;
        let added = 0;
        for (const candidate of candidates.rows || []) {
          if (Array.from(select.options).some(function(option) { return option.value === String(candidate.context_id); })) continue;
          const option = document.createElement("option");
          option.value = candidate.context_id;
          option.textContent = `${candidate.search_id} / ${candidate.search_revision} / run ${candidate.pipeline_run_id} · ${candidate.inherited_work_count} matching`;
          select.append(option);
          added += 1;
        }
        const total = Math.max(0, select.options.length - 1);
        if (scope === "all") {
          allSearchesLoaded = true;
          expandButton.textContent = "All earlier searches included";
          status.innerHTML = `<span class="header">All earlier searches checked</span>` + (added ? added + " additional eligible context" + (added === 1 ? " was" : "s were") + " added. " : "No additional cross-search contexts were found. ") + total + " total parent option" + (total === 1 ? " is" : "s are") + " available.";
        } else {
          sameSearchLoaded = true;
          status.innerHTML = `<span class="header">Same-search contexts ready</span>` + (total ? total + " eligible parent option" + (total === 1 ? " is" : "s are") + " available, including the recommendation when present." : "No same-search parent is available. Start empty or deliberately include all earlier searches.");
        }
      } catch (error: any) {
        status.className = "ui error message rw-review-candidate-status";
        status.innerHTML = `<span class="header">Context search failed</span>` + esc(error.message);
        if (scope === "all") expandButton.disabled = false;
      } finally {
        expandButton.classList.remove("loading");
      }
    }
    host.querySelector("[data-start-review]")!.addEventListener("click", async function() {
      dialog.showModal?.();
      if (!dialog.open) dialog.setAttribute("open", "");
      if (!sameSearchLoaded) await appendCandidates("same_search");
    });
    host.querySelector("[data-all-review-candidates]")!.addEventListener("click", async function() {
      if (allSearchesLoaded) return;
      await appendCandidates("all");
    });
    host.querySelector("[data-review-close]")!.addEventListener("click", closeDialog);
    host.querySelector("[data-review-cancel]")!.addEventListener("click", closeDialog);
    dialog.addEventListener("click", function(event) { if (event.target === dialog) closeDialog(); });
    host.querySelector("[data-review-context-form]")!.addEventListener("submit", async function(event) {
      event.preventDefault();
      const raw = (host.querySelector("[data-review-parent]") as HTMLSelectElement).value;
      const button = host.querySelector("[data-confirm-review]") as HTMLButtonElement;
      const status = host.querySelector("[data-review-candidates]") as HTMLElement;
      button.disabled = true;
      button.classList.add("loading");
      try {
        await mutate(`/api/runs/${runID}/review-context`, "POST", { parent_context_id: raw ? Number(raw) : null });
        closeDialog();
        await renderReview();
      } catch (error: any) {
        status.className = "ui error message rw-review-candidate-status";
        status.innerHTML = `<span class="header">Review could not be initialized</span>` + esc(error.message);
        button.disabled = false;
        button.classList.remove("loading");
      }
    });
  }

  /** Loads and binds complete status state, history, notes, PDF, and anchors. */
  async function renderReview(): Promise<void> {
    const data = await api(`/api/runs/${runID}/articles/${revisionID}/review`, {}, { method: "GET", headers: { Accept: "application/json" } });
    reviewEditable = data.editable;
    const state = data.review || { version: null };
    const version = state.version;
    const selectedStatus = version?.status || "not_evaluated";
    const selectedSubstatuses = new Set(version?.sub_statuses || []);
    const attribution = version ? `<span class="header">Current saved version</span>Version ` + version.id + " by " + esc(version.reviewer_display) + " at " + esc(formatTime(version.created_at)) + "." : `<span class="header">No saved decision yet</span>The current context defaults to Not Evaluated until you save a complete review state.`;
    const inheritedLabel = state.inherited_from_context_id ? `<span class="ui violet label">Inherited from context ` + state.inherited_from_context_id + "</span>" : `<span class="ui neutral label">This context</span>`;
    const disabled = data.editable ? "" : " disabled";
    host.innerHTML = `<section class="ui segment rw-review-panel"><div class="ui top attached header"><div><h3>Article review</h3><p>Run-scoped decisions, notes, and anchors append immutable versions.</p></div>` + inheritedLabel + `</div><div class="content">`
      + `<nav class="ui tabular menu rw-review-nav" aria-label="Article review sections" role="tablist"><button id="review-tab-decision" type="button" class="item active" role="tab" data-review-section="decision" aria-selected="true" aria-controls="review-panel-decision" tabindex="0">Decision</button><button id="review-tab-notes" type="button" class="item" role="tab" data-review-section="notes" aria-selected="false" aria-controls="review-panel-notes" tabindex="-1">Notes</button><button id="review-tab-anchors" type="button" class="item" role="tab" data-review-section="anchors" aria-selected="false" aria-controls="review-panel-anchors" tabindex="-1">PDF anchors</button></nav>`
      + `<section id="review-panel-decision" class="rw-review-section" role="tabpanel" data-review-section-panel="decision" aria-labelledby="review-tab-decision"><div class="rw-review-section__heading"><div><h4 id="review-decision-heading">Review decision</h4><p>Save the complete state for this article in the selected run context.</p></div></div>`
      + `<form class="ui form rw-review-form" data-review-form><div class="rw-review-form__primary"><div class="ui field"><label for="article-review-status">Decision status</label><div class="ui selection dropdown"><select id="article-review-status" data-review-status` + disabled + ">" + statuses.map(function(status) { return `<option value="` + status + `"` + (selectedStatus === status ? " selected" : "") + ">" + esc(humanLabel(status)) + "</option>"; }).join("") + "</select></div></div>"
      + `<div class="ui field"><label for="article-review-reason">Reason or review summary <span class="rw-optional">Optional</span></label><textarea id="article-review-reason" rows="4" data-review-reason maxlength="32768"` + disabled + ">" + esc(version?.reason || "") + "</textarea><p class=\"rw-field-help\">The saved reason is included in the append-only audit change for this decision.</p></div></div>"
      + `<fieldset class="rw-review-substatuses" data-review-substatuses><legend>Decision qualifiers</legend><p>Select qualifiers only when the status is Not Approved or Removed.</p><div class="rw-review-option-grid">` + substatuses.map(function(status) { return `<label class="rw-review-check"><input type="checkbox" value="` + status + `"` + (selectedSubstatuses.has(status) ? " checked" : "") + disabled + "><span>" + esc(humanLabel(status)) + "</span></label>"; }).join("") + "</div></fieldset>"
      + `<div class="ui info message rw-review-feedback" data-review-message aria-live="polite">` + attribution + "</div>"
      + `<div class="rw-review-actions"><button type="submit" class="ui primary button" data-review-save` + disabled + ">Save review decision</button><button type=\"button\" class=\"ui basic button\" data-review-history aria-expanded=\"false\">Show version history</button></div></form>"
      + `<div class="rw-review-history-panel" data-review-history-list hidden></div></section>`
      + `<section id="review-panel-notes" class="rw-review-section" role="tabpanel" data-review-section-panel="notes" aria-labelledby="review-tab-notes" hidden><div data-note-host></div></section>`
      + `<section id="review-panel-anchors" class="rw-review-section rw-anchor-panel" role="tabpanel" data-review-section-panel="anchors" aria-labelledby="review-tab-anchors" hidden><div class="rw-review-section__heading"><div><h4 id="review-anchors-heading">PDF anchors</h4><p>Select text in the document reader, then save a named anchor for this review context.</p></div></div><div data-anchor-candidate></div><div data-anchor-list></div></section></div></section>`;
    const sectionButtons = Array.from(host.querySelectorAll<HTMLButtonElement>("[data-review-section]"));
    const sectionPanels = Array.from(host.querySelectorAll<HTMLElement>("[data-review-section-panel]"));
    /** Switches visible review content without hiding its section identity or state. */
    setReviewSection = function(name: string) {
      sectionButtons.forEach(function(button) {
        const active = button.dataset.reviewSection === name;
        button.classList.toggle("active", active);
        button.setAttribute("aria-selected", String(active));
        button.tabIndex = active ? 0 : -1;
      });
      sectionPanels.forEach(function(panel) { panel.hidden = panel.dataset.reviewSectionPanel !== name; });
    };
    sectionButtons.forEach(function(button) { button.addEventListener("click", function() { setReviewSection(button.dataset.reviewSection as string); }); });
    host.querySelector<HTMLElement>(".rw-review-nav")!.addEventListener("keydown", function(event: KeyboardEvent) {
      const current = sectionButtons.indexOf(document.activeElement as HTMLButtonElement);
      if (current < 0) return;
      var target = current;
      if (event.key === "ArrowRight" || event.key === "ArrowDown") target = (current + 1) % sectionButtons.length;
      else if (event.key === "ArrowLeft" || event.key === "ArrowUp") target = (current - 1 + sectionButtons.length) % sectionButtons.length;
      else if (event.key === "Home") target = 0;
      else if (event.key === "End") target = sectionButtons.length - 1;
      else return;
      event.preventDefault();
      setReviewSection(sectionButtons[target].dataset.reviewSection as string);
      sectionButtons[target].focus();
    });
    const statusSelect = host.querySelector("[data-review-status]") as HTMLSelectElement;
    const substatusField = host.querySelector("[data-review-substatuses]") as HTMLFieldSetElement;
    /** Enables sub-status choices only for the two compatible terminal statuses. */
    function updateSubstatuses(): void {
      const compatible = statusSelect.value === "not_approved" || statusSelect.value === "removed";
      substatusField.disabled = !reviewEditable || !compatible;
      if (!compatible) substatusField.querySelectorAll("input").forEach(function(input) { (input as HTMLInputElement).checked = false; });
    }
    statusSelect.addEventListener("change", updateSubstatuses);
    updateSubstatuses();
    host.querySelector("[data-review-form]")!.addEventListener("submit", async function(event) {
      event.preventDefault();
      const message = host.querySelector("[data-review-message]") as HTMLElement;
      const saveButton = host.querySelector("[data-review-save]") as HTMLButtonElement;
      const reasonText = (host.querySelector("[data-review-reason]") as HTMLTextAreaElement).value.trim();
      saveButton.disabled = true;
      saveButton.classList.add("loading");
      try {
        const saved = await mutate(`/api/runs/${runID}/articles/${revisionID}/review`, "PUT", {
          expected_version_id: version?.id || null,
          status: statusSelect.value,
          sub_statuses: Array.from(substatusField.querySelectorAll("input:checked")).map(function(input) { return (input as HTMLInputElement).value; }),
          reason: reasonText || null,
        });
        await renderReview();
        if (saved.changed && onAuditChange) {
          try {
            await onAuditChange();
          } catch (error) {
            const currentMessage = host.querySelector("[data-review-message]") as HTMLElement;
            currentMessage.className = "ui warning message rw-review-feedback";
            currentMessage.innerHTML = `<span class="header">Decision saved</span>The audit display could not be refreshed. Reload the article to see the persisted event.`;
          }
        }
      } catch (error: any) {
        message.className = "ui error message rw-review-feedback";
        message.innerHTML = `<span class="header">Review was not saved</span>` + esc(error instanceof APIError && error.status === 409 ? "A newer version exists. Your input is preserved; inspect version history before retrying." : error.message);
        saveButton.disabled = false;
        saveButton.classList.remove("loading");
      }
    });
    host.querySelector("[data-review-history]")!.addEventListener("click", async function() {
      const button = host.querySelector("[data-review-history]") as HTMLButtonElement;
      const target = host.querySelector("[data-review-history-list]") as HTMLElement;
      if (!target.hidden) {
        target.hidden = true;
        button.setAttribute("aria-expanded", "false");
        button.textContent = "Show version history";
        return;
      }
      button.disabled = true;
      button.classList.add("loading");
      try {
        const historyData = await api(`/api/runs/${runID}/articles/${revisionID}/review/versions`, { limit: 100 }, { method: "GET", headers: { Accept: "application/json" } });
        target.innerHTML = `<div class="rw-review-section__heading"><div><h4>Version history</h4><p>The newest immutable decision appears first.</p></div></div><ol class="rw-review-history">` + (historyData.versions || []).map(function(item: any) {
          return "<li><div><strong>Version " + item.id + " · " + esc(humanLabel(item.status)) + "</strong><p>" + esc(item.reviewer_display) + " · " + esc(formatTime(item.created_at)) + "</p></div>"
            + (item.reason ? "<blockquote>" + esc(item.reason) + "</blockquote>" : "") + (item.sub_statuses?.length ? `<p class="rw-review-qualifiers">` + item.sub_statuses.map(humanLabel).map(esc).join(" · ") + "</p>" : "") + "</li>";
        }).join("") + "</ol>";
        target.hidden = false;
        button.setAttribute("aria-expanded", "true");
        button.textContent = "Hide version history";
      } catch (error: any) {
        target.innerHTML = `<p class="ui error message"><span class="header">History could not be loaded</span>` + esc(error.message) + "</p>";
        target.hidden = false;
      } finally {
        button.disabled = false;
        button.classList.remove("loading");
      }
    });
    if (data.editable) {
      await mountNoteEditor(host.querySelector("[data-note-host]") as HTMLElement, { corpusID: health.corpus_id, runID: runID, workRevisionID: revisionID });
    } else {
      (host.querySelector("[data-note-host]") as HTMLElement).innerHTML = `<p class="ui faded text">An available PDF is required before review notes can be changed.</p>`;
    }
    await loadAnchors();
    renderAnchorCandidate();
    if (pendingSelection || new URLSearchParams(location.search).get("anchor_id")) setReviewSection("anchors");
    else if (new URLSearchParams(location.search).get("note_id")) setReviewSection("notes");
  }

  /** Converts one current PDF text selection into an accessible anchor creation form. */
  function renderAnchorCandidate(): void {
    const targetElement = host.querySelector<HTMLElement>("[data-anchor-candidate]");
    if (!targetElement || !pendingSelection || !reviewEditable) return;
    const target = targetElement;
    const selection = pendingSelection;
    target.innerHTML = `<form class="ui form rw-anchor-candidate" data-anchor-form><div><span class="ui blue label">Selection from page ` + selection.page + "</span><blockquote>“" + esc(selection.selectedText) + "”</blockquote></div>"
      + `<div class="ui field"><label for="review-anchor-id">Anchor ID</label><input id="review-anchor-id" required pattern="[A-Za-z][A-Za-z0-9._-]{0,63}" placeholder="methods-sample" data-anchor-id><p class="rw-field-help">Begin with a letter, then use letters, numbers, periods, underscores, or hyphens.</p></div>`
      + `<div class="rw-review-actions"><button type="submit" class="ui primary button" data-anchor-save>Save anchor</button><button type="button" class="ui basic button" data-anchor-discard>Discard selection</button></div><p data-anchor-message aria-live="polite"></p></form>`;
    target.querySelector("[data-anchor-discard]")!.addEventListener("click", function() {
      pendingSelection = null;
      target.textContent = "";
      window.getSelection?.()?.removeAllRanges?.();
    });
    target.querySelector("form")!.addEventListener("submit", async function(event) {
      event.preventDefault();
      const button = target.querySelector("[data-anchor-save]") as HTMLButtonElement;
      const message = target.querySelector("[data-anchor-message]") as HTMLElement;
      button.disabled = true;
      button.classList.add("loading");
      try {
        await mutate(`/api/runs/${runID}/articles/${revisionID}/anchors`, "POST", { anchor_id: (target.querySelector("[data-anchor-id]") as HTMLInputElement).value, page: selection.page, selected_text: selection.selectedText, rectangles: selection.rectangles });
        pendingSelection = null;
        target.textContent = "";
        window.getSelection?.()?.removeAllRanges?.();
        await loadAnchors();
      } catch (error: any) {
        message.className = "ui error message";
        message.innerHTML = `<span class="header">Anchor was not saved</span>` + esc(error.message);
        button.disabled = false;
        button.classList.remove("loading");
      }
    });
  }

  /** Loads bounded active anchor heads, textual controls, and content-matched highlights. */
  async function loadAnchors(): Promise<void> {
    const target = host.querySelector("[data-anchor-list]") as HTMLElement | null;
    if (!target) return;
    const data = await api(`/api/runs/${runID}/articles/${revisionID}/anchors`, { limit: 100 }, { method: "GET", headers: { Accept: "application/json" } });
    const activeAnchors: AnchorHead[] = data.anchors || [];
    target.innerHTML = activeAnchors.length ? `<ul class="rw-anchor-list">` + activeAnchors.map(function(anchor) {
      const mismatch = anchor.version.pdf_content_hash !== detailData.pdf_status?.content_hash;
      return `<li data-anchor-id="` + esc(anchor.id) + `"><div class="rw-anchor-card__meta"><div><span class="ui label">` + esc(anchor.id) + `</span><span class="ui label">Page ` + anchor.version.page + `</span>` + (anchor.inherited_from_context_id ? `<span class="ui violet label">Inherited</span>` : `<span class="ui neutral label">This context</span>`) + "</div><span class=\"ui " + (mismatch ? "red" : "green") + " label\">" + (mismatch ? "PDF changed" : "Available") + "</span></div>"
        + "<blockquote>" + esc(anchor.version.selected_text || "") + "</blockquote><div class=\"rw-anchor-card__actions\"><button type=\"button\" class=\"ui primary button\" data-anchor-page=\"" + anchor.version.page + "\" aria-label=\"Open anchor " + esc(anchor.id) + " on PDF page " + anchor.version.page + "\">Open page " + anchor.version.page + "</button><button type=\"button\" class=\"ui basic button\" data-anchor-history>History</button><button type=\"button\" class=\"ui danger button\" data-anchor-delete" + (reviewEditable ? "" : " disabled") + ">Remove</button></div></li>";
    }).join("") + "</ul>" : `<p class="ui faded text">No active anchors. Select PDF text to add one, or use this keyboard-operable list to revisit existing anchors.</p>`;
    pdfController?.setAnchors(activeAnchors.filter(function(anchor) { return anchor.version.pdf_content_hash === detailData.pdf_status?.content_hash; }));
    for (const anchor of activeAnchors) {
      const row = target.querySelector(`[data-anchor-id="` + CSS.escape(anchor.id) + `"]`) as HTMLElement;
      (row.querySelector("[data-anchor-page]") as HTMLButtonElement).addEventListener("click", function() {
        history.replaceState({}, "", link({ anchor_id: anchor.id, pdf_page: anchor.version.page }));
        pdfController?.goToPage(Number(anchor.version.page));
      });
      (row.querySelector("[data-anchor-history]") as HTMLButtonElement).addEventListener("click", function() { void showAnchorHistory(anchor.id); });
      (row.querySelector("[data-anchor-delete]") as HTMLButtonElement).addEventListener("click", async function() {
        if (!reviewEditable) return;
        await mutate(`/api/runs/${runID}/anchors/${encodeURIComponent(anchor.id)}/versions`, "POST", { expected_version_id: anchor.version.id, state: "deleted", page: 0, selected_text: "", rectangles: [] });
        history.replaceState({}, "", link({ anchor_id: anchor.id }));
        await showAnchorHistory(anchor.id);
        await loadAnchors();
      });
    }
    const focused = new URLSearchParams(location.search).get("anchor_id");
    if (focused && !activeAnchors.some(function(anchor) { return anchor.id === focused; })) await showAnchorHistory(focused);
  }

  /** Displays bounded immutable active and tombstone ancestry for a focused anchor. */
  async function showAnchorHistory(anchorID: string): Promise<void> {
    const target = host.querySelector("[data-anchor-list]") as HTMLElement;
    const data = await api(`/api/runs/${runID}/anchors/${encodeURIComponent(anchorID)}/versions`, { limit: 100 }, { method: "GET", headers: { Accept: "application/json" } });
    const historyMarkup = `<section class="rw-anchor-history"><div class="rw-review-section__heading"><div><h4>Anchor ` + esc(anchorID) + ` history</h4><p>The newest immutable anchor version appears first.</p></div></div><ol>` + (data.versions || []).map(function(version: any) {
      return "<li><div><strong>Version " + version.id + " · " + esc(version.state) + "</strong><p>" + esc(version.reviewer_display) + " · " + esc(formatTime(version.created_at)) + (version.state === "active" ? " · page " + version.page : " · tombstone") + "</p></div>"
        + (version.state === "active" ? "<blockquote>" + esc(version.selected_text || "") + "</blockquote>" : "") + "</li>";
    }).join("") + "</ol></section>";
    target.querySelector(".rw-anchor-history")?.remove();
    target.insertAdjacentHTML("beforeend", historyMarkup);
  }
}