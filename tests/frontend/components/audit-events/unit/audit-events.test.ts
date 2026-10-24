// Unit tests for the shared chronological audit-event presentation.
import { describe, it } from 'node:test';
import assert from 'node:assert/strict';

import "../../../../support/node/setup.ts";
import { auditCategory, AuditEventMarkup, AuditStream, bindAuditRecordedData } from "../../../../../frontend/src/components/audit-events.tsx";
import { apiResponse } from "../../../../support/node/helpers/fetch.ts";
import { renderToString } from "../../../../support/node/helpers/jsx-render.ts";

const auditEventMarkup = (event: any): string => renderToString(AuditEventMarkup({ event: event }));
const auditStream = (events: any[]): string => renderToString(AuditStream({ events: events }));

describe('audit-events.tsx', function() {
  it("expands readable changes once and separates recorded JSON from the default presentation", async () => {
    const originalFetch = globalThis.fetch;
    var requests = 0;
    globalThis.fetch = (async () => {
      requests += 1;
      return apiResponse({
        event_id: 22,
        byte_limit: 16384,
        metadata: { checked: false, count: 0 },
        before: { unchanged: "same" },
        after: { status: "valid", empty_value: null, unchanged: "same", title: "<img src=x>", flags: ["has_doi"] },
      });
    }) as typeof fetch;

    try {
      const host = document.createElement("div");
      host.append(AuditEventMarkup({ event: { id: 22, action: "validation_changed" } }));
      bindAuditRecordedData(host);
      assert.equal(requests, 0);
      const details = host.querySelector<HTMLDetailsElement>(".rw-event-details")!;
      details.open = true;
      details.dispatchEvent(new window.Event("toggle"));
      await new Promise((resolve) => setTimeout(resolve, 0));

      assert.equal(requests, 1);
      assert.equal(details.querySelector("summary")!.getAttribute("aria-expanded"), "true");
      const changes = details.querySelector<HTMLTableElement>(".rw-event-changes")!;
      assert.match(changes.textContent, /StatusNot recordedvalid/);
      assert.match(changes.textContent, /Empty ValueNot recordedNone/);
      assert.ok(!changes.textContent.includes("Unchanged"));
      assert.equal(changes.querySelector("img"), null);
      assert.ok(changes.textContent.includes("<img src=x>"));
      assert.ok(details.textContent.includes("CheckedNo"));
      assert.ok(details.textContent.includes("Count0"));
      assert.equal(details.querySelector<HTMLDetailsElement>("[data-audit-json]")!.open, false);

      details.open = false;
      details.dispatchEvent(new window.Event("toggle"));
      details.open = true;
      details.dispatchEvent(new window.Event("toggle"));
      await new Promise((resolve) => setTimeout(resolve, 0));
      assert.equal(requests, 1);
    } finally {
      globalThis.fetch = originalFetch;
    }
  });

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
    assert.equal(host.querySelector(".ui.green.label")!.textContent, "Completed");
    const record = host.querySelector<HTMLAnchorElement>(".rw-audit-event__row .rw-audit-event__entity a")!;
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
    assert.ok(html.includes("<dt>Stage</dt><dd>validate</dd>"));
    assert.ok(html.includes("<dt>Provider</dt><dd>crossref</dd>"));
    assert.ok(html.includes("<dt>Duration</dt><dd>0 seconds</dd>"));
    assert.ok(!html.includes("must stay private"));
    const discarded = auditEventMarkup({
      action: "validation_changed",
      after_json: { status: "discarded" },
      metadata_json: { reasons: ["Missing DOI", "Year < 2000"] },
    });
    assert.ok(discarded.includes("Missing DOI; Year &lt; 2000"));
    assert.ok(discarded.includes("Discarded"));
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

  it("distinguishes enriched fields and uses readable PDF and outcome labels", () => {
    const enriched = auditEventMarkup({ action: "field_enriched", metadata_json: { field: "title", provider: "crossref" } });
    assert.ok(enriched.includes("Title Enriched"));
    const pdf = auditEventMarkup({ action: "pdf_inventory_registered", after_json: { status: "not_available" } });
    assert.ok(pdf.includes("PDF Inventory Registered"));
    assert.ok(pdf.includes("Not Available"));
    assert.ok(!pdf.includes(">not_available<"));
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
    assert.ok(html.includes('aria-controls="audit-event-8-information"'));
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
    assert.ok(html.includes('load privacy-scrubbed recorded evidence'));
    assert.ok(!html.includes('safe_id'));
    assert.ok(!html.includes('private prose'));
    assert.ok(!html.includes('private@example.test'));
    assert.ok(!html.includes('private version'));
  });
});
