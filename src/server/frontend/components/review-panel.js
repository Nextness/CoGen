// Run-scoped review context, complete status versions, notes, and PDF anchors.
import { api, mutate, APIError } from '../api.js';
import { esc, formatTime, humanLabel, link } from '../state.js';
import { mountNoteEditor } from './note-editor.js';
import { mountPDFViewer } from './pdf-viewer.js';

const statuses = ['not_evaluated', 'in_progress', 'approved', 'not_approved', 'removed'];
const substatuses = ['redacted', 'unrelated', 'out_of_scope', 'duplicate', 'retracted', 'withdrawn', 'superseded', 'predatory_low_quality', 'copyright_licensing', 'not_peer_reviewed'];

/** Mounts all editable review controls for one immutable run article revision. */
export async function mountArticleReview(host, pdfHost, record, detailData) {
  const runID = Number(record.pipeline_run_id);
  const revisionID = Number(record.id);
  const workID = Number(record.work_id);
  const health = await api('/api/health');
  const context = await api(`/api/runs/${runID}/review-context`);
  let pdfController = null;
  let pendingSelection = null;
  let reviewEditable = false;

  if (detailData.pdf_status?.status === 'available' && pdfHost) {
    pdfController = await mountPDFViewer(pdfHost, {
      url: `/api/pdf/${workID}`,
      page: Number(new URLSearchParams(location.search).get('pdf_page') || 1),
      onPageChange: function(page) { history.replaceState({}, '', link({ pdf_page: page })); },
      onSelection: function(selection) {
        pendingSelection = selection;
        renderAnchorCandidate();
      },
    }).catch(function(error) {
      pdfHost.innerHTML = '<p class="ui negative message">The embedded PDF could not be rendered. The original PDF remains available through the download endpoint: ' + esc(error.message) + '</p>';
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
  function renderStartReview(proposed) {
    const inherited = proposed ? '<p>The proposed parent is run ' + proposed.pipeline_run_id + ' from ' + esc(proposed.search_id) + ' / ' + esc(proposed.search_revision) + ', with ' + proposed.inherited_work_count + ' matching works. Inheritance is frozen when review starts.</p>' : '<p>No earlier review context was proposed. This run can start with empty review heads.</p>';
    host.innerHTML = '<section class="ui segment rw-review-panel"><h2>Article review</h2><p>This completed run has no review context.</p>' + inherited
      + '<button type="button" class="ui primary button" data-start-review>Start review</button><dialog data-review-dialog><form method="dialog"><h3>Start review context</h3>'
      + inherited + '<label>Parent context<select data-review-parent><option value="">Start empty</option>' + (proposed ? '<option selected value="' + proposed.context_id + '">Proposed run ' + proposed.pipeline_run_id + '</option>' : '') + '</select></label>'
      + '<div class="rw-filter-actions"><button type="button" data-all-review-candidates>Show contexts from all searches</button><span data-review-candidates aria-live="polite"></span></div><div class="rw-filter-actions"><button value="cancel">Cancel</button><button type="button" class="ui primary button" data-confirm-review>Initialize review</button></div></form></dialog></section>';
    const dialog = host.querySelector('[data-review-dialog]');
    /** Adds bounded eligible parents from same-search or explicitly expanded scope. */
    async function appendCandidates(scope) {
      const candidates = await api(`/api/runs/${runID}/review-context-candidates`, { scope: scope, limit: 100 });
      const select = host.querySelector('[data-review-parent]');
      let added = 0;
      for (const candidate of candidates.rows || []) {
        if (Array.from(select.options).some(function(option) { return option.value === String(candidate.context_id); })) continue;
        const option = document.createElement('option');
        option.value = candidate.context_id;
        option.textContent = `${candidate.search_id} / ${candidate.search_revision} / run ${candidate.pipeline_run_id} (${candidate.inherited_work_count} matching works)`;
        select.append(option);
        added += 1;
      }
      host.querySelector('[data-review-candidates]').textContent = added ? `${added} eligible context${added === 1 ? '' : 's'} added.` : 'No additional eligible contexts found.';
    }
    host.querySelector('[data-start-review]').addEventListener('click', async function() {
      dialog.showModal?.();
      if (!dialog.open) dialog.setAttribute('open', '');
      await appendCandidates('same_search');
    });
    host.querySelector('[data-all-review-candidates]').addEventListener('click', async function(event) {
      event.currentTarget.disabled = true;
      await appendCandidates('all');
    });
    host.querySelector('[data-confirm-review]').addEventListener('click', async function() {
      const raw = host.querySelector('[data-review-parent]').value;
      await mutate(`/api/runs/${runID}/review-context`, 'POST', { parent_context_id: raw ? Number(raw) : null });
      await renderReview();
    });
  }

  /** Loads and binds complete status state, history, notes, PDF, and anchors. */
  async function renderReview() {
    const data = await api(`/api/runs/${runID}/articles/${revisionID}/review`);
    reviewEditable = data.editable;
    const state = data.review || { version: null };
    const version = state.version;
    const selectedStatus = version?.status || 'not_evaluated';
    const selectedSubstatuses = new Set(version?.sub_statuses || []);
    host.innerHTML = '<section class="ui segment rw-review-panel"><div class="ui top attached header"><div><h2>Article review</h2><p>Run-scoped immutable evaluation state.</p></div>'
      + (state.inherited_from_context_id ? '<span class="ui label">Inherited from context ' + state.inherited_from_context_id + '</span>' : '') + '</div><div class="content">'
      + '<form class="ui form" data-review-form><label>Status<select data-review-status>' + statuses.map(function(status) { return '<option value="' + status + '"' + (selectedStatus === status ? ' selected' : '') + '>' + esc(humanLabel(status)) + '</option>'; }).join('') + '</select></label>'
      + '<fieldset data-review-substatuses><legend>Sub-statuses</legend>' + substatuses.map(function(status) { return '<label><input type="checkbox" value="' + status + '"' + (selectedSubstatuses.has(status) ? ' checked' : '') + '> ' + esc(humanLabel(status)) + '</label>'; }).join('') + '</fieldset>'
      + '<label>Optional reason<textarea rows="3" data-review-reason maxlength="32768">' + esc(version?.reason || '') + '</textarea></label>'
      + '<p data-review-message aria-live="polite">' + (version ? 'Version ' + version.id + ' by ' + esc(version.reviewer_display) + ' at ' + esc(formatTime(version.created_at)) + '.' : 'This status has not been explicitly saved.') + '</p>'
      + '<div class="rw-filter-actions"><button type="submit" class="ui primary button"' + (data.editable ? '' : ' disabled') + '>Save complete review state</button><button type="button" data-review-history>View history</button></div></form>'
      + '<div data-review-history-list></div><div data-note-host></div><section class="rw-anchor-panel"><h3>PDF anchors</h3><div data-anchor-candidate></div><div data-anchor-list></div></section></div></section>';
    const statusSelect = host.querySelector('[data-review-status]');
    const substatusField = host.querySelector('[data-review-substatuses]');
    /** Enables sub-status choices only for the two compatible terminal statuses. */
    function updateSubstatuses() {
      const enabled = statusSelect.value === 'not_approved' || statusSelect.value === 'removed';
      substatusField.disabled = !enabled;
      if (!enabled) substatusField.querySelectorAll('input').forEach(function(input) { input.checked = false; });
    }
    statusSelect.addEventListener('change', updateSubstatuses);
    updateSubstatuses();
    host.querySelector('[data-review-form]').addEventListener('submit', async function(event) {
      event.preventDefault();
      const message = host.querySelector('[data-review-message]');
      const reasonText = host.querySelector('[data-review-reason]').value.trim();
      try {
        await mutate(`/api/runs/${runID}/articles/${revisionID}/review`, 'PUT', {
          expected_version_id: version?.id || null,
          status: statusSelect.value,
          sub_statuses: Array.from(substatusField.querySelectorAll('input:checked')).map(function(input) { return input.value; }),
          reason: reasonText || null,
        });
        await renderReview();
      } catch (error) {
        message.textContent = error instanceof APIError && error.status === 409 ? 'A newer version exists. Your input is preserved; reload or inspect history before retrying.' : error.message;
      }
    });
    host.querySelector('[data-review-history]').addEventListener('click', async function() {
      const historyData = await api(`/api/runs/${runID}/articles/${revisionID}/review/versions`, { limit: 100 });
      host.querySelector('[data-review-history-list]').innerHTML = '<ol class="rw-review-history">' + (historyData.versions || []).map(function(item) {
        return '<li><strong>Version ' + item.id + ': ' + esc(humanLabel(item.status)) + '</strong> · ' + esc(item.reviewer_display) + ' · ' + esc(formatTime(item.created_at))
          + (item.reason ? '<blockquote>' + esc(item.reason) + '</blockquote>' : '') + (item.sub_statuses?.length ? '<p>' + item.sub_statuses.map(humanLabel).map(esc).join(', ') + '</p>' : '') + '</li>';
      }).join('') + '</ol>';
    });
    if (data.editable) {
      await mountNoteEditor(host.querySelector('[data-note-host]'), { corpusID: health.corpus_id, runID: runID, workRevisionID: revisionID });
    } else {
      host.querySelector('[data-note-host]').innerHTML = '<p class="ui faded text">An available PDF is required before review notes can be changed.</p>';
    }
    await loadAnchors();
    renderAnchorCandidate();
  }

  /** Converts one current PDF text selection into an accessible anchor creation form. */
  function renderAnchorCandidate() {
    const target = host.querySelector('[data-anchor-candidate]');
    if (!target || !pendingSelection || !reviewEditable) return;
    target.innerHTML = '<form class="ui form" data-anchor-form><p>Selected on page ' + pendingSelection.page + ': “' + esc(pendingSelection.selectedText) + '”</p><label>Anchor ID<input required pattern="[A-Za-z][A-Za-z0-9._-]{0,63}" data-anchor-id></label><button type="submit">Add anchor</button></form>';
    target.querySelector('form').addEventListener('submit', async function(event) {
      event.preventDefault();
      await mutate(`/api/runs/${runID}/articles/${revisionID}/anchors`, 'POST', { anchor_id: target.querySelector('[data-anchor-id]').value, page: pendingSelection.page, selected_text: pendingSelection.selectedText, rectangles: pendingSelection.rectangles });
      pendingSelection = null;
      target.textContent = '';
      await loadAnchors();
    });
  }

  /** Loads bounded active anchor heads, textual controls, and content-matched highlights. */
  async function loadAnchors() {
    const target = host.querySelector('[data-anchor-list]');
    if (!target) return;
    const data = await api(`/api/runs/${runID}/articles/${revisionID}/anchors`, { limit: 100 });
    const activeAnchors = data.anchors || [];
    target.innerHTML = activeAnchors.length ? '<ul class="rw-anchor-list">' + activeAnchors.map(function(anchor) {
      const mismatch = anchor.version.pdf_content_hash !== detailData.pdf_status?.content_hash;
      return '<li data-anchor-id="' + esc(anchor.id) + '"><button type="button" data-anchor-page="' + anchor.version.page + '" aria-label="Open anchor ' + esc(anchor.id) + ' on PDF page ' + anchor.version.page + '">' + esc(anchor.id) + '</button> · page ' + anchor.version.page
        + (anchor.inherited_from_context_id ? ' · inherited' : ' · current context') + (mismatch ? ' · unavailable for the current PDF content' : ' · available') + '<blockquote>' + esc(anchor.version.selected_text || '') + '</blockquote><button type="button" data-anchor-history>History</button> <button type="button" data-anchor-delete' + (reviewEditable ? '' : ' disabled') + '>Remove</button></li>';
    }).join('') + '</ul>' : '<p class="ui faded text">No active anchors. Select PDF text to add one, or use this keyboard-operable list to revisit existing anchors.</p>';
    pdfController?.setAnchors(activeAnchors.filter(function(anchor) { return anchor.version.pdf_content_hash === detailData.pdf_status?.content_hash; }));
    for (const anchor of activeAnchors) {
      const row = target.querySelector('[data-anchor-id="' + CSS.escape(anchor.id) + '"]');
      row.querySelector('[data-anchor-page]').addEventListener('click', function() {
        history.replaceState({}, '', link({ anchor_id: anchor.id, pdf_page: anchor.version.page }));
        pdfController?.goToPage(Number(anchor.version.page));
      });
      row.querySelector('[data-anchor-history]').addEventListener('click', function() { void showAnchorHistory(anchor.id); });
      row.querySelector('[data-anchor-delete]').addEventListener('click', async function() {
        if (!reviewEditable) return;
        await mutate(`/api/runs/${runID}/anchors/${encodeURIComponent(anchor.id)}/versions`, 'POST', { expected_version_id: anchor.version.id, state: 'deleted', page: 0, selected_text: '', rectangles: [] });
        history.replaceState({}, '', link({ anchor_id: anchor.id }));
        await showAnchorHistory(anchor.id);
        await loadAnchors();
      });
    }
    const focused = new URLSearchParams(location.search).get('anchor_id');
    if (focused && !activeAnchors.some(function(anchor) { return anchor.id === focused; })) await showAnchorHistory(focused);
  }

  /** Displays bounded immutable active and tombstone ancestry for a focused anchor. */
  async function showAnchorHistory(anchorID) {
    const target = host.querySelector('[data-anchor-list]');
    const data = await api(`/api/runs/${runID}/anchors/${encodeURIComponent(anchorID)}/versions`, { limit: 100 });
    const historyMarkup = '<section class="rw-anchor-history"><h4>Anchor ' + esc(anchorID) + ' history</h4><ol>' + (data.versions || []).map(function(version) {
      return '<li><strong>Version ' + version.id + ' · ' + esc(version.state) + '</strong> · ' + esc(version.reviewer_display) + ' · ' + esc(formatTime(version.created_at))
        + (version.state === 'active' ? ' · page ' + version.page + '<blockquote>' + esc(version.selected_text || '') + '</blockquote>' : ' · tombstone') + '</li>';
    }).join('') + '</ol></section>';
    target.querySelector('.rw-anchor-history')?.remove();
    target.insertAdjacentHTML('beforeend', historyMarkup);
  }
}
