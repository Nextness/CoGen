import { readFileSync, readdirSync, writeFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

import { classTokens } from "../src/jsx/classes.ts";

const frontendDirectory = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const sourceDirectory = resolve(frontendDirectory, "src");
const knownTokens = new Set<string>(classTokens);
const classNamesDeclaration = /\/\*\* Typed compound class names used by this module\. \*\/\nconst classNames = \{\n(?<body>[\s\S]*?)\n\};\n/;

/** Returns authored TSX files below a directory. */
function tsxFiles(directory: string): string[] {
  const files: string[] = [];
  for (const entry of readdirSync(directory, { withFileTypes: true })) {
    const path = resolve(directory, entry.name);
    if (entry.isDirectory()) {
      files.push(...tsxFiles(path));
    } else if (entry.name.endsWith(".tsx")) {
      files.push(path);
    }
  }
  return files;
}

/** Adds required names to the module's existing JSX runtime import. */
function addRuntimeImports(source: string, required: readonly string[]): string {
  return source.replace(/import \{([^}]*)\} from ((?:"[^"\n]*jsx-runtime\.ts"|'[^'\n]*jsx-runtime\.ts'));/, (statement, imports: string, specifier: string) => {
    const importParts = imports.split(",");
    const names = importParts.map((name) => name.trim());
    for (const requiredName of required) {
      if (!names.includes(requiredName)) names.push(requiredName);
    }
    return `import { ${names.join(", ")} } from ${specifier};`;
  });
}

/** Returns trimmed class tokens from one space-separated class string. */
function tokensIn(value: string): string[] {
  const parts = value.trim().split(/\s+/);
  return parts.filter(Boolean);
}

/** Returns a camelCase property name for one ordered class-token combination. */
function classNamesProperty(tokens: readonly string[]): string {
  const properties = tokens.map((token, index) => {
    const tokenParts = token.split(/[-_]+/);
    const words = tokenParts.filter(Boolean);
    const wordParts = words.map((word, wordIndex) => {
      const firstLetter = word.slice(0, 1);
      const remainingLetters = word.slice(1);
      if (index === 0 && wordIndex === 0) return `${firstLetter.toLowerCase()}${remainingLetters}`;
      return `${firstLetter.toUpperCase()}${remainingLetters}`;
    });
    const part = wordParts.join("");
    if (index === 0) return part;
    return part.slice(0, 1).toUpperCase() + part.slice(1);
  });
  return properties.join("");
}

/** Reads the class combinations already maintained in a module-local declaration. */
function existingClassNames(source: string): Map<string, readonly string[]> {
  const entries = new Map<string, readonly string[]>();
  const declaration = source.match(classNamesDeclaration);
  if (!declaration?.groups) return entries;

  const entryPattern = /^  ([A-Za-z][A-Za-z0-9]*): cx\(((?:"[^"]*"\s*,?\s*)+)\),$/gm;
  const matches = declaration.groups.body.matchAll(entryPattern);
  for (const match of matches) {
    const tokens = Array.from(match[2].matchAll(/"([^"]*)"/g), (tokenMatch) => tokenMatch[1]);
    entries.set(match[1], tokens);
  }
  return entries;
}

/** Inserts or replaces the documented module-local class-combination declaration. */
function withClassNamesDeclaration(source: string, entries: ReadonlyMap<string, readonly string[]>): string {
  if (entries.size === 0) return source;

  const properties = Array.from(entries);
  properties.sort(([left], [right]) => left.localeCompare(right));
  const propertyLines = properties.map(([name, tokens]) => {
    const argumentValues = tokens.map((token) => JSON.stringify(token));
    const argumentsList = argumentValues.join(", ");
    return `  ${name}: cx(${argumentsList}),`;
  });
  const body = propertyLines.join("\n");
  const declaration = `/** Typed compound class names used by this module. */\nconst classNames = {\n${body}\n};\n`;
  if (classNamesDeclaration.test(source)) return source.replace(classNamesDeclaration, declaration);

  const imports = Array.from(source.matchAll(/^import\b[\s\S]*?;$/gm));
  const lastImport = imports.at(-1);
  if (!lastImport || lastImport.index === undefined) {
    throw new Error("could not locate the final import for the class-name declaration");
  }
  const insertion = lastImport.index + lastImport[0].length;
  return `${source.slice(0, insertion)}\n\n${declaration}${source.slice(insertion + 1)}`;
}

/** Rewrites defined compound JSX classes to named, typed module-local values. */
export function rewriteClassNames(source: string, file: string): { source: string; replacements: number; unknown: string[] } {
  let replacements = 0;
  const unknown = new Set<string>();
  const combinations = existingClassNames(source);

  /** Registers one combination and returns its module-local property reference. */
  function reference(tokens: readonly string[]): string {
    const name = classNamesProperty(tokens);
    const existing = combinations.get(name);
    if (existing && existing.join(" ") !== tokens.join(" ")) {
      throw new Error(`${file}: class-name property collision for ${name}`);
    }
    combinations.set(name, tokens);
    return `classNames.${name}`;
  }

  let rewritten = source.replace(/className="([^"]*)"/g, (attribute, value: string) => {
    const tokens = tokensIn(value);
    const missing = tokens.filter((token) => !knownTokens.has(token));
    if (missing.length > 0) {
      for (const token of missing) {
        unknown.add(token);
      }
      return attribute;
    }
    if (tokens.length <= 1) {
      return attribute;
    }
    replacements += 1;
    return `className={${reference(tokens)}}`;
  });
  rewritten = rewritten.replace(/className=\{cx\(((?:"[^"]*"\s*,?\s*)+)\)\}/g, (attribute, argumentsList: string) => {
    const tokens = Array.from(argumentsList.matchAll(/"([^"]*)"/g), (match) => match[1]);
    replacements += 1;
    if (tokens.length === 1) return `className=${JSON.stringify(tokens[0])}`;
    return `className={${reference(tokens)}}`;
  });
  rewritten = rewritten.replace(/className:\s*cx\(((?:"[^"]*"\s*,?\s*)+)\)/g, (property, argumentsList: string) => {
    const tokens = Array.from(argumentsList.matchAll(/"([^"]*)"/g), (match) => match[1]);
    replacements += 1;
    if (tokens.length === 1) return `className: ${JSON.stringify(tokens[0])}`;
    return `className: ${reference(tokens)}`;
  });
  rewritten = rewritten.replace(/\.className\s*=\s*"([^"]*)"/g, (assignment, value: string) => {
    const tokens = tokensIn(value);
    const missing = tokens.filter((token) => !knownTokens.has(token));
    if (missing.length > 0) {
      for (const token of missing) unknown.add(token);
      return assignment;
    }
    if (tokens.length <= 1) return assignment;
    replacements += 1;
    return `.className = ${reference(tokens)}`;
  });
  rewritten = rewritten.replace(/\.className\s*=\s*cx\(((?:"[^"]*"\s*,?\s*)+)\)/g, (assignment, argumentsList: string) => {
    const tokens = Array.from(argumentsList.matchAll(/"([^"]*)"/g), (match) => match[1]);
    replacements += 1;
    if (tokens.length === 1) return `.className = ${JSON.stringify(tokens[0])}`;
    return `.className = ${reference(tokens)}`;
  });
  if (replacements > 0) {
    const withImport = addRuntimeImports(rewritten, ["cx"]);
    if (withImport === rewritten && !/import \{[^}]*\bcx\b[^}]*\} from (?:"[^"]*jsx-runtime\.ts"|'[^']*jsx-runtime\.ts');/.test(rewritten)) {
      throw new Error(`${file}: could not locate the JSX runtime import needed for cx`);
    }
    rewritten = withClassNamesDeclaration(withImport, combinations);
  }
  const unknownTokens = Array.from(unknown);
  unknownTokens.sort();
  return {
    source: rewritten,
    replacements: replacements,
    unknown: unknownTokens,
  };
}

/** Rewrites direct DOM class mutations to typed runtime helpers. */
export function rewriteDOMClassUses(source: string, file: string): { source: string; replacements: number; unknown: string[] } {
  let replacements = 0;
  const unknown = new Set<string>();
  const required = new Set<string>();
  const element = String.raw`([A-Za-z_$][\w$]*(?:(?:\.[A-Za-z_$][\w$]*)|!)*)`;

  /** Returns whether a DOM helper token belongs to the generated registry. */
  function registered(token: string): boolean {
    if (knownTokens.has(token)) return true;
    unknown.add(token);
    return false;
  }

  let rewritten = source.replace(new RegExp(`${element}\\.classList\\.toggle\\("([^"]+)", ([^)\\n]+)\\)`, "g"), (call, target: string, token: string, force: string) => {
    if (!registered(token)) return call;
    replacements += 1;
    required.add("classToggle");
    return `classToggle(${target}, ${JSON.stringify(token)}, ${force})`;
  });
  rewritten = rewritten.replace(new RegExp(`${element}\\.classList\\.toggle\\("([^"]+)"\\)`, "g"), (call, target: string, token: string) => {
    if (!registered(token)) return call;
    replacements += 1;
    required.add("classToggle");
    return `classToggle(${target}, ${JSON.stringify(token)})`;
  });
  rewritten = rewritten.replace(new RegExp(`${element}\\.classList\\.add\\("([^"]+)"\\)`, "g"), (call, target: string, token: string) => {
    if (!registered(token)) return call;
    replacements += 1;
    required.add("classAdd");
    return `classAdd(${target}, [${JSON.stringify(token)}])`;
  });
  rewritten = rewritten.replace(new RegExp(`${element}\\.classList\\.remove\\("([^"]+)"\\)`, "g"), (call, target: string, token: string) => {
    if (!registered(token)) return call;
    replacements += 1;
    required.add("classRemove");
    return `classRemove(${target}, ${JSON.stringify(token)})`;
  });
  rewritten = rewritten.replace(new RegExp(`${element}\\.classList\\.contains\\("([^"]+)"\\)`, "g"), (call, target: string, token: string) => {
    if (!registered(token)) return call;
    replacements += 1;
    required.add("classHas");
    return `classHas(${target}, ${JSON.stringify(token)})`;
  });
  if (required.size > 0) {
    const withImports = addRuntimeImports(rewritten, Array.from(required));
    if (withImports === rewritten && !Array.from(required).every((name) => new RegExp(`\\b${name}\\b`).test(source))) {
      throw new Error(`${file}: could not locate the JSX runtime import needed for typed DOM class helpers`);
    }
    rewritten = withImports;
  }
  const unknownTokens = Array.from(unknown);
  unknownTokens.sort();
  return {
    source: rewritten,
    replacements: replacements,
    unknown: unknownTokens,
  };
}

/** Applies the class-literal rewrite to authored TSX source files. */
export function main(): void {
  let replacements = 0;
  let hasUnknown = false;
  for (const file of tsxFiles(sourceDirectory)) {
    const source = readFileSync(file, "utf8");
    const jsxResult = rewriteClassNames(source, file);
    const domResult = rewriteDOMClassUses(jsxResult.source, file);
    if (domResult.source !== source) writeFileSync(file, domResult.source);
    replacements += jsxResult.replacements + domResult.replacements;
    const unknown = Array.from(new Set([...jsxResult.unknown, ...domResult.unknown]));
    unknown.sort();
    if (unknown.length > 0) {
      hasUnknown = true;
      console.warn(`${file.slice(frontendDirectory.length + 1)}: undefined tokens ${unknown.join(", ")}`);
    }
  }
  console.log(`Rewrote ${replacements} JSX and DOM class uses.`);
  if (hasUnknown) console.log("Undefined class tokens were left unchanged for manual resolution.");
}

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) main();
