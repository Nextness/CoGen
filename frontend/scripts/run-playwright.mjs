import { access, copyFile, mkdir } from 'node:fs/promises';
import { constants } from 'node:fs';
import { spawn } from 'node:child_process';
import { fileURLToPath } from 'node:url';
import path from 'node:path';

const frontendDir = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const rootDir = path.resolve(frontendDir, '..');
const binary = process.env.ANALYSIS_BINARY || path.join(rootDir, 'build', 'analysis');
const fixtureDB = process.env.FIXTURE_DB || path.join(rootDir, 'src', 'server', 'testdata', 'workspace.fixture.db');
const assetsDir = process.env.ASSETS_DIR || path.join(rootDir, 'frontend', 'dist');
const runDir = path.join(rootDir, 'build', 'playwright', `run-${Date.now()}-${process.pid}`);
const reportDir = path.join(runDir, 'report');
const resultDir = path.join(runDir, 'results');
const timeoutMS = 30_000;

await mustExist(binary, 'analysis binary', 'Run make build first.');
await mustExist(fixtureDB, 'viewer fixture database', 'Run make fixture first.');
await mustExist(assetsDir, 'frontend asset directory', 'Run make frontend-build first.');
await mkdir(runDir, { recursive: true });
const isolatedDB = await copyFixturePair();

let server;
let serverExited = false;

try {
  const baseURL = await startServer();
  await waitForHealth(baseURL);
  console.log(`Playwright server: ${baseURL}`);
  console.log(`Playwright report: ${path.relative(rootDir, reportDir)}`);

  const playwright = spawn(npmCommand(), ['exec', '--', 'playwright', 'test', ...process.argv.slice(2)], {
    cwd: frontendDir,
    env: {
      ...process.env,
      PLAYWRIGHT_BASE_URL: baseURL,
      PLAYWRIGHT_HTML_OUTPUT_DIR: reportDir,
      PLAYWRIGHT_TEST_RESULTS_DIR: resultDir,
    },
    stdio: 'inherit',
  });
  process.exitCode = await exitCode(playwright, 'Playwright');
} catch (error) {
  console.error(error instanceof Error ? error.message : error);
  process.exitCode = 1;
} finally {
  await stopServer();
}

/** Asynchronously implements must exist for the viewer. */
async function mustExist(target, name, hint) {
  try {
    await access(target, constants.R_OK);
  } catch {
    throw new Error(`${name} is unavailable at ${target}. ${hint}`);
  }
}

/** Starts the fixture-backed viewer on an operating-system-assigned loopback port. */
function startServer() {
  return new Promise((resolve, reject) => {
    let output = '';
    let settled = false;
    const timer = setTimeout(() => fail(new Error(`timed out starting isolated viewer server after ${timeoutMS}ms`)), timeoutMS);

    server = spawn(binary, ['serve', '--db', isolatedDB, '--addr', '127.0.0.1:0', '--assets-dir', assetsDir], {
      cwd: rootDir,
      stdio: ['ignore', 'ignore', 'pipe'],
    });
    server.stderr.on('data', (chunk) => {
      const text = chunk.toString();
      process.stderr.write(text);
      output += text;
      const match = output.match(/viewer listening.*\baddr=([^\s]+)/);
      if (match) {
        settled = true;
        clearTimeout(timer);
        resolve(`http://${match[1]}`);
      }
    });
    server.once('error', (error) => fail(new Error(`start isolated viewer server: ${error.message}`)));
    server.once('exit', (code, signal) => {
      serverExited = true;
      if (!settled) {
        fail(new Error(`isolated viewer server exited before becoming ready (code=${code}, signal=${signal})\n${output}`));
      }
    });

    /** Stops startup and rejects with the server process failure. */
    function fail(error) {
      if (settled) return;
      settled = true;
      clearTimeout(timer);
      reject(error);
    }
  });
}

/** Copies the generated fixture pair so browser mutations never alter their authoritative base. */
async function copyFixturePair() {
  const inferredPDF = fixtureDB.endsWith('.metadata.db') ? fixtureDB.replace(/\.metadata\.db$/, '.pdf.db') : fixtureDB.replace(/\.db$/, '.pdf.db');
  const sourcePDF = process.env.FIXTURE_PDF_DB || inferredPDF;
  await mustExist(sourcePDF, 'viewer PDF fixture database', 'Run make fixture first or set FIXTURE_PDF_DB.');
  const explicitMetadata = process.env.PLAYWRIGHT_MUTATION_DB ? path.resolve(process.env.PLAYWRIGHT_MUTATION_DB) : '';
  if (explicitMetadata) {
    const relative = path.relative(rootDir, explicitMetadata);
    if (relative.startsWith('..') || (!relative.startsWith(path.join('build', 'e2e')) && !relative.startsWith(path.join('build', 'playwright')))) {
      throw new Error(`PLAYWRIGHT_MUTATION_DB must be under build/e2e or build/playwright, got ${explicitMetadata}`);
    }
  }
  const destination = explicitMetadata ? path.dirname(explicitMetadata) : path.join(runDir, 'fixture');
  await mkdir(destination, { recursive: true });
  const metadataCopy = explicitMetadata || path.join(destination, path.basename(fixtureDB));
  const pdfCopy = path.join(destination, path.basename(sourcePDF));
  await Promise.all([copyFile(fixtureDB, metadataCopy, constants.COPYFILE_EXCL), copyFile(sourcePDF, pdfCopy, constants.COPYFILE_EXCL)]);
  return metadataCopy;
}

/** Asynchronously implements wait for health for the viewer. */
async function waitForHealth(baseURL) {
  const deadline = Date.now() + timeoutMS;
  let lastError;
  while (Date.now() < deadline) {
    try {
      const response = await fetch(`${baseURL}/api/health`);
      if (response.ok) return;
      lastError = new Error(`health endpoint returned ${response.status}`);
    } catch (error) {
      lastError = error;
    }
    await delay(100);
  }
  throw new Error(`isolated viewer server did not become healthy: ${lastError instanceof Error ? lastError.message : lastError}`);
}

/** Asynchronously implements stop server for the viewer. */
async function stopServer() {
  if (!server || serverExited) return;
  server.kill('SIGTERM');
  await Promise.race([
    new Promise((resolve) => server.once('exit', resolve)),
    delay(5_000).then(() => {
      if (!serverExited) server.kill('SIGKILL');
    }),
  ]);
}

/** Normalizes a child-process exit result. */
function exitCode(child, name) {
  return new Promise((resolve, reject) => {
    child.once('error', (error) => reject(new Error(`start ${name}: ${error.message}`)));
    child.once('exit', (code, signal) => resolve(code ?? (signal ? 1 : 0)));
  });
}

/** Returns a promise that resolves after the requested interval. */
function delay(milliseconds) {
  return new Promise((resolve) => setTimeout(resolve, milliseconds));
}

/** Returns the platform-appropriate npm command. */
function npmCommand() {
  return process.platform === 'win32' ? 'npm.cmd' : 'npm';
}
