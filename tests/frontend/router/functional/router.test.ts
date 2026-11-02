// Unit tests for router.tsx — URL state, view routing, render orchestrator.
import { describe, it, before, beforeEach } from "node:test";
import assert from "node:assert/strict";

import "../../../support/node/setup.ts";
import { seedViewerState } from "../../../support/node/seed.ts";
import { setURL, bindFocusContext, render, replaceState } from "../../../../frontend/src/router.tsx";
import { state, app, viewerState } from "../../../../frontend/src/state.tsx";

describe("router.tsx — setURL", () => {

  beforeEach(() => {
    seedViewerState({ view: "overview" });
  });

  it("assigns the destination page for a cross-view push", () => {
    const originalLocation = globalThis.location;
    let assigned = "";
    globalThis.location = {
      href: originalLocation.href,
      origin: originalLocation.origin,
      pathname: originalLocation.pathname,
      search: originalLocation.search,
      assign: (href: string) => { assigned = href; },
      replace: () => {},
    } as unknown as Location;

    try {
      setURL({ view: "corpus" }, false);
      assert.equal(assigned, "/corpus");
      assert.equal(viewerState.view, "corpus");
      assert.equal(sessionStorage.getItem("rw-viewer-state"), JSON.stringify({ view: "corpus" }));
    } finally {
      globalThis.location = originalLocation;
    }
  });

  it("replaces the destination page for a cross-view replacement", () => {
    const originalLocation = globalThis.location;
    let replaced = "";
    globalThis.location = {
      href: originalLocation.href,
      origin: originalLocation.origin,
      pathname: originalLocation.pathname,
      search: originalLocation.search,
      assign: () => {},
      replace: (href: string) => { replaced = href; },
    } as unknown as Location;

    try {
      setURL({ view: "provenance" }, true);
      assert.equal(replaced, "/provenance");
    } finally {
      globalThis.location = originalLocation;
    }
  });

  it("pushes history and renders for a same-view update", () => {
    const previousRequest = state.request;

    setURL({
      view: "overview",
      run_id: "1",
    }, false);

    assert.equal(location.pathname, "/overview");
    assert.equal(viewerState.run_id, "1");
    assert.equal(history.state.run_id, "1");
    assert.equal(sessionStorage.getItem("rw-viewer-state"), JSON.stringify({
      view: "overview",
      run_id: "1",
    }));
    assert.ok(state.request > previousRequest);
  });

});

describe("router.tsx — replaceState", () => {

  beforeEach(() => {
    seedViewerState({
      view: "corpus",
      section: "articles",
    });
  });

  it("updates viewerState, history.state, and sessionStorage without rendering", () => {
    const previousRequest = state.request;
    replaceState({ page: 3 });
    assert.equal(viewerState.page, "3");
    assert.equal(history.state.page, "3");
    assert.equal(location.pathname, "/corpus");
    assert.equal(sessionStorage.getItem("rw-viewer-state"), JSON.stringify({
      view: "corpus",
      section: "articles",
      page: "3",
    }));
    assert.equal(state.request, previousRequest);
  });

});

describe("router.tsx — bindFocusContext", () => {

  it("binds click handler to focus-context button", () => {
    document.querySelectorAll("[data-focus-context]").forEach((element) => { element.remove(); });
    const button = document.createElement("button");
    button.dataset.focusContext = "";
    document.body.appendChild(button);
    const trigger = document.querySelector(`[data-context-dropdown="search"] .rw-search-dropdown__trigger`) as HTMLButtonElement;
    trigger.disabled = false;

    bindFocusContext();

    button.click();
    assert.equal(document.activeElement, trigger);

    button.remove();
  });

  it("does nothing when button is missing", () => {
    document.querySelectorAll("[data-focus-context]").forEach((element) => { element.remove(); });
    (document.querySelector(`[data-context-dropdown="search"] .rw-search-dropdown__trigger`) as HTMLButtonElement).focus();
    const focused = document.activeElement;
    bindFocusContext();
    assert.equal(document.activeElement, focused);
  });

});

describe("router.tsx — render", () => {

  before(() => {
    state.request = 0;
    state.controller = null;
  });

  it("renders overview when no view is set", async () => {
    const originalFetch = globalThis.fetch;
    globalThis.fetch = () => {
      return Promise.resolve({
        ok: true,
        status: 200,
        json: () => { return Promise.resolve({ data: [] }); },
      } as unknown as Response);
    };

    seedViewerState({ view: "overview" });

    await render({ focusTitle: true });
    assert.ok(app.querySelector("#page-title"));
    assert.equal(document.activeElement, app.querySelector("#page-title"));
    assert.equal((document.querySelector(".rw-context-panel") as HTMLElement).hidden, false);
    assert.equal((document.querySelector(".rw-primary-nav") as HTMLElement).hidden, false);
    assert.equal(document.querySelector(`[data-view-link="overview"]`)!.classList.contains("active"), true);
    assert.match(document.querySelector("#workspace-breadcrumb")!.textContent, /Home.*Deepdive.*Overview/);

    globalThis.fetch = originalFetch;
  });

  it("renders Home as the root shell without Deepdive context or tabs", async () => {
    const originalFetch = globalThis.fetch;
    globalThis.fetch = () => {
      return Promise.resolve({
        ok: true,
        status: 200,
        json: () => { return Promise.resolve({ data: [] }); },
      } as unknown as Response);
    };
    seedViewerState({ view: "home" });

    await render();

    assert.equal(document.querySelector(".rw-page-header__kicker"), null);
    assert.equal((document.querySelector(".rw-context-panel") as HTMLElement).hidden, true);
    assert.equal((document.querySelector(".rw-primary-nav") as HTMLElement).hidden, true);
    assert.equal(document.querySelector("#workspace-breadcrumb")!.textContent.trim(), "Home");

    globalThis.fetch = originalFetch;
  });

  it("claims the next route controller synchronously before teardown yields", async () => {
    const originalFetch = globalThis.fetch;
    globalThis.fetch = (() => {
      return Promise.resolve({
        ok: true,
        status: 200,
        json: () => Promise.resolve({ data: [] }),
      } as Response);
    }) as typeof fetch;
    seedViewerState({ view: "home" });
    const previous = new AbortController();
    state.controller = previous;

    const pending = render();

    assert.equal(previous.signal.aborted, true);
    assert.notEqual(state.controller, previous);
    await pending;
    globalThis.fetch = originalFetch;
  });

});
