// Bounded note parsing and safe preview rendering for the review editor.
import { esc, link } from '../state.ts';

export const noteBodyLimit = 262144;
const supportedSchemes = new Set(['note', 'article', 'pdf', 'anchor', 'ext']);
const anchorPattern = /^[A-Za-z][A-Za-z0-9._-]{0,63}$/;

/** One parsed note block in the bounded block taxonomy. */
export interface NoteBlock {
  type: string;
  text?: string;
  offset?: number;
  level?: number;
  ordered?: boolean;
  items?: Array<{ text: string; offset: number }>;
  header?: string[];
  rows?: string[][];
}

/** One extracted note link with its positional and display details. */
export interface NoteLink {
  ordinal: number;
  target_type: string;
  raw_target: string;
  display_text: string | null;
  position: number;
  length: number;
}

/** One positional note diagnostic. */
export interface NoteDiagnostic {
  position: number;
  length: number;
  message: string;
}

/** Parses bounded note text into blocks, extracted links, and UTF-16 diagnostics. */
export function parseNote(body: any): { blocks: NoteBlock[]; links: NoteLink[]; errors: NoteDiagnostic[] } {
  body = String(body || '');
  const errors: NoteDiagnostic[] = [];
  if (new TextEncoder().encode(body).length > noteBodyLimit) {
    errors.push({ position: 0, length: body.length, message: `Note body exceeds ${noteBodyLimit} UTF-8 bytes.` });
  }
  const lines = body.replace(/\r\n?/g, '\n').split('\n');
  const blocks: NoteBlock[] = [];
  const links: NoteLink[] = [];
  let offset = 0;
  let fence: { lines: string[]; offset: number } | null = null;
  let paragraph: string[] = [];
  let paragraphOffset = 0;

  /** Commits accumulated paragraph lines and their custom links to parser output. */
  function flushParagraph(): void {
    if (!paragraph.length) return;
    const text = paragraph.join('\n');
    blocks.push({ type: 'paragraph', text: text, offset: paragraphOffset });
    extractLinks(text, paragraphOffset, links, errors);
    paragraph = [];
  }

  for (let index = 0; index < lines.length; index += 1) {
    const line = lines[index];
    if (fence) {
      if (line === '```') {
        blocks.push({ type: 'code', text: fence.lines.join('\n'), offset: fence.offset });
        fence = null;
      } else {
        fence.lines.push(line);
      }
      offset += line.length + 1;
      continue;
    }
    if (line === '```') {
      flushParagraph();
      fence = { lines: [], offset: offset };
      offset += line.length + 1;
      continue;
    }
    if (!line.trim()) {
      flushParagraph();
      offset += line.length + 1;
      continue;
    }
    const heading = /^(#{1,4}) (.*)$/.exec(line);
    if (heading) {
      flushParagraph();
      blocks.push({ type: 'heading', level: heading[1].length, text: heading[2], offset: offset + heading[1].length + 1 });
      extractLinks(heading[2], offset + heading[1].length + 1, links, errors);
      offset += line.length + 1;
      continue;
    }
    const listItem = /^(?:- |1\. )(.*)$/.exec(line);
    if (listItem) {
      flushParagraph();
      const ordered = line.startsWith('1. ');
      const itemOffset = offset + (ordered ? 3 : 2);
      const previous = blocks.at(-1);
      if (previous?.type === 'list' && previous.ordered === ordered) {
        previous.items!.push({ text: listItem[1], offset: itemOffset });
      } else {
        blocks.push({ type: 'list', ordered: ordered, items: [{ text: listItem[1], offset: itemOffset }] });
      }
      extractLinks(listItem[1], itemOffset, links, errors);
      offset += line.length + 1;
      continue;
    }
    if (line.startsWith('> ')) {
      flushParagraph();
      const text = line.slice(2);
      blocks.push({ type: 'quote', text: text, offset: offset + 2 });
      extractLinks(text, offset + 2, links, errors);
      offset += line.length + 1;
      continue;
    }
    if (line.includes('|') && index + 2 < lines.length && tableDelimiter(lines[index + 1])) {
      flushParagraph();
      const tableLines = [line, lines[index + 1], lines[index + 2]];
      const cells = tableLines.map(splitTableRow);
      if (cells.some(function(row) { return row === null; }) || cells[0]!.length !== cells[1]!.length || cells[0]!.length !== cells[2]!.length) {
        errors.push({ position: offset, length: tableLines.join('\n').length, message: 'Malformed table shape.' });
      } else {
        blocks.push({ type: 'table', header: cells[0]!, rows: [cells[2]!], offset: offset });
        let tableOffset = offset;
        for (const source of [line, lines[index + 2]]) {
          extractLinks(source, tableOffset, links, errors);
          tableOffset += source.length + 1;
        }
      }
      offset += tableLines.reduce(function(total, item) { return total + item.length + 1; }, 0);
      index += 2;
      continue;
    }
    if (!paragraph.length) paragraphOffset = offset;
    paragraph.push(line);
    offset += line.length + 1;
  }
  flushParagraph();
  if (fence) {
    errors.push({ position: fence.offset, length: body.length - fence.offset, message: 'Unclosed code fence.' });
    blocks.push({ type: 'code', text: fence.lines.join('\n'), offset: fence.offset });
  }
  return { blocks: blocks, links: links, errors: errors };
}

/** Extracts syntactically valid custom links and positional diagnostics from plain text. */
function extractLinks(text: string, baseOffset: number, links: NoteLink[], errors: NoteDiagnostic[]): void {
  for (let index = 0; index < text.length; index += 1) {
    if (text[index] !== '[' || text[index + 1] !== '[' || isEscaped(text, index)) continue;
    const start = index;
    let end = -1;
    for (index += 2; index < text.length - 1; index += 1) {
      if (text[index] === ']' && text[index + 1] === ']' && !isEscaped(text, index)) {
        end = index;
        break;
      }
    }
    if (end < 0) {
      errors.push({ position: baseOffset + start, length: text.length - start, message: 'Unclosed note link.' });
      return;
    }
    const raw = text.slice(start + 2, end);
    const separator = unescapedIndex(raw, ':');
    const displaySeparator = unescapedIndex(raw, '|');
    const scheme = separator < 1 ? '' : raw.slice(0, separator);
    const targetEnd = displaySeparator > separator ? displaySeparator : raw.length;
    const target = unescapeLink(raw.slice(separator + 1, targetEnd));
    const display = displaySeparator > separator ? unescapeLink(raw.slice(displaySeparator + 1)) : '';
    const diagnostic = validateLink(scheme, target, raw);
    if (diagnostic) {
      errors.push({ position: baseOffset + start, length: end + 2 - start, message: diagnostic });
    } else {
      links.push({ ordinal: links.length + 1, target_type: scheme === 'pdf' ? 'pdf_page' : scheme, raw_target: scheme === 'pdf' ? target.slice(5) : target, display_text: display || null, position: baseOffset + start, length: end + 2 - start });
    }
    index = end + 1;
  }
}

/** Validates scheme-specific target grammar without requiring target existence. */
function validateLink(scheme: string, target: string, raw: string): string {
  if (!supportedSchemes.has(scheme)) return 'Unknown or missing link scheme.';
  if (!target) return 'Link target must not be empty.';
  if (/\\(?![\]\\|])/.test(raw)) return 'Malformed link escape.';
  if (scheme === 'note' && !/^[1-9]\d*$/.test(target)) return 'Note links require a positive numeric ID.';
  if (scheme === 'pdf' && !/^page=[1-9]\d*$/.test(target)) return 'PDF links require page=<positive number>.';
  if (scheme === 'anchor' && !anchorPattern.test(target)) return 'Anchor link ID has an invalid format.';
  if (scheme === 'ext') {
    const protocol = /^([A-Za-z][A-Za-z0-9+.-]*):/.exec(target);
    if (protocol && protocol[1] !== 'http' && protocol[1] !== 'https') return 'External URL protocol must be HTTP or HTTPS.';
  }
  return '';
}

/** Reports whether the character at an index has an odd backslash prefix. */
function isEscaped(text: string, index: number): boolean {
  let slashes = 0;
  for (let cursor = index - 1; cursor >= 0 && text[cursor] === '\\'; cursor -= 1) slashes += 1;
  return slashes % 2 === 1;
}

/** Returns the first unescaped delimiter index or minus one. */
function unescapedIndex(text: string, character: string): number {
  for (let index = 0; index < text.length; index += 1) {
    if (text[index] === character && !isEscaped(text, index)) return index;
  }
  return -1;
}

/** Removes only the escapes supported inside custom-link fields. */
function unescapeLink(text: string): string {
  return text.replace(/\\([\]\\|])/g, '$1');
}

/** Splits one simple table row while preserving escaped vertical bars. */
function splitTableRow(line: string): string[] | null {
  const trimmed = line.trim().replace(/^\||\|$/g, '');
  const cells = [];
  let cell = '';
  for (let index = 0; index < trimmed.length; index += 1) {
    if (trimmed[index] === '|' && !isEscaped(trimmed, index)) {
      cells.push(unescapeLink(cell.trim()));
      cell = '';
    } else {
      cell += trimmed[index];
    }
  }
  cells.push(unescapeLink(cell.trim()));
  return cells.length >= 2 ? cells : null;
}

/** Reports whether every table cell is a valid delimiter marker. */
function tableDelimiter(line: string): boolean {
  const cells = splitTableRow(line);
  return Boolean(cells && cells.every(function(cell) { return /^-{3,}$/.test(cell); }));
}

/** One resolved note link used by the preview renderer. */
export interface ResolvedNoteLink {
  ordinal: number;
  resolved: boolean;
  target_type?: string;
  url?: string;
  work_revision_id?: any;
  note_id?: any;
  anchor_id?: any;
  page?: any;
}

/** Renders a parsed note as escaped HTML with context-preserving resolved links. */
export function renderNote(document: { blocks: NoteBlock[] }, resolvedLinks?: ResolvedNoteLink[] | null): string {
  const links = new Map((resolvedLinks || []).map(function(item) { return [item.ordinal, item]; }));
  let ordinal = 0;
  /** Renders escaped inline text and associates parsed links with stored resolutions. */
  function inline(text: string): string {
    let output = '';
    let cursor = 0;
    const parsed: NoteLink[] = [];
    extractLinks(text, 0, parsed, []);
    for (const item of parsed) {
      output += esc(text.slice(cursor, item.position));
      ordinal += 1;
      const resolved = links.get(ordinal);
      const label = item.display_text || item.raw_target;
      output += renderLink(label, resolved);
      cursor = item.position + item.length;
    }
    return output + esc(text.slice(cursor)).replace(/\n/g, '<br>');
  }
  return document.blocks.map(function(block) {
    if (block.type === 'heading') return '<div class="rw-note-heading rw-note-heading--' + block.level + '" role="heading" aria-level="' + ((block.level as number) + 4) + '">' + inline(block.text as string) + '</div>';
    if (block.type === 'code') return '<pre><code>' + esc(block.text) + '</code></pre>';
    if (block.type === 'quote') return '<blockquote>' + inline(block.text as string) + '</blockquote>';
    if (block.type === 'list') {
      const tag = block.ordered ? 'ol' : 'ul';
      return `<${tag}>` + (block.items as Array<{ text: string; offset: number }>).map(function(item) { return '<li>' + inline(item.text) + '</li>'; }).join('') + `</${tag}>`;
    }
    if (block.type === 'table') {
      return '<div class="table-wrap"><table class="ui compact table rw-note-table"><thead><tr>' + (block.header as string[]).map(function(cell) { return '<th>' + inline(cell) + '</th>'; }).join('') + '</tr></thead><tbody>' + (block.rows as string[][]).map(function(row) { return '<tr>' + row.map(function(cell) { return '<td>' + inline(cell) + '</td>'; }).join('') + '</tr>'; }).join('') + '</tbody></table></div>';
    }
    return '<p>' + inline(block.text as string) + '</p>';
  }).join('');
}

/** Renders one safe resolved link or an accessible unresolved label. */
function renderLink(label: string, resolved?: ResolvedNoteLink): string {
  if (!resolved?.resolved) return '<span class="rw-note-link rw-note-link--unresolved" aria-label="Unresolved link">' + esc(label) + ' <span aria-hidden="true">?</span></span>';
  if (resolved.target_type === 'ext') return '<a class="rw-note-link" href="' + esc(resolved.url) + '" target="_blank" rel="noopener noreferrer">' + esc(label) + '</a>';
  const updates: Record<string, any> = { view: 'article', article_id: resolved.work_revision_id, note_id: '', anchor_id: '', pdf_page: '' };
  if (resolved.note_id) updates.note_id = resolved.note_id;
  if (resolved.anchor_id) updates.anchor_id = resolved.anchor_id;
  if (resolved.page) updates.pdf_page = resolved.page;
  return '<a class="rw-note-link" href="' + link(updates) + '">' + esc(label) + '</a>';
}