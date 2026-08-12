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
const timeoutMS = 30_000;
const mutationSpecs = ['review.spec.cjs', 'e2e.spec.cjs'];

await mustExist(binary, 'analysis binary', 'Run make build first.');
await mustExist(fixtureDB, 'viewer fixture database', 'Run make fixture first.');
await mustExist(assetsDir, 'frontend asset directory', 'Run make frontend-build first.');
await mkdir(runDir, { recursive: true });

const requestedArguments = process.argv.slice(2);
const selectedSuites = suitesForArguments(requestedArguments);
let result = 0;
try {
  if (selectedSuites.includes('read')) {
    result = Math.max(result, await runSuite('read', requestedArguments));
  }
  if (selectedSuites.includes('mutation')) {
    const mutationArguments = withoutOptions(requestedArguments, ['workers']);
    mutationArguments.push('--workers=1');
    result = Math.max(result, await runSuite('mutation', mutationArguments));
  }
  process.exitCode = result;
} catch (error) {
  console.error(error instanceof Error ? error.message : error);
  process.exitCode = 1;
}

/** Runs one Playwright suite against its own fixture copy and viewer process. */
async function runSuite(suite, args) {
  const reportDir = path.join(runDir, 'report', suite);
  const resultDir = path.join(runDir, 'results', suite);
  const isolatedDB = await copyFixturePair(path.join(runDir, 'fixture', suite));
  const server = await startServer(isolatedDB);
  try {
    const baseURL = server.baseURL;
    await waitForHealth(baseURL);
    console.log(`Playwright ${suite} server: ${baseURL}`);
    console.log(`Playwright ${suite} report: ${path.relative(rootDir, reportDir)}`);
    const playwright = spawn(npmCommand(), ['exec', '--', 'playwright', 'test', ...args], {
      cwd: frontendDir,
      env: {
        ...process.env,
        PLAYWRIGHT_BASE_URL: baseURL,
        PLAYWRIGHT_HTML_OUTPUT_DIR: reportDir,
        PLAYWRIGHT_TEST_RESULTS_DIR: resultDir,
        PLAYWRIGHT_SUITE: suite,
      },
      stdio: 'inherit',
    });
    return await exitCode(playwright, 'Playwright');
  } finally {
    await stopServer(server);
  }
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
function startServer(db) {
  return new Promise((resolve, reject) => {
    let output = '';
    let settled = false;
    const timer = setTimeout(() => fail(new Error(`timed out starting isolated viewer server after ${timeoutMS}ms`)), timeoutMS);

    const child = spawn(binary, ['serve', '--db', db, '--addr', '127.0.0.1:0', '--assets-dir', assetsDir], {
      cwd: rootDir,
      stdio: ['ignore', 'ignore', 'pipe'],
    });
    const server = { child: child, exited: false, baseURL: '' };
    child.stderr.on('data', (chunk) => {
      const text = chunk.toString();
      process.stderr.write(text);
      output += text;
      const match = output.match(/viewer listening.*\baddr=([^\s]+)/);
      if (match) {
        settled = true;
        clearTimeout(timer);
        server.baseURL = `http://${match[1]}`;
        resolve(server);
      }
    });
    child.once('error', (error) => fail(new Error(`start isolated viewer server: ${error.message}`)));
    child.once('exit', (code, signal) => {
      server.exited = true;
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
async function copyFixturePair(destination) {
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
  const copyDestination = explicitMetadata ? path.dirname(explicitMetadata) : destination;
  await mkdir(copyDestination, { recursive: true });
  const metadataCopy = explicitMetadata || path.join(copyDestination, path.basename(fixtureDB));
  const pdfCopy = path.join(copyDestination, path.basename(sourcePDF));
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
async function stopServer(server) {
  if (!server || server.exited) return;
  server.child.kill('SIGTERM');
  await Promise.race([
    new Promise((resolve) => server.child.once('exit', resolve)),
    delay(5_000).then(() => {
      if (!server.exited) server.child.kill('SIGKILL');
    }),
  ]);
}

/** Selects read-only, mutation, or both suites from explicit test-file arguments. */
function suitesForArguments(args) {
  const selectors = args.filter((argument) => argument.includes('.spec.cjs'));
  if (selectors.length === 0) return ['read', 'mutation'];
  const hasMutation = selectors.some((argument) => mutationSpecs.some((spec) => argument.includes(spec)));
  const hasRead = selectors.some((argument) => !mutationSpecs.some((spec) => argument.includes(spec)));
  if (hasMutation && !hasRead) return ['mutation'];
  if (hasRead && !hasMutation) return ['read'];
  return ['read', 'mutation'];
}

/** Removes named CLI options in both --name=value and --name value forms. */
function withoutOptions(args, names) {
  const result = [];
  for (let index = 0; index < args.length; index += 1) {
    const argument = args[index];
    const name = names.find((candidate) => argument === `--${candidate}` || argument.startsWith(`--${candidate}=`));
    if (!name) {
      result.push(argument);
    } else if (argument === `--${name}`) {
      index += 1;
    }
  }
  return result;
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
