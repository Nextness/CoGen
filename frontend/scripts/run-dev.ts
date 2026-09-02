import { copyFile, mkdir } from 'node:fs/promises';
import { spawn } from 'node:child_process';
import path from 'node:path';

import { inferFixturePDF } from './fixture-paths.ts';

const [binary, fixtureDB, assetsDir, address] = process.argv.slice(2);
if (!binary || !fixtureDB || !assetsDir || !address) {
  throw new Error('usage: node run-dev.ts <analysis-binary> <fixture-db> <assets-dir> <loopback-address>');
}
const rootDir = path.resolve(process.cwd(), '..');
const absoluteFixture = path.resolve(rootDir, fixtureDB);
const sourcePDF = inferFixturePDF(absoluteFixture);
const destination = path.resolve(rootDir, 'build', 'dev', `run-${Date.now()}-${process.pid}`);
await mkdir(destination, { recursive: true });
const metadataCopy = path.join(destination, path.basename(absoluteFixture));
await Promise.all([
  copyFile(absoluteFixture, metadataCopy),
  copyFile(sourcePDF, path.join(destination, path.basename(sourcePDF))),
]);
console.log(`Development review data: ${path.relative(rootDir, destination)}`);
const server = spawn(path.resolve(rootDir, binary), ['serve', '--db', metadataCopy, '--addr', address, '--assets-dir', path.resolve(rootDir, assetsDir)], { cwd: rootDir, stdio: 'inherit' });
const signals: NodeJS.Signals[] = ['SIGINT', 'SIGTERM'];
for (const signal of signals) process.on(signal, function() { server.kill(signal); });
server.once('error', function(error) { throw error; });
process.exitCode = await new Promise<number>(function(resolve) { server.once('exit', function(code) { resolve(code || 0); }); });
