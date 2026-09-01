// Unit tests for bounded review-note parsing and safe rendering.
import { describe, it } from 'node:test';
import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';

import '../setup.ts';
import { parseNote, NoteDocument, resolutionMatches } from '../../../src/components/note-parser.tsx';
import { renderToString } from '../helpers/jsx-render.ts';

const renderNote = (document: any, resolvedLinks?: any[]): string => renderToString(NoteDocument({ document: document, resolvedLinks: resolvedLinks }));

describe('note-parser.tsx', function() {
  it('parses every custom link form and suppresses links in code fences', function() {
    const document = parseNote('# Review\n\n[[note:12|note]] [[article:10.1000/example]] [[pdf:page=2]] [[anchor:methods-1]] [[ext:https://example.test]]\n\n```\n[[note:99]]\n```');
    assert.equal(document.errors.length, 0);
    assert.deepEqual(document.links.map(function(item) { return item.target_type; }), ['note', 'article', 'pdf_page', 'anchor', 'ext']);
  });

  it('reports unsafe protocols, malformed links, tables, and UTF-16 positions', function() {
    const unsafe = parseNote('😀 [[ext:javascript:alert(1)]]');
    assert.equal(unsafe.errors[0].position, 3);
    assert.match(unsafe.errors[0].message, /absolute/);
    assert.ok(parseNote('[[pdf:page=0]]').errors.length);
    assert.ok(parseNote('| A | B |\n| --- | --- |\n| only one |').errors.length);
    assert.ok(parseNote('```\nunclosed').errors.length);
  });

  it('escapes raw HTML and visibly labels unresolved links', function() {
    const document = parseNote('<img src=x onerror=alert(1)> [[note:3|related]]');
    const html = renderNote(document, [{ ordinal: 1, target_type: 'note', raw_target: '3', display_text: 'related', resolved: false }]);
    assert.ok(html.includes('&lt;img'));
    assert.ok(!html.includes('<img'));
    assert.ok(html.includes('aria-label="Unresolved note target: 3.'));
  });

  it('scopes note headings and tables beneath the review hierarchy', function() {
    const html = renderNote(parseNote('# Finding\n\n| Field | Value |\n| --- | --- |\n| Status | Valid |'));
    assert.ok(html.includes('class="rw-note-heading rw-note-heading--1"'));
    assert.ok(html.includes('role="heading" aria-level="5"'));
    assert.ok(html.includes(`class="ui table rw-note-table"`));
    assert.ok(!html.includes('<h1>'));
  });

  it("matches the shared normalized block, link, and diagnostic fixtures", async () => {
    const fixtures = JSON.parse(await readFile(new URL("../../../../src/notes/testdata/conformance.json", import.meta.url), "utf8"));
    for (const fixture of fixtures) {
      const document = parseNote(fixture.body);
      assert.deepEqual(document.blocks, fixture.blocks, `${fixture.name} blocks`);
      assert.deepEqual(document.links, fixture.links, `${fixture.name} links`);
      assert.deepEqual(document.errors, fixture.errors, `${fixture.name} errors`);
    }
  });
});

describe('note resolution identity', function() {
  it('applies persisted link resolutions only to the exact parsed identity', function() {
    const [source] = parseNote('[[article:10.1000/current|Current]]').links;
    const persisted = {
      ordinal: 1,
      target_type: 'article',
      raw_target: '10.1000/old',
      display_text: 'Old',
      resolved: true,
    };
    assert.equal(resolutionMatches(source, persisted), false);
    assert.equal(resolutionMatches(source, { ...persisted, raw_target: source.raw_target, display_text: source.display_text }), true);
  });
});
