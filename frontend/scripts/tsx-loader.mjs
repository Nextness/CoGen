// Self-registering loader hook that lets Node execute .tsx test files and
// .tsx source imports at runtime. Node's native type stripping supports only
// .ts, so .tsx modules are transformed with esbuild before execution. The hook
// is used only by unit tests; the production build compiles .tsx with esbuild
// directly and the type check runs through tsc --noEmit.
import { transformSync } from 'esbuild';
import { registerHooks } from 'node:module';
import { readFileSync } from 'node:fs';

registerHooks({
  resolve(specifier, context, nextResolve) {
    if (specifier.endsWith('.tsx')) {
      return { url: new URL(specifier, context.parentURL).href + '.js', shortCircuit: true };
    }
    return nextResolve(specifier, context);
  },
  load(url, context, nextLoad) {
    if (url.endsWith('.tsx.js')) {
      const fileUrl = url.slice(0, -3);
      const source = readFileSync(new URL(fileUrl));
      const result = transformSync(source.toString(), { loader: 'tsx', format: 'esm', target: 'es2022', jsx: 'transform', jsxFactory: 'h', jsxFragment: 'Fragment' });
      return { format: 'module', source: result.code, shortCircuit: true };
    }
    return nextLoad(url, context);
  },
});