// compare-dom.mjs diffs the two DOM capture files produced by running
// frontend/tests/dom-capture.cjs once against frontend/dist (baseline) and
// once against frontend/dist-ts (candidate). Each capture file maps route
// names to verbatim document.body.innerHTML strings; any per-route difference
// is reported as a candidate TypeScript regression and fails the script.
import { readFile } from 'node:fs/promises';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const frontendDir = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const repoRoot = path.resolve(frontendDir, '..');
const args = process.argv.slice(2);

/** Reads a capture JSON file and returns its route map. */
async function readCapture(flag, fallback) {
  const index = args.indexOf(flag);
  const target = index >= 0 ? args[index + 1] : fallback;
  const raw = await readFile(path.resolve(repoRoot, target), 'utf8');
  const parsed = JSON.parse(raw);
  return { tree: parsed.tree, routes: parsed.routes };
}

const baseline = await readCapture('--baseline', 'build/playwright/dom-capture-dist.json');
const candidate = await readCapture('--candidate', 'build/playwright/dom-capture-dist-ts.json');
const routeNames = Array.from(new Set([...Object.keys(baseline.routes), ...Object.keys(candidate.routes)])).sort();

console.log(`comparing ${baseline.tree} (${Object.keys(baseline.routes).length} routes) vs ${candidate.tree} (${Object.keys(candidate.routes).length} routes)`);

const failures = [];
for (const route of routeNames) {
  if (!(route in baseline.routes)) {
    failures.push(`${route}: missing in baseline capture`);
    continue;
  }
  if (!(route in candidate.routes)) {
    failures.push(`${route}: missing in candidate capture`);
    continue;
  }
  const baselineHTML = baseline.routes[route];
  const candidateHTML = candidate.routes[route];
  if (baselineHTML === candidateHTML) {
    console.log(`DOM matches: ${route}`);
    continue;
  }
  const length = Math.min(baselineHTML.length, candidateHTML.length);
  let offset = 0;
  while (offset < length && baselineHTML[offset] === candidateHTML[offset]) offset += 1;
  const snippet = baselineHTML.slice(offset, offset + 120) || '(baseline ended)';
  failures.push(`${route}: DOM differs at offset ${offset}\n  baseline: ${snippet}\n  candidate: ${candidateHTML.slice(offset, offset + 120)}`);
}

if (failures.length) {
  for (const failure of failures) {
    console.error(`- ${failure}`);
  }
  process.exit(1);
}
console.log(`compare-dom clean: ${routeNames.length} routes match`);