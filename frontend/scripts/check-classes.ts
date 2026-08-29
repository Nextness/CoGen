import { readFileSync, readdirSync } from "node:fs";
import { dirname, extname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

import { classTokens } from "../src/jsx/classes.ts";

const frontendDirectory = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const checkedExtensions = new Set([".ts", ".tsx"]);

/** One statically visible CSS class token and its source file. */
type ClassUse = {
  file: string;
  token: string;
};

/** Returns authored TypeScript source files below a directory. */
function sourceFiles(directory: string): string[] {
  const files: string[] = [];
  for (const entry of readdirSync(directory, { withFileTypes: true })) {
    const path = resolve(directory, entry.name);
    if (entry.isDirectory()) {
      files.push(...sourceFiles(path));
    } else if (checkedExtensions.has(extname(entry.name))) {
      files.push(path);
    }
  }
  return files;
}

/** Adds every non-empty token in one class string to the use inventory. */
function addTokens(uses: ClassUse[], file: string, value: string): void {
  for (const token of value.trim().split(/\s+/)) {
    if (token) {
      uses.push({
        file: file,
        token: token,
      });
    }
  }
}

/** Adds quoted class arguments from a statically inspectable expression. */
function addQuotedArguments(uses: ClassUse[], file: string, value: string): void {
  for (const match of value.matchAll(/"([^"\\]*(?:\\.[^"\\]*)*)"|'([^'\\]*(?:\\.[^'\\]*)*)'/g)) {
    addTokens(uses, file, match[1] ?? match[2] ?? "");
  }
}

/** Adds tokens from values whose ClassName annotations make them compile-time checked. */
function addTypedClassUses(uses: ClassUse[], source: string, file: string): void {
  const recordPattern = /\b(?:const|let|var)\s+[A-Za-z][A-Za-z0-9]*\s*:\s*Record<[^=;]*ClassName[^=;]*>\s*=\s*\{([\s\S]*?)^\s*\};/gm;
  for (const recordMatch of source.matchAll(recordPattern)) {
    const values = recordMatch[1].matchAll(/:\s*"([^"]*)"/g);
    for (const valueMatch of values) addTokens(uses, file, valueMatch[1]);
  }

  const arrayPattern = /\b(?:const|let|var)\s+([A-Za-z][A-Za-z0-9]*)\s*:\s*(?:readonly\s+)?ClassName\[\]\s*=\s*\[([^\]]*)\]/g;
  for (const arrayMatch of source.matchAll(arrayPattern)) {
    addQuotedArguments(uses, file, arrayMatch[2]);
    const pushPattern = new RegExp(`\\b${arrayMatch[1]}\\.push\\(([^)]*)\\)`, "g");
    for (const pushMatch of source.matchAll(pushPattern)) addQuotedArguments(uses, file, pushMatch[1]);
  }

  const variablePattern = /\b(?:const|let|var)\s+([A-Za-z][A-Za-z0-9]*)\s*:\s*ClassName(?!\s*\[)(?:\s*\|\s*undefined)?(?:\s*=\s*"([^"]*)")?/g;
  for (const variableMatch of source.matchAll(variablePattern)) {
    if (variableMatch[2]) addTokens(uses, file, variableMatch[2]);
    const assignmentPattern = new RegExp(`\\b${variableMatch[1]}\\s*=\\s*"([^"]*)"`, "g");
    for (const assignmentMatch of source.matchAll(assignmentPattern)) addTokens(uses, file, assignmentMatch[1]);
  }

  const functionPattern = /\bfunction\s+[A-Za-z][A-Za-z0-9]*\([^)]*\):\s*ClassName\s*\{([\s\S]*?)^\}/gm;
  for (const functionMatch of source.matchAll(functionPattern)) {
    const returns = functionMatch[1].matchAll(/\breturn\s+"([^"]*)"/g);
    for (const returnMatch of returns) addTokens(uses, file, returnMatch[1]);
  }
}

/** Collects statically visible class-token uses outside JSX's type coverage. */
export function collectClassUses(source: string, file: string): ClassUse[] {
  const uses: ClassUse[] = [];
  const literalPatterns = [
    /\bclassName\s*=\s*"([^"]*)"/g,
    /\bclassName\s*:\s*"([^"]*)"/g,
    /\bclassName\s*:\s*'([^']*)'/g,
    /\.className\s*=\s*"([^"]*)"/g,
    /\.className\s*=\s*'([^']*)'/g,
    /\bclass\s*=\s*"([^"]*)"/g,
    /\bmodifier\s*=\s*"([^"]*)"/g,
  ];
  for (const pattern of literalPatterns) {
    for (const match of source.matchAll(pattern)) {
      addTokens(uses, file, match[1]);
    }
  }

  const callPatterns = [
    /\.classList\.(?:add|remove|toggle|contains)\s*\(([^)]*)\)/g,
    /\b(?:cx|classAdd|classRemove|classToggle|classHas)\s*\(([^)]*)\)/g,
    /\b(?:classes|tableClasses)=\{\[([^\]]*)\]\}/g,
  ];
  for (const pattern of callPatterns) {
    for (const match of source.matchAll(pattern)) {
      addQuotedArguments(uses, file, match[1]);
    }
  }
  addTypedClassUses(uses, source, file);
  return uses;
}

/** Returns class-token uses that are absent from the generated registry. */
export function unknownClassUses(uses: readonly ClassUse[], knownTokens: ReadonlySet<string>): ClassUse[] {
  return uses.filter((use) => !knownTokens.has(use.token));
}

/** Returns direct DOM class operations that cannot be statically validated. */
export function untypedDOMClassUses(source: string): string[] {
  const uses: string[] = [];
  const patterns = [
    /\.classList\.(?:add|remove|toggle|contains)\s*\(/g,
    /\.className\s*=(?!\s*(?:cx\(|classNames\.[A-Za-z][A-Za-z0-9]*|"[^"]*"|'[^']*'))/g,
  ];
  for (const pattern of patterns) {
    for (const match of source.matchAll(pattern)) {
      uses.push(match[0]);
    }
  }
  return uses;
}

/** Checks non-JSX class uses and reports defined tokens that no authored source uses. */
export function main(): void {
  const files = sourceFiles(resolve(frontendDirectory, "src"));
  const uses: ClassUse[] = [];
  for (const file of files) {
    const relativeFile = file.slice(frontendDirectory.length + 1);
    const source = readFileSync(file, "utf8");
    uses.push(...collectClassUses(source, relativeFile));
  }
  uses.push(...collectClassUses(readFileSync(resolve(frontendDirectory, "index.html"), "utf8"), "index.html"));

  const runtimePath = resolve(frontendDirectory, "src/jsx/jsx-runtime.ts");
  const untyped: Array<{ file: string; use: string }> = [];
  for (const file of files) {
    if (file === runtimePath) continue;
    const fileUses = untypedDOMClassUses(readFileSync(file, "utf8"));
    for (const use of fileUses) {
      untyped.push({
        file: file.slice(frontendDirectory.length + 1),
        use: use,
      });
    }
  }
  if (untyped.length > 0) {
    for (const item of untyped) {
      console.error(`${item.file}: direct DOM class operation ${JSON.stringify(item.use)} bypasses typed helpers`);
    }
    process.exitCode = 1;
    return;
  }

  const knownTokens = new Set<string>(classTokens);
  const unknown = unknownClassUses(uses, knownTokens);
  if (unknown.length > 0) {
    for (const use of unknown) {
      console.error(`${use.file}: undefined class token ${JSON.stringify(use.token)}`);
    }
    process.exitCode = 1;
    return;
  }

  const usedTokens = new Set(uses.map((use) => use.token));
  const unused = classTokens.filter((token) => !usedTokens.has(token));
  console.log(`Class registry: ${classTokens.length} defined, ${usedTokens.size} statically used, ${unused.length} defined but not statically used.`);
  if (unused.length > 0) {
    console.log(`Defined but not statically used: ${unused.join(", ")}`);
  }
}

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) main();
