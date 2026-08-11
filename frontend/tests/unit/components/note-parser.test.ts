// Unit tests for bounded review-note parsing and safe rendering.
import { describe, it } from 'node:test';
import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';

import '../setup.ts';
import { parseNote, renderNote } from '../../../src/components/note-parser.tsx';

describe('note-parser.js', function() {
  it('parses every custom link form and suppresses links in code fences', function() {
    const document = parseNote('# Review\n\n[[note:12|note]] [[article:10.1000/example]] [[pdf:page=2]] [[anchor:methods-1]] [[ext:https://example.test]]\n\n```\n[[note:99]]\n```');
    assert.equal(document.errors.length, 0);
    assert.deepEqual(document.links.map(function(item) { return item.target_type; }), ['note', 'article', 'pdf_page', 'anchor', 'ext']);
  });

  it('reports unsafe protocols, malformed links, tables, and UTF-16 positions', function() {
    const unsafe = parseNote('😀 [[ext:javascript:alert(1)]]');
    assert.equal(unsafe.errors[0].position, 3);
    assert.match(unsafe.errors[0].message, /protocol/);
    assert.ok(parseNote('[[pdf:page=0]]').errors.length);
    assert.ok(parseNote('| A | B |\n| --- | --- |\n| only one |').errors.length);
    assert.ok(parseNote('```\nunclosed').errors.length);
  });

  it('escapes raw HTML and visibly labels unresolved links', function() {
    const document = parseNote('<img src=x onerror=alert(1)> [[note:3|related]]');
    const html = renderNote(document, [{ ordinal: 1, target_type: 'note', resolved: false }]);
    assert.ok(html.includes('&lt;img'));
    assert.ok(!html.includes('<img'));
    assert.ok(html.includes('aria-label="Unresolved link"'));
  });

  it('scopes note headings and tables beneath the review hierarchy', function() {
    const html = renderNote(parseNote('# Finding\n\n| Field | Value |\n| --- | --- |\n| Status | Valid |'));
    assert.ok(html.includes('class="rw-note-heading rw-note-heading--1"'));
    assert.ok(html.includes('role="heading" aria-level="5"'));
    assert.ok(html.includes('class="ui compact table rw-note-table"'));
    assert.ok(!html.includes('<h1>'));
  });

  it('matches the shared normalized link and diagnostic fixtures', async function() {
    const fixtures = JSON.parse(await readFile(new URL('../../../../src/notes/testdata/conformance.json', import.meta.url), 'utf8'));
    for (const fixture of fixtures) {
      const document = parseNote(fixture.body);
      assert.equal(document.errors.length, fixture.error_count, fixture.name);
      assert.deepEqual(document.links.map(function(item) { return item.target_type; }), fixture.link_types, fixture.name);
      if (fixture.first_error_position !== undefined) assert.equal(document.errors[0].position, fixture.first_error_position, fixture.name);
    }
  });
});
