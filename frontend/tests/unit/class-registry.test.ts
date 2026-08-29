import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { describe, it } from "node:test";
import assert from "node:assert/strict";

import { collectClassUses, unknownClassUses, untypedDOMClassUses } from "../../scripts/check-classes.ts";
import { extractClassTokens, generateClassRegistry, isClassRegistryFresh } from "../../scripts/generate-classes.ts";
import { rewriteClassNames, rewriteDOMClassUses } from "../../scripts/rewrite-class-names.ts";
import { classTokens } from "../../src/jsx/classes.ts";

describe("generated CSS class registry", () => {
  it("extracts class tokens from compound and functional selectors without pseudo or attribute values", () => {
    const css = `.alpha:is(.beta, .gamma):has(.delta):not(.epsilon):nth-child(even)::before[class*="wide"] { color: red; }\n@media (width > 1px) { .zeta::after { color: blue; } }`;
    assert.deepEqual(extractClassTokens(css), ["alpha", "beta", "delta", "epsilon", "gamma", "zeta"]);
  });

  it("matches freshly generated contents and rejects stale contents", () => {
    const committed = readFileSync(resolve("src/jsx/classes.ts"), "utf8");
    const generated = generateClassRegistry();
    assert.equal(isClassRegistryFresh(committed, generated), true);
    assert.equal(isClassRegistryFresh(`${committed}\n`, generated), false);
  });

  it("does not include pseudo-class, pseudo-element, or nth-child argument names", () => {
    const tokens = new Set<string>(classTokens);
    for (const invalid of ["before", "after", "even", "checked", "empty", "backdrop"]) {
      assert.equal(tokens.has(invalid), false, invalid);
    }
  });

  it("identifies undefined static tokens and direct untyped DOM operations", () => {
    const uses = collectClassUses(`<div className="ui missing-token"></div>`, "fixture.tsx");
    const expectedUnknown = [{
      file: "fixture.tsx",
      token: "missing-token",
    }];
    assert.deepEqual(unknownClassUses(uses, new Set<string>(classTokens)), expectedUnknown);

    const untypedSource = `node.classList.add("ui"); node.className = dynamicClass; node.className = classNames.uiButton;`;
    assert.deepEqual(untypedDOMClassUses(untypedSource), [".classList.add(", ".className ="]);
  });

  it("rewrites compound JSX and DOM classes through typed named helpers", () => {
    const source = `import { h } from "./jsx/jsx-runtime.ts";\nexport function Example(): JSX.Element { return <button className="ui basic button">Open</button>; }\n`;
    const jsxResult = rewriteClassNames(source, "fixture.tsx");
    assert.equal(jsxResult.replacements, 1);
    assert.match(jsxResult.source, /const classNames = \{/);
    assert.match(jsxResult.source, /uiBasicButton: cx\("ui", "basic", "button"\)/);
    assert.match(jsxResult.source, /className=\{classNames\.uiBasicButton\}/);
    assert.doesNotMatch(jsxResult.source, /className=\{cx\(/);

    const domSource = `import { h } from "./jsx/jsx-runtime.ts";\nbutton.classList.toggle("active", selected);\n`;
    const domResult = rewriteDOMClassUses(domSource, "fixture.tsx");
    assert.equal(domResult.replacements, 1);
    assert.match(domResult.source, /classToggle\(button, "active", selected\)/);
  });
});
