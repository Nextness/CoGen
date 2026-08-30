import { cp, mkdir, readFile, rm } from 'node:fs/promises';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const expectedVersion = '4.2.67';
const frontendDir = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const packageDir = path.join(frontendDir, 'node_modules', 'pdfjs-dist');
const destination = path.resolve(frontendDir, 'vendor', 'pdfjs');
const declaration = JSON.parse(await readFile(path.join(packageDir, 'package.json'), 'utf8'));

if (declaration.version !== expectedVersion) {
  throw new Error(`pdfjs-dist ${expectedVersion} is required, but ${declaration.version} is installed`);
}

await rm(destination, { recursive: true, force: true });
await mkdir(destination, { recursive: true });
await Promise.all([
  cp(path.join(packageDir, 'build', 'pdf.min.mjs'), path.join(destination, 'pdf.min.mjs')),
  cp(path.join(packageDir, 'build', 'pdf.worker.min.mjs'), path.join(destination, 'pdf.worker.min.mjs')),
  cp(path.join(packageDir, 'cmaps'), path.join(destination, 'cmaps'), { recursive: true }),
  cp(path.join(packageDir, 'standard_fonts'), path.join(destination, 'standard_fonts'), { recursive: true }),
  cp(path.join(packageDir, 'LICENSE'), path.join(destination, 'LICENSE')),
]);

const [core, worker] = await Promise.all([
  readFile(path.join(destination, 'pdf.min.mjs'), 'utf8'),
  readFile(path.join(destination, 'pdf.worker.min.mjs'), 'utf8'),
]);
for (const [name, source] of [['core', core], ['worker', worker]]) {
  if (!source.includes(expectedVersion)) {
    throw new Error(`vendored PDF.js ${name} does not declare version ${expectedVersion}`);
  }
}
