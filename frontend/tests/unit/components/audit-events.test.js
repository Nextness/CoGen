// Unit tests for the shared chronological audit-event presentation.
import { describe, it } from 'node:test';
import assert from 'node:assert/strict';

import '../setup.js';
import { auditCategory, auditEventMarkup, auditStream } from '../../../../src/server/frontend/components/audit-events.js';

describe('audit-events.js', function() {
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
    assert.ok(html.includes('safe_id'));
    assert.ok(!html.includes('private prose'));
    assert.ok(!html.includes('private@example.test'));
    assert.ok(!html.includes('private version'));
  });
});
