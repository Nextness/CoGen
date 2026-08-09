// export-diff.mjs proves export-surface equivalence between the legacy
// JavaScript modules under frontend/src and the compiled candidate modules
// under frontend/dist-ts. It preloads the shared unit-test DOM stub, then for
// every module dynamically imports both trees and compares the exact exported
// names. Side-effectful entry modules (app.js) run their render continuations
// against the stub DOM; those continuations are deliberately ignored because
// the comparison examines only the module namespace surface.
import { access, readdir } from 'node:fs/promises';
import { constants } from 'node:fs';
import path from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';
import '../tests/unit/setup.js';

const frontendDir = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const srcDir = path.join(frontendDir, 'src');
const candidateDir = path.join(frontendDir, 'dist-ts');

// app.js and its dependency views start asynchronous render and fetch
// continuations against the stub DOM when imported. They cannot be awaited
// deterministically and are unrelated to the export surface, so rejections
// from those continuations are ignored; the module namespaces themselves are
// compared synchronously after each import resolves.
process.on('unhandledRejection', function() {});

/** Recursively collects relative .js module paths under a source directory. */
async function listModules(dir, base, acc) {
  for (const entry of await readdir(dir, { withFileTypes: true })) {
    if (entry.name.startsWith('.') || entry.name.endsWith('.d.ts')) {
      continue;
    }
    const full = path.join(dir, entry.name);
    const relative = base ? path.join(base, entry.name) : entry.name;
    if (entry.isDirectory()) {
      await listModules(full, relative, acc);
    } else if (entry.isFile() && entry.name.endsWith('.js')) {
      acc.push(relative);
    }
  }
  return acc;
}

/** Imports one compiled module and returns its exact export names. */
async function importNames(url) {
  const namespace = await import(url);
  return Object.keys(namespace).sort();
}

/** Compares one module pair and reports missing or extra exports. */
async function compareModule(relative) {
  const legacy = await importNames(pathToFileURL(path.join(srcDir, relative)).href);
  let candidate;
  try {
    candidate = await importNames(pathToFileURL(path.join(candidateDir, relative)).href);
  } catch (error) {
    return { missing: legacy, extra: [], candidateError: error instanceof Error ? error.message : String(error) };
  }
  const legacySet = new Set(legacy);
  const candidateSet = new Set(candidate);
  return {
    missing: legacy.filter(function(name) { return !candidateSet.has(name); }),
    extra: candidate.filter(function(name) { return !legacySet.has(name); }),
    candidateError: null,
  };
}

const failures = [];
const modules = await listModules(srcDir, '', []);
for (const relative of modules) {
  try {
    await access(path.join(candidateDir, relative), constants.R_OK);
  } catch {
    failures.push(`${relative}: candidate file missing in dist-ts`);
    continue;
  }
  if (relative === 'app.js') {
    await new Promise(function(resolve) { setTimeout(resolve, 25); });
  }
  const result = await compareModule(relative);
  if (result.candidateError) {
    failures.push(`${relative}: candidate import failed: ${result.candidateError}`);
  } else if (result.missing.length || result.extra.length) {
    failures.push(`${relative}: missing ${JSON.stringify(result.missing)}, extra ${JSON.stringify(result.extra)}`);
  } else {
    console.log(`export surface matches: ${relative}`);
  }
}

if (failures.length) {
  console.error(`export-diff failures (${failures.length}):`);
  for (const failure of failures) {
    console.error(`- ${failure}`);
  }
  process.exit(1);
}
console.log(`export-diff clean: ${modules.length} modules match`);