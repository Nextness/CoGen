// Bounded note parsing and safe preview rendering for the review editor.
import { link, stateFor } from "../state.tsx";
import { h, Fragment, cx } from "../jsx/jsx-runtime.ts";
import type { ResolvedNoteLink } from "../api/types.ts";
import type { ClassNames } from "../jsx/jsx-runtime.ts";
import type { ClassName } from "../jsx/classes.ts";

/** Typed compound class names used by this module. */
const classNames = {
  uiTableRwNoteTable: cx("ui", "table", "rw-note-table"),
};

/** Defined heading modifiers indexed by the supported note heading level. */
const noteHeadingModifiers: Record<number, ClassName> = {
  1: "rw-note-heading--1",
  2: "rw-note-heading--2",
  3: "rw-note-heading--3",
  4: "rw-note-heading--4",
};

export const noteBodyLimit = 262144;
const supportedSchemes = new Set(["note", "article", "pdf", "anchor", "ext"]);
const anchorPattern = /^[A-Za-z][A-Za-z0-9._-]{0,63}$/;

/** Returns the bounded class combination for one note heading level. */
function noteHeadingClass(level: number): ClassNames {
  const modifier = noteHeadingModifiers[level] || "rw-note-heading--4";
  return cx("rw-note-heading", modifier);
}

/** One parsed note block in the bounded block taxonomy. */
export interface NoteBlock {
  type: string;
  text?: string;
  level?: number;
  ordered?: boolean;
  items?: string[];
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
export function parseNote(body: unknown): { blocks: NoteBlock[]; links: NoteLink[]; errors: NoteDiagnostic[] } {
  const source = String(body || "");
  const errors: NoteDiagnostic[] = [];
  if (new TextEncoder().encode(source).length > noteBodyLimit) {
    errors.push({
      position: 0,
      length: source.length,
      message: `note body exceeds ${noteBodyLimit} bytes`,
    });
    return { blocks: [], links: [], errors: errors };
  }

  const rawLines: string[] = source.split("\n");
  const lines: Array<{ text: string; start: number }> = [];
  let sourceOffset = 0;
  rawLines.forEach((rawLine) => {
    var text = rawLine;
    if (text.endsWith("\r")) text = text.slice(0, -1);
    lines.push({ text: text, start: sourceOffset });
    sourceOffset += rawLine.length + 1;
  });

  const blocks: NoteBlock[] = [];
  const links: NoteLink[] = [];

  let index = 0;
  while (index < lines.length) {
    const current = lines[index];
    if (current.text === "") {
      index += 1;
      continue;
    }

    if (current.text === "```") {
      const fenceStart = current.start;
      const contents: string[] = [];
      index += 1;
      while (index < lines.length && lines[index].text !== "```") {
        contents.push(lines[index].text);
        index += 1;
      }
      if (index === lines.length) {
        errors.push({ position: fenceStart, length: 3, message: "unclosed code fence" });
      } else {
        index += 1;
      }
      blocks.push({ type: "code", text: contents.join("\n") });
      continue;
    }

    const heading = /^(#{1,4}) (.*)$/.exec(current.text);
    if (heading) {
      blocks.push({ type: "heading", level: heading[1].length, text: heading[2] });
      index += 1;
      continue;
    }

    if (current.text.startsWith("> ")) {
      const quoteLines: string[] = [];
      while (index < lines.length && lines[index].text.startsWith("> ")) {
        quoteLines.push(lines[index].text.slice(2));
        index += 1;
      }
      blocks.push({ type: "quote", text: quoteLines.join("\n") });
      continue;
    }

    const listItem = parseListItem(current.text);
    if (listItem) {
      const items: string[] = [listItem.text];
      index += 1;
      while (index < lines.length) {
        const next = parseListItem(lines[index].text);
        if (!next || next.ordered !== listItem.ordered) break;
        items.push(next.text);
        index += 1;
      }
      const block: NoteBlock = { type: "list", items: items };
      if (listItem.ordered) block.ordered = true;
      blocks.push(block);
      continue;
    }

    if (hasUnescapedPipe(current.text) && index + 1 < lines.length && hasUnescapedPipe(lines[index + 1].text)) {
      const header = splitTableRow(current.text);
      const delimiter = splitTableRow(lines[index + 1].text);
      const delimiterValid = header.length >= 2 && delimiter.length === header.length && delimiter.every((cell) => /^-{3,}$/.test(cell.trim()));
      const firstRowValid = index + 2 < lines.length && lines[index + 2].text !== "" && splitTableRow(lines[index + 2].text).length === header.length;
      if (!delimiterValid || !firstRowValid) {
        const paragraphLines: string[] = [];
        errors.push({ position: current.start, length: current.text.length, message: "malformed table" });
        while (index < lines.length && lines[index].text !== "" && hasUnescapedPipe(lines[index].text)) {
          paragraphLines.push(lines[index].text);
          index += 1;
        }
        blocks.push({ type: "paragraph", text: paragraphLines.join("\n") });
      } else {
        const rows: string[][] = [];
        index += 2;
        while (index < lines.length && lines[index].text !== "" && hasUnescapedPipe(lines[index].text)) {
          const row = splitTableRow(lines[index].text);
          if (row.length !== header.length) {
            errors.push({
              position: lines[index].start,
              length: lines[index].text.length,
              message: "table row has the wrong number of cells",
            });
          } else {
            rows.push(row);
          }
          index += 1;
        }
        blocks.push({ type: "table", header: header, rows: rows });
      }
      continue;
    }

    const paragraphLines: string[] = [current.text];
    index += 1;
    while (index < lines.length && lines[index].text !== "" && lines[index].text !== "```") {
      if (/^#{1,4} /.test(lines[index].text)) break;
      if (lines[index].text.startsWith("> ")) break;
      if (parseListItem(lines[index].text)) break;
      paragraphLines.push(lines[index].text);
      index += 1;
    }
    blocks.push({ type: "paragraph", text: paragraphLines.join("\n") });
  }

  var inFence = false;
  lines.forEach((line) => {
    if (line.text === "```") {
      inFence = !inFence;
    } else if (!inFence) {
      extractLinks(line.text, line.start, links, errors);
    }
  });

  return { blocks: blocks, links: links, errors: errors };
}

/** Parses one supported list marker and its plain item text. */
function parseListItem(line: string): { ordered: boolean; text: string } | null {
  if (line.startsWith("- ")) return { ordered: false, text: line.slice(2) };
  if (line.startsWith("1. ")) return { ordered: true, text: line.slice(3) };
  return null;
}

/** Extracts syntactically valid custom links and positional diagnostics from plain text. */
function extractLinks(text: string, baseOffset: number, links: NoteLink[], errors: NoteDiagnostic[]): void {
  for (let index = 0; index < text.length; index += 1) {
    if (text[index] !== "[" || text[index + 1] !== "[" || isEscaped(text, index)) continue;
    const start = index;
    let end = -1;
    for (index += 2; index < text.length - 1; index += 1) {
      if (text[index] === "]" && text[index + 1] === "]" && !isEscaped(text, index)) {
        end = index;
        break;
      }
    }
    if (end < 0) {
      errors.push({
        position: baseOffset + start,
        length: text.length - start,
        message: "unclosed custom link",
      });
      return;
    }
    const decoded = decodeLink(text.slice(start + 2, end));
    if (decoded.message || !decoded.link) {
      errors.push({
        position: baseOffset + start,
        length: end + 2 - start,
        message: decoded.message,
      });
    } else {
      links.push({
        ordinal: links.length + 1,
        target_type: decoded.link.target_type,
        raw_target: decoded.link.raw_target,
        display_text: decoded.link.display_text,
        position: baseOffset + start,
        length: end + 2 - start,
      });
    }
    index = end + 1;
  }
}

/** Decodes one custom-link payload into its canonical persisted identity. */
function decodeLink(input: string): { link: Pick<NoteLink, "target_type" | "raw_target" | "display_text"> | null; message: string } {
  const split = splitEscaped(input);
  if (split.message) return { link: null, message: split.message };
  if (split.parts.length > 2) return { link: null, message: "custom link contains more than one display separator" };

  const separator = split.parts[0].indexOf(":");
  if (separator < 0 || !split.parts[0].slice(separator + 1)) return { link: null, message: "custom link target is empty" };

  const scheme = split.parts[0].slice(0, separator);
  var target = split.parts[0].slice(separator + 1);
  if (new TextEncoder().encode(target).length > 2048) return { link: null, message: "custom link target exceeds 2048 bytes" };

  var displayText: string | null = null;
  if (split.parts.length === 2) {
    displayText = split.parts[1];
    if (new TextEncoder().encode(displayText).length > 1024) return { link: null, message: "custom link display text exceeds 1024 bytes" };
  }

  if (!supportedSchemes.has(scheme)) return { link: null, message: `unknown custom link scheme ${JSON.stringify(scheme)}` };
  if (scheme === "note") {
    if (!/^[1-9]\d*$/.test(target)) return { link: null, message: "note target must be a positive integer" };
    return { link: { target_type: "note", raw_target: target, display_text: displayText }, message: "" };
  }
  if (scheme === "article") {
    target = normalizeDOI(target);
    if (target.length > 60 || !/^10\.\d{4,}\/\S+$/.test(target)) return { link: null, message: "article target must be a DOI" };
    return { link: { target_type: "article", raw_target: target, display_text: displayText }, message: "" };
  }
  if (scheme === "pdf") {
    const pageMatch = /^page=([1-9]\d*)$/.exec(target);
    if (!pageMatch) return { link: null, message: "PDF target must use page=<positive integer>" };
    return { link: { target_type: "pdf_page", raw_target: String(Number(pageMatch[1])), display_text: displayText }, message: "" };
  }
  if (scheme === "anchor") {
    if (!anchorPattern.test(target)) return { link: null, message: "anchor target has an invalid identifier" };
    return { link: { target_type: "anchor", raw_target: target, display_text: displayText }, message: "" };
  }

  try {
    const parsed = new URL(target);
    if ((parsed.protocol !== "http:" && parsed.protocol !== "https:") || !parsed.host) throw new Error("unsupported protocol");
  } catch {
    return { link: null, message: "external URL must use absolute http or https" };
  }
  return { link: { target_type: "ext", raw_target: target, display_text: displayText }, message: "" };
}

/** Reports whether the character at an index has an odd backslash prefix. */
function isEscaped(text: string, index: number): boolean {
  let slashes = 0;
  for (let cursor = index - 1; cursor >= 0 && text[cursor] === "\\"; cursor -= 1) slashes += 1;
  return slashes % 2 === 1;
}

/** Returns the first unescaped delimiter index or minus one. */
function splitEscaped(input: string): { parts: string[]; message: string } {
  const parts: string[] = [""];
  var escaped = false;
  for (const character of input) {
    if (escaped) {
      if (character !== "]" && character !== "|" && character !== "\\") return { parts: [], message: "malformed custom link escape" };
      parts[parts.length - 1] += character;
      escaped = false;
      continue;
    }
    if (character === "\\") {
      escaped = true;
      continue;
    }
    if (character === "|") {
      parts.push("");
      continue;
    }
    parts[parts.length - 1] += character;
  }
  if (escaped) return { parts: [], message: "malformed custom link escape" };
  return { parts: parts, message: "" };
}

/** Splits one simple table row while preserving escaped vertical bars. */
function splitTableRow(line: string): string[] {
  const trimmedLine = line.trim();
  var trimmed = trimmedLine;
  if (trimmed.startsWith("|")) trimmed = trimmed.slice(1);
  if (trimmed.endsWith("|")) trimmed = trimmed.slice(0, -1);
  const cells: string[] = [];
  let cell = "";
  var escaped = false;
  for (let index = 0; index < trimmed.length; index += 1) {
    const character = trimmed[index];
    if (escaped) {
      if (character === "|" || character === "\\") {
        cell += character;
      } else {
        cell += `\\${character}`;
      }
      escaped = false;
      continue;
    }
    if (character === "\\") {
      escaped = true;
      continue;
    }
    if (character === "|") {
      cells.push(cell.trim());
      cell = "";
    } else {
      cell += character;
    }
  }
  if (escaped) cell += "\\";
  cells.push(cell.trim());
  return cells;
}

/** Reports whether a line contains an unescaped table separator. */
function hasUnescapedPipe(line: string): boolean {
  for (let index = 0; index < line.length; index += 1) {
    if (line[index] === "|" && !isEscaped(line, index)) return true;
  }
  return false;
}

/** Canonicalizes article-link DOI targets without database access. */
function normalizeDOI(value: string): string {
  var normalized = value.trim().toLowerCase();
  const prefixes = ["https://doi.org/", "http://doi.org/", "doi:"];
  for (const prefix of prefixes) {
    if (normalized.startsWith(prefix)) normalized = normalized.slice(prefix.length);
  }
  return normalized.trim();
}

/** One resolved note link used by the preview renderer. */
export type { ResolvedNoteLink } from "../api/types.ts";

/** Renders a parsed note as escaped HTML with context-preserving resolved links. */
export function NoteDocument(props: { document: { blocks: NoteBlock[] }; resolvedLinks?: ResolvedNoteLink[] | null }): JSX.Element {
  const links = new Map((props.resolvedLinks || []).map((item) => {
    return [item.ordinal, item];
  }));
  let ordinal = 0;

  /** Splits escaped text on newlines, inserting br elements between lines. */
  function textWithBreaks(text: string): Array<JSX.Element | string> {
    const lines = text.split("\n");
    const nodes: Array<JSX.Element | string> = [];
    lines.forEach((line, index) => {
      if (index > 0) {
        nodes.push(<br />);
      }
      nodes.push(line);
    });
    return nodes;
  }

  /** Renders escaped inline text and associates parsed links with stored resolutions. */
  function inline(text: string): Array<JSX.Element | string> {
    const parts: Array<JSX.Element | string> = [];
    let cursor = 0;
    const parsed: NoteLink[] = [];
    extractLinks(text, 0, parsed, []);
    for (const item of parsed) {
      parts.push(...textWithBreaks(text.slice(cursor, item.position)));
      ordinal += 1;
      const candidate = links.get(ordinal);
      var resolved: ResolvedNoteLink | undefined;
      if (resolutionMatches(item, candidate)) resolved = candidate;
      const label = item.display_text || item.raw_target;
      parts.push(renderLink(label, item, resolved));
      cursor = item.position + item.length;
    }
    parts.push(...textWithBreaks(text.slice(cursor)));
    return parts;
  }

  const blockElements = props.document.blocks.map((block) => {
    if (block.type === "heading") {
      const headingClass = noteHeadingClass(Number(block.level));
      const headingText = inline(block.text as string);
      const headingLevel = (block.level as number) + 4;
      return <div className={headingClass} role="heading" aria-level={headingLevel}>{headingText}</div>;
    }
    if (block.type === "code") {
      return <pre><code>{block.text}</code></pre>;
    }
    if (block.type === "quote") {
      const quoteText = inline(block.text as string);
      return <blockquote>{quoteText}</blockquote>;
    }
    if (block.type === "list") {
      const items = (block.items as string[]).map((item) => {
        const itemText = inline(item);
        return <li>{itemText}</li>;
      });
      let listTag: "ol" | "ul" = "ul";
      if (block.ordered) listTag = "ol";
      return h(listTag, null, items);
    }
    if (block.type === "table") {
      const headerCells = (block.header as string[]).map((cell) => {
        const cellText = inline(cell);
        return <th>{cellText}</th>;
      });
      const bodyRows = (block.rows as string[][]).map((row) => {
        const rowCells = row.map((cell) => {
          const cellText = inline(cell);
          return <td>{cellText}</td>;
        });
        return <tr>{rowCells}</tr>;
      });
      return (
        <div className="table-wrap">
          <table className={classNames.uiTableRwNoteTable}>
            <thead><tr>{headerCells}</tr></thead>
            <tbody>{bodyRows}</tbody>
          </table>
        </div>
      );
    }
    const paragraphText = inline(block.text as string);
    return <p>{paragraphText}</p>;
  });

  return <>{blockElements}</>;
}

/** Renders one safe resolved link or an accessible unresolved label. */
function renderLink(label: string, source: NoteLink, resolved?: ResolvedNoteLink): JSX.Element {
  if (!resolved?.resolved) {
    const diagnostic = `Unresolved ${source.target_type} target: ${source.raw_target}. Save the note to resolve links against the selected review context.`;
    return (
      <span className="rw-note-link--unresolved" aria-label={diagnostic} title={diagnostic}>
        {label}
        <span aria-hidden="true"> ?</span>
      </span>
    );
  }
  if (resolved.target_type === "ext") {
    return <a href={resolved.url} target="_blank" rel="noopener noreferrer">{label}</a>;
  }
  const updates: Record<string, unknown> = {
    view: "article",
    article_id: resolved.work_revision_id,
    note_id: "",
    anchor_id: "",
    pdf_page: "",
  };
  if (resolved.note_id) updates.note_id = resolved.note_id;
  if (resolved.anchor_id) updates.anchor_id = resolved.anchor_id;
  if (resolved.page) updates.pdf_page = resolved.page;
  return <a href={link(updates)} data-state={JSON.stringify(stateFor(updates))}>{label}</a>;
}

/** Confirms that a persisted ordinal resolution still describes the exact parsed draft link identity. */
export function resolutionMatches(source: NoteLink, resolved?: ResolvedNoteLink): boolean {
  if (!resolved) return false;
  return resolved.target_type === source.target_type
    && resolved.raw_target === source.raw_target
    && (resolved.display_text || null) === (source.display_text || null);
}
