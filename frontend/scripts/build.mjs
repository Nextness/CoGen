// build.mjs assembles the served frontend root into frontend/dist (and, in
// TypeScript mode, frontend/dist-ts) from frontend/index.html, styles/,
// vendor/, and src/. Phase 1 copies src/ verbatim; TypeScript mode compiles
// every file under src/ through esbuild per-file and rewrites .ts import
// specifiers to .js in the emitted output.
import { cp, mkdir, readFile, readdir, rm, writeFile } from 'node:fs/promises';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const frontendDir = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const tsMode = process.argv.includes('--ts');
const outDir = tsMode ? path.join(frontendDir, 'dist-ts') : path.join(frontendDir, 'dist');
const srcDir = path.join(frontendDir, 'src');

/** Reports whether a file name must not be served (dotfiles and underscore-prefixed files). */
function isHidden(name) {
  return name.startsWith('.') || name.startsWith('_');
}

/** Copies one path into the output root, skipping hidden entries. */
async function copyTree(from, to) {
  await cp(from, to, {
    recursive: true,
    filter: (source) => !isHidden(path.basename(source)),
  });
}

/** Lists every file under src/ (excluding hidden entries). */
async function listSources() {
  const files = [];
  const walk = async (dir) => {
    for (const entry of await readdir(dir, { withFileTypes: true })) {
      if (isHidden(entry.name)) {
        continue;
      }
      const full = path.join(dir, entry.name);
      if (entry.isDirectory()) {
        await walk(full);
      } else if (entry.isFile()) {
        files.push(full);
      }
    }
  };
  await walk(srcDir);
  return files;
}

/** Compiles src/** with esbuild per-file and rewrites .ts specifiers to .js. */
async function compileSources() {
  const { build } = await import('esbuild');
  const files = await listSources();
  for (const file of files) {
    const relative = path.relative(srcDir, file);
    const outFile = path.join(outDir, relative.replace(/\.ts$/, '.js'));
    await mkdir(path.dirname(outFile), { recursive: true });
    await build({
      entryPoints: [file],
      outfile: outFile,
      format: 'esm',
      target: 'es2022',
      bundle: false,
      write: true,
    });
    const emitted = await readFile(outFile, 'utf8');
    const rewritten = emitted.replace(/\.ts(["'])/g, '.js$1');
    await writeFile(outFile, rewritten);
  }
  for (const file of files) {
    const emitted = await readFile(path.join(outDir, path.relative(srcDir, file).replace(/\.ts$/, '.js')), 'utf8');
    if (/["']\.\.?\/[^"']+\.ts["']/.test(emitted)) {
      throw new Error(`stale .ts import specifier remains in ${path.relative(outDir, file)}`);
    }
  }
}

/** Verifies the assembled root contains no TypeScript, declaration, or map files. */
async function assertClean(root) {
  const found = [];
  const walk = async (dir) => {
    for (const entry of await readdir(dir, { withFileTypes: true })) {
      const full = path.join(dir, entry.name);
      if (entry.isDirectory()) {
        await walk(full);
      } else if (entry.isFile() && /\.(d\.ts|ts|map)$/.test(entry.name)) {
        found.push(full);
      }
    }
  };
  await walk(root);
  if (found.length > 0) {
    throw new Error(`assembled root contains non-servable files:\n${found.join('\n')}`);
  }
}

await rm(outDir, { recursive: true, force: true });
await mkdir(outDir, { recursive: true });

if (tsMode) {
  await compileSources();
} else {
  await copyTree(srcDir, outDir);
}
await copyTree(path.join(frontendDir, 'index.html'), path.join(outDir, 'index.html'));
await copyTree(path.join(frontendDir, 'styles'), path.join(outDir, 'styles'));
await copyTree(path.join(frontendDir, 'vendor'), path.join(outDir, 'vendor'));
await assertClean(outDir);
console.log(`assembled frontend assets in ${path.relative(frontendDir, outDir)}${tsMode ? ' (ts)' : ''}`);