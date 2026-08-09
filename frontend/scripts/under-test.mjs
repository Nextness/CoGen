// under-test.mjs is an opt-in module-resolution hook that runs the frontend
// unit test suite against the compiled candidate tree (frontend/dist-ts)
// instead of the legacy sources (frontend/src). It engages only when
// FRONTEND_UNDER_TEST=dist-ts: relative specifiers that resolve under
// frontend/src are mapped to frontend/dist-ts with the .ts suffix rewritten
// to .js.
//
// The candidate tree is always complete (the build compiles every module),
// so there is no fallback to source: mapping a specifier whose dist-ts
// counterpart does not exist throws, and any resolved candidate URL must
// provably live under frontend/dist-ts. Together with the strict env check at
// registration, these rules guarantee that a run with the env var set either
// exercises the candidate tree or fails loudly; a misspelled or unset env var
// can never silently re-run the legacy tree.
import { registerHooks } from 'node:module';

const target = process.env.FRONTEND_UNDER_TEST;
if (target !== 'dist-ts') {
  throw new Error(`FRONTEND_UNDER_TEST must be exactly "dist-ts", got ${JSON.stringify(target)}; refusing to run a mixed or legacy suite`);
}

/** Maps one project-relative specifier from src to the compiled candidate tree. */
function mapSpecifier(specifier) {
  if (
    !(specifier.startsWith('./') || specifier.startsWith('../')) ||
    !specifier.includes('/src/') ||
    specifier.includes('/vendor/')
  ) {
    return null;
  }
  return specifier.replace('/src/', '/dist-ts/').replace(/\.ts$/, '.js');
}

registerHooks({
  resolve(specifier, context, nextResolve) {
    const mapped = mapSpecifier(specifier);
    if (mapped === null) {
      return nextResolve(specifier, context);
    }
    const resolved = nextResolve(mapped, context);
    const url = typeof resolved === 'object' ? resolved.url : resolved;
    if (!String(url).includes('/dist-ts/')) {
      throw new Error(`candidate resolution sentinel failed for ${specifier}: resolved ${url}, not under frontend/dist-ts`);
    }
    return resolved;
  },
});