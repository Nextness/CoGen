// build.mjs assembles the served frontend root into frontend/dist from
// frontend/index.html, styles/, vendor/, and the TypeScript sources under
// src/. Every file under src/ is compiled per-file through esbuild, and .ts
// import specifiers in the emitted output are rewritten to .js because
// per-file emit preserves them verbatim. The assembled root contains no
// .ts/.d.ts/.map files and no dot- or underscore-prefixed entries.
import { cp, mkdir, readFile, readdir, rm, writeFile } from 'node:fs/promises';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const frontendDir = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const outDir = path.join(frontendDir, 'dist');
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

/** Lists every source file under src/ (excluding hidden entries). */
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
      } else if (entry.isFile() && !entry.name.endsWith('.d.ts')) {
        files.push(full);
      }
    }
  };
  await walk(srcDir);
  return files;
}

/** Compiles src/** with esbuild per-file and rewrites .ts/.tsx specifiers to .js. */
async function compileSources() {
  const { build } = await import('esbuild');
  const files = await listSources();
  for (const file of files) {
    const relative = path.relative(srcDir, file);
    const outFile = path.join(outDir, relative.replace(/\.(ts|tsx)$/, '.js'));
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
    const rewritten = emitted.replace(/\.tsx?(["'])/g, '.js$1');
    await writeFile(outFile, rewritten);
  }
  for (const file of files) {
    const emitted = await readFile(path.join(outDir, path.relative(srcDir, file).replace(/\.(ts|tsx)$/, '.js')), 'utf8');
    if (/["']\.\.?\/[^"']+\.tsx?["']/.test(emitted)) {
      throw new Error(`stale .ts/.tsx import specifier remains in ${path.relative(outDir, file)}`);
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
      } else if (entry.isFile() && /\.(d\.ts|tsx?|map)$/.test(entry.name)) {
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

await compileSources();
await copyTree(path.join(frontendDir, 'index.html'), path.join(outDir, 'index.html'));
await copyTree(path.join(frontendDir, 'styles'), path.join(outDir, 'styles'));
await copyTree(path.join(frontendDir, 'vendor'), path.join(outDir, 'vendor'));
await assertClean(outDir);
console.log(`assembled frontend assets in ${path.relative(frontendDir, outDir)}`);