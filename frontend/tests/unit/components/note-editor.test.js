// Unit tests for browser-local drafts and bounded note comparisons.
import { describe, it } from 'node:test';
import assert from 'node:assert/strict';

import '../setup.js';
import { clearDraft, draftKey, lineDiff, readDraft, writeDraft } from '../../../src/components/note-editor.js';

describe('note-editor.js', function() {
  it('namespaces and clears only an exact saved draft', function() {
    const values = new Map();
    const storage = { getItem: function(key) { return values.get(key) ?? null; }, setItem: function(key, value) { values.set(key, value); }, removeItem: function(key) { values.delete(key); } };
    const first = draftKey('corpus', 1, 2, 'new', null);
    const second = draftKey('corpus', 1, 2, 8, 9);
    assert.equal(writeDraft(first, 'draft one', storage), true);
    assert.equal(writeDraft(second, 'draft two', storage), true);
    clearDraft(first, storage);
    assert.equal(readDraft(first, storage), null);
    assert.equal(readDraft(second, storage), 'draft two');
  });

  it('bounds quadratic comparison and identifies changed lines', function() {
    const changed = lineDiff('one\ntwo', 'one\nthree');
    assert.equal(changed.fallback, false);
    assert.deepEqual(changed.rows.map(function(row) { return row.type; }), ['same', 'added', 'removed']);
    assert.equal(lineDiff(Array(202).fill('x').join('\n'), 'x').fallback, true);
  });
});
