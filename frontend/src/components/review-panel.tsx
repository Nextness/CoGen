// Run-scoped review context, complete status versions, notes, and PDF anchors.
import { api, mutate, APIError } from "../api.tsx";
import { formatTime, humanLabel, link } from "../state.tsx";
import { h, Fragment, render as renderTree } from "../jsx/jsx-runtime.ts";
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
  let setReviewSection: (name: string) => void = () => {};

  if (detailData.pdf_status?.status === "available" && pdfHost) {
    pdfController = await mountPDFViewer(pdfHost, {
      url: `/api/pdf/${workID}`,
      page: Number(new URLSearchParams(location.search).get("pdf_page") || 1),
      onPageChange: (page: number) => {
        history.replaceState({}, "", link({ pdf_page: page }));
      },
      onSelection: (selection: PDFSelection) => {
        pendingSelection = selection;
        setReviewSection("anchors");
        renderAnchorCandidate();
      },
    }).catch((error: any) => {
      const errorMarkup = <p className="ui negative message">The embedded PDF could not be rendered. The original PDF remains available through the download endpoint: {error.message}</p>;
      renderTree(errorMarkup, pdfHost!);
      return null;
    });
  }

  if (!context.context_initialized) {
    renderStartReview(context.proposed_parent);
    return { destroy: () => { return pdfController?.destroy(); } };
  }
  await renderReview();
  return { destroy: () => { return pdfController?.destroy(); } };

  /** Renders explicit context initialization with safe parent confirmation. */
  function renderStartReview(proposed: ProposedParent | null): void {
    var proposedSummary = "No earlier compatible review context was proposed. You can start this run with an empty review context.";
    if (proposed) {
      var plural = "s";
      if (proposed.inherited_work_count === 1) plural = "";
      proposedSummary = `Run ${proposed.pipeline_run_id} from ${proposed.search_id} / ${proposed.search_revision} contains ${proposed.inherited_work_count} matching work${plural}.`;
    }
    var recommendedOption: JSX.Element | null = null;
    if (proposed) {
      recommendedOption = <option selected value={proposed.context_id}>{`Recommended: run ${proposed.pipeline_run_id} · ${proposed.search_id} / ${proposed.search_revision} · ${proposed.inherited_work_count} matching`}</option>;
    }
    const startReviewMarkup = (
      <section className="ui segment rw-review-panel rw-review-panel--empty">
        <div className="ui top attached header">
          <div>
            <h3>Article review</h3>
            <p>Record a run-scoped decision, notes, and PDF anchors without changing pipeline evidence.</p>
          </div>
          <span className="ui label">Not started</span>
        </div>
        <div className="content">
          <div className="rw-review-onboarding">
            <div>
              <h4>Start a review context for this run</h4>
              <p>Starting review freezes any inherited article decisions, notes, and anchors so later changes remain independent.</p>
            </div>
            <button type="button" className="ui primary button" data-start-review>Start review</button>
          </div>
          <p className="ui info message"><span className="header">Suggested lineage</span>{proposedSummary}</p>
        </div>
        <dialog className="rw-review-dialog" data-review-dialog aria-labelledby="review-dialog-title" aria-describedby="review-dialog-description">
          <form className="ui form rw-review-dialog__form" data-review-context-form>
            <div className="rw-review-dialog__header">
              <div>
                <p className="rw-review-dialog__eyebrow">Review lineage</p>
                <h3 id="review-dialog-title">Start article review</h3>
                <p id="review-dialog-description">Choose which earlier review context to inherit, or start empty. This choice cannot be changed after initialization.</p>
              </div>
              <button type="button" className="ui icon basic button rw-review-dialog__close" data-review-close aria-label="Close review setup">{"\u00D7"}</button>
            </div>
            <div className="rw-review-dialog__body">
              <div className="ui info message">
                <span className="header">Recommended starting point</span>
                {`${proposedSummary} Inheritance is frozen when review starts.`}
              </div>
              <div className="ui field">
                <label htmlFor="review-parent-context">Parent review context</label>
                <div className="ui selection dropdown">
                  <select id="review-parent-context" data-review-parent>
                    <option value="">Start empty with no inherited review evidence</option>
                    {recommendedOption}
                  </select>
                </div>
                <p className="rw-field-help">Only earlier review contexts are eligible. Matching work heads are copied by immutable version reference.</p>
              </div>
              <section className="rw-review-candidate-panel" aria-labelledby="review-candidate-heading">
                <div>
                  <h4 id="review-candidate-heading">Available context scope</h4>
                  <p>Same-search contexts load automatically. Expand only when you intentionally need lineage from another search.</p>
                </div>
                <button type="button" className="ui basic button" data-all-review-candidates>Include all earlier searches</button>
              </section>
              <div className="ui info message rw-review-candidate-status" data-review-candidates aria-live="polite"><span className="header">Same-search contexts</span>Open this dialog to load eligible alternatives.</div>
            </div>
            <div className="rw-review-dialog__actions">
              <button type="button" className="ui basic button" data-review-cancel>Cancel</button>
              <button type="submit" className="ui primary button" data-confirm-review>Initialize review</button>
            </div>
          </form>
        </dialog>
      </section>
    );
    renderTree(startReviewMarkup, host);
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
      var scopeLabel = "same-search";
      if (scope === "all") scopeLabel = "cross-search";
      const loadingMarkup = (
        <>
          <span className="header">Searching review history</span>
          Loading eligible {scopeLabel} contexts.
        </>
      );
      renderTree(loadingMarkup, status);
      if (scope === "all") {
        expandButton.disabled = true;
        expandButton.classList.add("loading");
      }
      try {
        const candidates = await api(`/api/runs/${runID}/review-context-candidates`, { scope: scope, limit: 100 }, { method: "GET", headers: { Accept: "application/json" } });
        const select = host.querySelector("[data-review-parent]") as HTMLSelectElement;
        let added = 0;
        for (const candidate of candidates.rows || []) {
          const existingOptions = Array.from(select.options);
          if (existingOptions.some((option) => {
            return option.value === String(candidate.context_id);
          })) continue;
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
          var addedSummary = "No additional cross-search contexts were found. ";
          if (added) {
            var addedVerb = "s were";
            if (added === 1) addedVerb = " was";
            addedSummary = `${added} additional eligible context${addedVerb} added. `;
          }
          var totalVerb = "s are";
          if (total === 1) totalVerb = " is";
          const allMarkup = (
            <>
              <span className="header">All earlier searches checked</span>
              {addedSummary}{total} total parent option{totalVerb} available.
            </>
          );
          renderTree(allMarkup, status);
        } else {
          sameSearchLoaded = true;
          var sameSummary = "No same-search parent is available. Start empty or deliberately include all earlier searches.";
          if (total) {
            var sameVerb = "s are";
            if (total === 1) sameVerb = " is";
            sameSummary = `${total} eligible parent option${sameVerb} available, including the recommendation when present.`;
          }
          const sameMarkup = (
            <>
              <span className="header">Same-search contexts ready</span>
              {sameSummary}
            </>
          );
          renderTree(sameMarkup, status);
        }
      } catch (error: any) {
        status.className = "ui error message rw-review-candidate-status";
        const errorMarkup = (
          <>
            <span className="header">Context search failed</span>
            {error.message}
          </>
        );
        renderTree(errorMarkup, status);
        if (scope === "all") expandButton.disabled = false;
      } finally {
        expandButton.classList.remove("loading");
      }
    }
    host.querySelector("[data-start-review]")!.addEventListener("click", async () => {
      dialog.showModal?.();
      if (!dialog.open) dialog.setAttribute("open", "");
      if (!sameSearchLoaded) await appendCandidates("same_search");
    });
    host.querySelector("[data-all-review-candidates]")!.addEventListener("click", async () => {
      if (allSearchesLoaded) return;
      await appendCandidates("all");
    });
    host.querySelector("[data-review-close]")!.addEventListener("click", closeDialog);
    host.querySelector("[data-review-cancel]")!.addEventListener("click", closeDialog);
    dialog.addEventListener("click", (event) => {
      if (event.target === dialog) closeDialog();
    });
    host.querySelector("[data-review-context-form]")!.addEventListener("submit", async (event) => {
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
        const errorMarkup = (
          <>
            <span className="header">Review could not be initialized</span>
            {error.message}
          </>
        );
        renderTree(errorMarkup, status);
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
    var contextLabel: JSX.Element = <span className="ui neutral label">This context</span>;
    if (state.inherited_from_context_id) {
      contextLabel = <span className="ui violet label">Inherited from context {state.inherited_from_context_id}</span>;
    }
    const statusOptions = statuses.map((status) => {
      return <option value={status} selected={selectedStatus === status}>{humanLabel(status)}</option>;
    });
    const substatusOptions = substatuses.map((status) => {
      return (
        <label className="rw-review-check">
          <input type="checkbox" value={status} checked={selectedSubstatuses.has(status)} disabled={!data.editable} />
          <span>{humanLabel(status)}</span>
        </label>
      );
    });
    var feedbackMarkup: JSX.Element;
    if (version) {
      feedbackMarkup = (
        <>
          <span className="header">Current saved version</span>
          Version {version.id} by {version.reviewer_display} at {formatTime(version.created_at)}.
        </>
      );
    } else {
      feedbackMarkup = (
        <>
          <span className="header">No saved decision yet</span>
          The current context defaults to Not Evaluated until you save a complete review state.
        </>
      );
    }
    const reviewMarkup = (
      <section className="ui segment rw-review-panel">
        <div className="ui top attached header">
          <div>
            <h3>Article review</h3>
            <p>Run-scoped decisions, notes, and anchors append immutable versions.</p>
          </div>
          {contextLabel}
        </div>
        <div className="content">
          <nav className="ui tabular menu rw-review-nav" aria-label="Article review sections" role="tablist">
            <button id="review-tab-decision" type="button" className="item active" role="tab" data-review-section="decision" aria-selected="true" aria-controls="review-panel-decision" tabindex={0}>Decision</button>
            <button id="review-tab-notes" type="button" className="item" role="tab" data-review-section="notes" aria-selected="false" aria-controls="review-panel-notes" tabindex={-1}>Notes</button>
            <button id="review-tab-anchors" type="button" className="item" role="tab" data-review-section="anchors" aria-selected="false" aria-controls="review-panel-anchors" tabindex={-1}>PDF anchors</button>
          </nav>
          <section id="review-panel-decision" className="rw-review-section" role="tabpanel" data-review-section-panel="decision" aria-labelledby="review-tab-decision">
            <div className="rw-review-section__heading">
              <div>
                <h4 id="review-decision-heading">Review decision</h4>
                <p>Save the complete state for this article in the selected run context.</p>
              </div>
            </div>
            <form className="ui form rw-review-form" data-review-form>
              <div className="rw-review-form__primary">
                <div className="ui field">
                  <label htmlFor="article-review-status">Decision status</label>
                  <div className="ui selection dropdown">
                    <select id="article-review-status" data-review-status disabled={!data.editable}>{statusOptions}</select>
                  </div>
                </div>
                <div className="ui field">
                  <label htmlFor="article-review-reason">Reason or review summary <span className="rw-optional">Optional</span></label>
                  <textarea id="article-review-reason" rows={4} data-review-reason maxlength={32768} disabled={!data.editable}>{version?.reason || ""}</textarea>
                  <p className="rw-field-help">The saved reason is included in the append-only audit change for this decision.</p>
                </div>
              </div>
              <fieldset className="rw-review-substatuses" data-review-substatuses>
                <legend>Decision qualifiers</legend>
                <p>Select qualifiers only when the status is Not Approved or Removed.</p>
                <div className="rw-review-option-grid">{substatusOptions}</div>
              </fieldset>
              <div className="ui info message rw-review-feedback" data-review-message aria-live="polite">{feedbackMarkup}</div>
              <div className="rw-review-actions">
                <button type="submit" className="ui primary button" data-review-save disabled={!data.editable}>Save review decision</button>
                <button type="button" className="ui basic button" data-review-history aria-expanded="false">Show version history</button>
              </div>
            </form>
            <div className="rw-review-history-panel" data-review-history-list hidden></div>
          </section>
          <section id="review-panel-notes" className="rw-review-section" role="tabpanel" data-review-section-panel="notes" aria-labelledby="review-tab-notes" hidden>
            <div data-note-host></div>
          </section>
          <section id="review-panel-anchors" className="rw-review-section rw-anchor-panel" role="tabpanel" data-review-section-panel="anchors" aria-labelledby="review-tab-anchors" hidden>
            <div className="rw-review-section__heading">
              <div>
                <h4 id="review-anchors-heading">PDF anchors</h4>
                <p>Select text in the document reader, then save a named anchor for this review context.</p>
              </div>
            </div>
            <div data-anchor-candidate></div>
            <div data-anchor-list></div>
          </section>
        </div>
      </section>
    );
    renderTree(reviewMarkup, host);
    const sectionButtons = Array.from(host.querySelectorAll<HTMLButtonElement>("[data-review-section]"));
    const sectionPanels = Array.from(host.querySelectorAll<HTMLElement>("[data-review-section-panel]"));
    /** Switches visible review content without hiding its section identity or state. */
    setReviewSection = (name: string) => {
      sectionButtons.forEach((button) => {
        const active = button.dataset.reviewSection === name;
        button.classList.toggle("active", active);
        button.setAttribute("aria-selected", String(active));
        button.tabIndex = active ? 0 : -1;
      });
      sectionPanels.forEach((panel) => {
        panel.hidden = panel.dataset.reviewSectionPanel !== name;
      });
    };
    sectionButtons.forEach((button) => {
      button.addEventListener("click", () => {
        setReviewSection(button.dataset.reviewSection as string);
      });
    });
    host.querySelector<HTMLElement>(".rw-review-nav")!.addEventListener("keydown", (event: KeyboardEvent) => {
      const current = sectionButtons.indexOf(document.activeElement as HTMLButtonElement);
      if (current < 0) return;
      let target = current;
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
      if (!compatible) {
        const checkedInputs = substatusField.querySelectorAll("input");
        checkedInputs.forEach((input) => {
          (input as HTMLInputElement).checked = false;
        });
      }
    }
    statusSelect.addEventListener("change", updateSubstatuses);
    updateSubstatuses();
    host.querySelector("[data-review-form]")!.addEventListener("submit", async (event) => {
      event.preventDefault();
      const message = host.querySelector("[data-review-message]") as HTMLElement;
      const saveButton = host.querySelector("[data-review-save]") as HTMLButtonElement;
      const reasonText = (host.querySelector("[data-review-reason]") as HTMLTextAreaElement).value.trim();
      saveButton.disabled = true;
      saveButton.classList.add("loading");
      try {
        const checkedInputs = substatusField.querySelectorAll("input:checked");
        const saved = await mutate(`/api/runs/${runID}/articles/${revisionID}/review`, "PUT", {
          expected_version_id: version?.id || null,
          status: statusSelect.value,
          sub_statuses: Array.from(checkedInputs).map((input) => {
            return (input as HTMLInputElement).value;
          }),
          reason: reasonText || null,
        });
        await renderReview();
        if (saved.changed && onAuditChange) {
          try {
            await onAuditChange();
          } catch (error) {
            const currentMessage = host.querySelector("[data-review-message]") as HTMLElement;
            currentMessage.className = "ui warning message rw-review-feedback";
            const warningMarkup = (
              <>
                <span className="header">Decision saved</span>
                The audit display could not be refreshed. Reload the article to see the persisted event.
              </>
            );
            renderTree(warningMarkup, currentMessage);
          }
        }
      } catch (error: any) {
        message.className = "ui error message rw-review-feedback";
        var errorMessage = error.message;
        if (error instanceof APIError && error.status === 409) {
          errorMessage = "A newer version exists. Your input is preserved; inspect version history before retrying.";
        }
        const errorMarkup = (
          <>
            <span className="header">Review was not saved</span>
            {errorMessage}
          </>
        );
        renderTree(errorMarkup, message);
        saveButton.disabled = false;
        saveButton.classList.remove("loading");
      }
    });
    host.querySelector("[data-review-history]")!.addEventListener("click", async () => {
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
        const historyItems = (historyData.versions || []).map((item: any) => {
          var reasonMarkup: JSX.Element | null = null;
          if (item.reason) {
            reasonMarkup = <blockquote>{item.reason}</blockquote>;
          }
          var qualifiersMarkup: JSX.Element | null = null;
          if (item.sub_statuses?.length) {
            const qualifiers = item.sub_statuses.map(humanLabel);
            const qualifiersText = qualifiers.join(" · ");
            qualifiersMarkup = <p className="rw-review-qualifiers">{qualifiersText}</p>;
          }
          return (
            <li>
              <div>
                <strong>Version {item.id} · {humanLabel(item.status)}</strong>
                <p>{item.reviewer_display} · {formatTime(item.created_at)}</p>
              </div>
              {reasonMarkup}
              {qualifiersMarkup}
            </li>
          );
        });
        const historyMarkup = (
          <Fragment>
            <div className="rw-review-section__heading">
              <div>
                <h4>Version history</h4>
                <p>The newest immutable decision appears first.</p>
              </div>
            </div>
            <ol className="rw-review-history">{historyItems}</ol>
          </Fragment>
        );
        renderTree(historyMarkup, target);
        target.hidden = false;
        button.setAttribute("aria-expanded", "true");
        button.textContent = "Hide version history";
      } catch (error: any) {
        const errorMarkup = (
          <p className="ui error message">
            <span className="header">History could not be loaded</span>
            {error.message}
          </p>
        );
        renderTree(errorMarkup, target);
        target.hidden = false;
      } finally {
        button.disabled = false;
        button.classList.remove("loading");
      }
    });
    if (data.editable) {
      await mountNoteEditor(host.querySelector("[data-note-host]") as HTMLElement, { corpusID: health.corpus_id, runID: runID, workRevisionID: revisionID });
    } else {
      const noteLockedMarkup = <p className="ui faded text">An available PDF is required before review notes can be changed.</p>;
      renderTree(noteLockedMarkup, host.querySelector("[data-note-host]") as HTMLElement);
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
    const selection = pendingSelection;
    const anchorFormMarkup = (
      <form className="ui form rw-anchor-candidate" data-anchor-form>
        <div>
          <span className="ui blue label">Selection from page {selection.page}</span>
          <blockquote>{"\u201C"}{selection.selectedText}{"\u201D"}</blockquote>
        </div>
        <div className="ui field">
          <label htmlFor="review-anchor-id">Anchor ID</label>
          <input id="review-anchor-id" required pattern="[A-Za-z][A-Za-z0-9._-]{0,63}" placeholder="methods-sample" data-anchor-id />
          <p className="rw-field-help">Begin with a letter, then use letters, numbers, periods, underscores, or hyphens.</p>
        </div>
        <div className="rw-review-actions">
          <button type="submit" className="ui primary button" data-anchor-save>Save anchor</button>
          <button type="button" className="ui basic button" data-anchor-discard>Discard selection</button>
        </div>
        <p data-anchor-message aria-live="polite"></p>
      </form>
    );
    renderTree(anchorFormMarkup, targetElement);
    const discardButton = targetElement.querySelector("[data-anchor-discard]");
    discardButton!.addEventListener("click", () => {
      pendingSelection = null;
      targetElement.textContent = "";
      window.getSelection?.()?.removeAllRanges?.();
    });
    const anchorForm = targetElement.querySelector("form");
    anchorForm!.addEventListener("submit", async (event) => {
      event.preventDefault();
      const button = targetElement.querySelector("[data-anchor-save]") as HTMLButtonElement;
      const message = targetElement.querySelector("[data-anchor-message]") as HTMLElement;
      button.disabled = true;
      button.classList.add("loading");
      try {
        await mutate(`/api/runs/${runID}/articles/${revisionID}/anchors`, "POST", {
          anchor_id: (targetElement.querySelector("[data-anchor-id]") as HTMLInputElement).value,
          page: selection.page,
          selected_text: selection.selectedText,
          rectangles: selection.rectangles,
        });
        pendingSelection = null;
        targetElement.textContent = "";
        window.getSelection?.()?.removeAllRanges?.();
        await loadAnchors();
      } catch (error: any) {
        message.className = "ui error message";
        const errorMarkup = (
          <>
            <span className="header">Anchor was not saved</span>
            {error.message}
          </>
        );
        renderTree(errorMarkup, message);
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
    var anchorsMarkup: JSX.Element = <p className="ui faded text">No active anchors. Select PDF text to add one, or use this keyboard-operable list to revisit existing anchors.</p>;
    if (activeAnchors.length) {
      const anchorItems = activeAnchors.map((anchor) => {
        const mismatch = anchor.version.pdf_content_hash !== detailData.pdf_status?.content_hash;
        var contextLabel: JSX.Element = <span className="ui neutral label">This context</span>;
        if (anchor.inherited_from_context_id) {
          contextLabel = <span className="ui violet label">Inherited</span>;
        }
        var statusClass = "ui green label";
        var statusText = "Available";
        if (mismatch) {
          statusClass = "ui red label";
          statusText = "PDF changed";
        }
        return (
          <li data-anchor-id={anchor.id}>
            <div className="rw-anchor-card__meta">
              <div>
                <span className="ui label">{anchor.id}</span>
                <span className="ui label">Page {anchor.version.page}</span>
                {contextLabel}
              </div>
              <span className={statusClass}>{statusText}</span>
            </div>
            <blockquote>{anchor.version.selected_text || ""}</blockquote>
            <div className="rw-anchor-card__actions">
              <button type="button" className="ui primary button" data-anchor-page={anchor.version.page} aria-label={`Open anchor ${anchor.id} on PDF page ${anchor.version.page}`}>Open page {anchor.version.page}</button>
              <button type="button" className="ui basic button" data-anchor-history>History</button>
              <button type="button" className="ui danger button" data-anchor-delete disabled={!reviewEditable}>Remove</button>
            </div>
          </li>
        );
      });
      anchorsMarkup = <ul className="rw-anchor-list">{anchorItems}</ul>;
    }
    renderTree(anchorsMarkup, target);
    const matchedAnchors = activeAnchors.filter((anchor) => {
      return anchor.version.pdf_content_hash === detailData.pdf_status?.content_hash;
    });
    pdfController?.setAnchors(matchedAnchors);
    for (const anchor of activeAnchors) {
      const row = target.querySelector(`[data-anchor-id="${CSS.escape(anchor.id)}"]`) as HTMLElement;
      const pageButton = row.querySelector("[data-anchor-page]") as HTMLButtonElement;
      pageButton.addEventListener("click", () => {
        history.replaceState({}, "", link({ anchor_id: anchor.id, pdf_page: anchor.version.page }));
        pdfController?.goToPage(Number(anchor.version.page));
      });
      const historyButton = row.querySelector("[data-anchor-history]") as HTMLButtonElement;
      historyButton.addEventListener("click", () => { void showAnchorHistory(anchor.id); });
      const deleteButton = row.querySelector("[data-anchor-delete]") as HTMLButtonElement;
      deleteButton.addEventListener("click", async () => {
        if (!reviewEditable) return;
        await mutate(`/api/runs/${runID}/anchors/${encodeURIComponent(anchor.id)}/versions`, "POST", {
          expected_version_id: anchor.version.id,
          state: "deleted",
          page: 0,
          selected_text: "",
          rectangles: [],
        });
        history.replaceState({}, "", link({ anchor_id: anchor.id }));
        await showAnchorHistory(anchor.id);
        await loadAnchors();
      });
    }
    const focused = new URLSearchParams(location.search).get("anchor_id");
    if (focused && !activeAnchors.some((anchor) => {
      return anchor.id === focused;
    })) await showAnchorHistory(focused);
  }

  /** Displays bounded immutable active and tombstone ancestry for a focused anchor. */
  async function showAnchorHistory(anchorID: string): Promise<void> {
    const target = host.querySelector("[data-anchor-list]") as HTMLElement;
    const data = await api(`/api/runs/${runID}/anchors/${encodeURIComponent(anchorID)}/versions`, { limit: 100 }, { method: "GET", headers: { Accept: "application/json" } });
    const historyItems = (data.versions || []).map((version: any) => {
      var pageSummary = " · tombstone";
      if (version.state === "active") pageSummary = ` · page ${version.page}`;
      var quoteMarkup: JSX.Element | null = null;
      if (version.state === "active") {
        quoteMarkup = <blockquote>{version.selected_text || ""}</blockquote>;
      }
      return (
        <li>
          <div>
            <strong>Version {version.id} · {version.state}</strong>
            <p>{version.reviewer_display} · {formatTime(version.created_at)}{pageSummary}</p>
          </div>
          {quoteMarkup}
        </li>
      );
    });
    const historyNode = (
      <section className="rw-anchor-history">
        <div className="rw-review-section__heading">
          <div>
            <h4>Anchor {anchorID} history</h4>
            <p>The newest immutable anchor version appears first.</p>
          </div>
        </div>
        <ol>{historyItems}</ol>
      </section>
    );
    target.querySelector(".rw-anchor-history")?.remove();
    target.appendChild(historyNode);
  }
}
