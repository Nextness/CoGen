import { readFileSync } from "node:fs";
import { describe, it } from "node:test";
import assert from "node:assert/strict";

import { collectClassUses, unknownClassUses, untypedDOMClassUses } from "../../../../../tooling/node/frontend/check-classes.ts";
import { extractClassTokens, generateClassRegistry, isClassRegistryFresh } from "../../../../../tooling/node/frontend/generate-classes.ts";
import { classTokens } from "../../../../../frontend/src/jsx/classes.ts";

describe("generated CSS class registry", () => {
  it("extracts class tokens from compound and functional selectors without pseudo or attribute values", () => {
    const css = `.alpha:is(.beta, .gamma):has(.delta):not(.epsilon):nth-child(even)::before[class*="wide"] { color: red; }\n@media (width > 1px) { .zeta::after { color: blue; } }`;
    assert.deepEqual(extractClassTokens(css), ["alpha", "beta", "delta", "epsilon", "gamma", "zeta"]);
  });

  it("matches freshly generated contents and rejects stale contents", () => {
    const committed = readFileSync(new URL("../../../../../frontend/src/jsx/classes.ts", import.meta.url), "utf8");
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
});
