// Unit tests for the shared chronological audit-event presentation.
import { describe, it } from 'node:test';
import assert from 'node:assert/strict';

import '../setup.ts';
import { auditCategory, AuditEventMarkup, AuditStream } from '../../../src/components/audit-events.tsx';
import { renderToString } from '../helpers/jsx-render.ts';

const auditEventMarkup = (event: any): string => renderToString(AuditEventMarkup({ event: event }));
const auditStream = (events: any[]): string => renderToString(AuditStream({ events: events }));

describe('audit-events.tsx', function() {
  it("shows run completion as a successful outcome with compact UTC time", () => {
    const event = AuditEventMarkup({ event: {
      id: 20,
      action: "run_completed",
      entity_type: "pipeline_run",
      entity_id: "7",
      pipeline_run_id: 7,
      occurred_at: "2026-09-02T03:22:27Z",
    } });
    const host = document.createElement("div");
    host.append(event);
    assert.equal(host.querySelector("time > span")!.textContent, "03:22:27");
    assert.equal(host.querySelector("time")!.getAttribute("datetime"), "2026-09-02T03:22:27Z");
    assert.match(host.querySelector("time")!.title, /UTC/);
    assert.match(host.textContent, /Pipeline execution finished successfully/);
    assert.equal(host.querySelector(".ui.green.label")!.textContent, "completed");
    const record = host.querySelector<HTMLAnchorElement>(".rw-audit-event__heading .rw-audit-event__entity a")!;
    assert.equal(JSON.parse(record.dataset.state!).run_id, "7");
    assert.ok(!host.textContent.includes("Recorded append-only audit event"));
  });

  it("summarizes validation results and only exposes recorded operational facts", () => {
    const html = auditEventMarkup({
      id: 21,
      action: "validation_changed",
      entity_type: "work",
      entity_id: "3475",
      actor: "pipeline",
      pipeline_run_id: 7,
      before_json: { status: "pending" },
      after_json: { status: "valid" },
      metadata_json: { reasons: [], stage: "validate", provider: "crossref", duration_seconds: 0, note_body: "must stay private" },
    });
    assert.ok(html.includes("Validation changed from Pending to Valid."));
    assert.ok(html.includes("Work #3475"));
    assert.ok(html.includes("<dt>Stage</dt><dd>Validate</dd>"));
    assert.ok(html.includes("<dt>Provider</dt><dd>crossref</dd>"));
    assert.ok(html.includes("<dt>Duration</dt><dd>0 s</dd>"));
    assert.ok(!html.includes("must stay private"));
    const discarded = auditEventMarkup({
      action: "validation_changed",
      after_json: { status: "discarded" },
      metadata_json: { reasons: ["Missing DOI", "Year < 2000"] },
    });
    assert.ok(discarded.includes("Missing DOI; Year &lt; 2000"));
    assert.ok(discarded.includes("discarded"));
  });

  it("does not invent summaries or results for sparse historical events", () => {
    const unknown = auditEventMarkup({ action: "custom_event" });
    assert.ok(!unknown.includes("Recorded append-only audit event"));
    assert.ok(unknown.includes("Time not recorded"));
    const validation = auditEventMarkup({ action: "validation_changed" });
    assert.ok(validation.includes("result was not recorded"));
    assert.ok(!validation.includes("Validation result: Valid"));
  });

  it('classifies review and PDF evidence independently from pipeline events', function() {
    assert.equal(auditCategory({ action: 'work_review_version_created' }), 'review');
    assert.equal(auditCategory({ action: 'pdf_inventory_registered' }), 'pdf');
    assert.equal(auditCategory({ action: 'pipeline_completed' }), 'pipeline');
  });

  it('shows outcome and run context from recorded payloads', function() {
    const html = auditEventMarkup({
      id: 8,
      action: 'validation_changed',
      actor: 'pipeline',
      pipeline_run_id: 3,
      entity_type: 'work_revision',
      entity_id: 4,
      occurred_at: '2024-01-20T10:00:00Z',
      after_json: '{"status":"discarded"}'
    });
    assert.ok(html.includes('rw-audit-event--validation'));
    assert.ok(html.includes('ui red label'));
    assert.ok(html.includes('Run 3'));
    assert.ok(html.includes('Recorded data'));
  });

  it('shows the complete previous and new review decision states', function() {
    const html = auditEventMarkup({
      id: 10,
      action: 'work_review_version_created',
      actor: 'reviewer',
      pipeline_run_id: 3,
      entity_type: 'work_review_version',
      entity_id: 8,
      occurred_at: '2024-01-20T10:00:00Z',
      before_json: '{"status":"approved","reason":"Initial evidence","sub_statuses":[]}',
      after_json: '{"status":"not_approved","reason":"Excluded <after review>","sub_statuses":["out_of_scope","not_peer_reviewed"]}'
    });
    assert.ok(html.includes('Review decision changed from Approved to Not Approved.'));
    assert.ok(html.includes('Previous decision'));
    assert.ok(html.includes('New decision'));
    assert.ok(html.includes('Initial evidence'));
    assert.ok(html.includes('Excluded &lt;after review&gt;'));
    assert.ok(html.includes('Out Of Scope'));
    assert.ok(html.includes('Not Peer Reviewed'));
  });

  it('does not invent decision details for historical review events without state payloads', function() {
    const html = auditEventMarkup({
      id: 11,
      action: 'work_review_version_created',
      actor: 'reviewer',
      pipeline_run_id: 3,
      entity_type: 'work_review_version',
      entity_id: 7,
      occurred_at: '2024-01-19T10:00:00Z'
    });
    assert.ok(html.includes('An immutable local review version was recorded.'));
    assert.ok(!html.includes('Review decision changed from'));
    assert.ok(!html.includes('Previous decision'));
  });

  it('renders chronological list semantics without exposing review prose or contact fields', function() {
    const html = auditStream([{
      id: 9,
      action: 'review_note_version_created',
      actor: 'reviewer',
      pipeline_run_id: 3,
      entity_type: 'work_revision',
      entity_id: 4,
      occurred_at: '2024-01-20T10:00:00Z',
      metadata_json: '{"note_body":"private prose","reviewer_email":"private@example.test","safe_id":12}',
      after_json: '{"body":"private version","status":"recorded"}'
    }]);
    assert.ok(html.includes('<ol class="rw-audit-events">'));
    assert.ok(html.includes('rw-audit-event--review'));
    assert.ok(html.includes('data-audit-recorded-details="9"'));
    assert.ok(html.includes('load privacy-scrubbed recorded JSON'));
    assert.ok(!html.includes('safe_id'));
    assert.ok(!html.includes('private prose'));
    assert.ok(!html.includes('private@example.test'));
    assert.ok(!html.includes('private version'));
  });
});
